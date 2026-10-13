import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;

import '../models/aicrew.dart';

/// Токены сессии устройства.
class AuthTokens {
  const AuthTokens({required this.accessToken, required this.refreshToken, required this.userId, required this.deviceId});

  final String accessToken;
  final String refreshToken;
  final String userId;
  final String deviceId;

  factory AuthTokens.fromJson(Map<String, dynamic> j) => AuthTokens(
        accessToken: j['access_token'] as String,
        refreshToken: j['refresh_token'] as String,
        userId: j['user_id'] as String,
        deviceId: j['device_id'] as String,
      );

  Map<String, dynamic> toJson() =>
      {'access_token': accessToken, 'refresh_token': refreshToken, 'user_id': userId, 'device_id': deviceId};
}

/// Где хранятся токены: Keychain / Keystore / Credential Manager — реализация на платформе.
abstract interface class TokenStore {
  Future<AuthTokens?> read();
  Future<void> write(AuthTokens tokens);
  Future<void> clear();
}

class MemoryTokenStore implements TokenStore {
  AuthTokens? _t;
  @override
  Future<AuthTokens?> read() async => _t;
  @override
  Future<void> write(AuthTokens tokens) async => _t = tokens;
  @override
  Future<void> clear() async => _t = null;
}

/// Ошибка, которую вернул сервер.
class ApiException implements Exception {
  ApiException(this.status, this.code, this.message, [this.body]);
  final int status;
  final String code;
  final String message;
  final Map<String, dynamic>? body;

  @override
  String toString() => 'ApiException($status $code: $message)';
}

/// Сервер недоступен (нет сети, таймаут). Данные остаются в очереди.
class NetworkException implements Exception {
  NetworkException(this.cause);
  final Object cause;
  @override
  String toString() => 'NetworkException($cause)';
}

/// Сессия больше не действует: нужно войти заново. Локальные данные не трогаем.
class AuthRequiredException implements Exception {}

class ApiClient {
  ApiClient({
    required this.baseUrl,
    required this.tokens,
    http.Client? client,
    this.timeout = const Duration(seconds: 30),
    this.headers = const {},
  }) : _http = client ?? http.Client();

  final Uri baseUrl;
  final TokenStore tokens;

  /// Дополнительные заголовки каждого запроса (веб: X-PT-Client: web).
  final Map<String, String> headers;
  final http.Client _http;
  final Duration timeout;
  Future<AuthTokens>? _refreshing;

  Uri _u(String path, [Map<String, String>? query]) =>
      baseUrl.replace(path: '${baseUrl.path.replaceAll(RegExp(r'/$'), '')}/api/v1$path', queryParameters: query);

  Uri get wsUrl {
    final u = _u('/ws');
    return u.replace(scheme: u.scheme == 'https' ? 'wss' : 'ws');
  }

  Future<http.Response> _send(String method, Uri url, {Object? body, String? token}) async {
    final req = http.Request(method, url);
    req.headers.addAll(headers);
    req.headers['Content-Type'] = 'application/json';
    if (token != null) req.headers['Authorization'] = 'Bearer $token';
    if (body != null) req.body = jsonEncode(body);
    try {
      return await http.Response.fromStream(await _http.send(req).timeout(timeout));
    } on TimeoutException catch (e) {
      throw NetworkException(e);
    } on http.ClientException catch (e) {
      throw NetworkException(e);
    }
  }

  Map<String, dynamic> _json(http.Response r) {
    if (r.body.isEmpty) return {};
    final v = jsonDecode(utf8.decode(r.bodyBytes));
    return v is Map<String, dynamic> ? v : {};
  }

  Never _fail(http.Response r) {
    Map<String, dynamic> body = {};
    try {
      body = _json(r);
    } catch (_) {}
    final err = body['error'];
    if (err is Map) {
      throw ApiException(r.statusCode, err['code'] as String? ?? 'error', err['message'] as String? ?? '', body);
    }
    throw ApiException(r.statusCode, 'http_${r.statusCode}', r.reasonPhrase ?? '', body);
  }

  // ── auth ──

  Future<void> register(String email, String password, {String name = ''}) async {
    final r = await _send('POST', _u('/auth/register'), body: {'email': email, 'password': password, 'name': name});
    if (r.statusCode != 201) _fail(r);
  }

  Future<AuthTokens> login(String email, String password,
      {required String deviceId, required String deviceName, required String platform}) async {
    final r = await _send('POST', _u('/auth/login'), body: {
      'email': email,
      'password': password,
      'device': {'id': deviceId, 'name': deviceName, 'platform': platform},
    });
    if (r.statusCode != 200) _fail(r);
    final t = AuthTokens.fromJson(_json(r));
    await tokens.write(t);
    return t;
  }

  Future<void> logout() async {
    final t = await tokens.read();
    await tokens.clear();
    if (t == null) return;
    try {
      await _send('POST', _u('/auth/logout'), body: {'refresh_token': t.refreshToken});
    } on NetworkException {
      // Токен и так удалён с устройства; сессия истечёт сама.
    }
  }

  /// Один refresh на всех: параллельные запросы ждут общий результат.
  Future<AuthTokens> _refresh(AuthTokens old) => _refreshing ??= _doRefresh(old).whenComplete(() => _refreshing = null);

  Future<AuthTokens> _doRefresh(AuthTokens old) async {
    for (var attempt = 0; attempt < 2; attempt++) {
      final current = await tokens.read();
      if (current == null) throw AuthRequiredException();
      if (current.refreshToken != old.refreshToken) return current; // уже обновил кто-то другой
      final r = await _send('POST', _u('/auth/refresh'), body: {'refresh_token': current.refreshToken});
      if (r.statusCode == 200) {
        final t = AuthTokens.fromJson(_json(r));
        await tokens.write(t);
        return t;
      }
      if (r.statusCode == 409) {
        // Токен только что ротировал другой запрос; ждём, пока новый попадёт в хранилище.
        await Future<void>.delayed(const Duration(milliseconds: 500));
        continue;
      }
      if (r.statusCode == 401) {
        await tokens.clear();
        throw AuthRequiredException();
      }
      _fail(r);
    }
    throw AuthRequiredException();
  }

  Future<http.Response> _authed(String method, Uri url, {Object? body}) async {
    var t = await tokens.read();
    if (t == null) throw AuthRequiredException();
    var r = await _send(method, url, body: body, token: t.accessToken);
    if (r.statusCode == 401) {
      t = await _refresh(t);
      r = await _send(method, url, body: body, token: t.accessToken);
      if (r.statusCode == 401) throw AuthRequiredException();
    }
    return r;
  }

  /// Текущий access-токен (обновлённый при необходимости) — для WebSocket.
  Future<String> freshAccessToken() async {
    final t = await tokens.read();
    if (t == null) throw AuthRequiredException();
    final r = await _send('GET', _u('/me'), token: t.accessToken);
    if (r.statusCode == 200) return t.accessToken;
    if (r.statusCode == 401) return (await _refresh(t)).accessToken;
    _fail(r);
  }

  // ── sync ──

  /// Ответ 503 с частью результатов — это нормальный ответ: обрабатываем то, что есть.
  Future<List<Map<String, dynamic>>> push(List<Map<String, dynamic>> operations) async {
    final r = await _authed('POST', _u('/sync/push'), body: {'operations': operations});
    if (r.statusCode != 200 && r.statusCode != 503) _fail(r);
    final results = (_json(r)['results'] as List? ?? const []).cast<Map<String, dynamic>>();
    if (r.statusCode == 503 && results.isEmpty) _fail(r);
    return results;
  }

  Future<Map<String, dynamic>> pull(int cursor, {int limit = 500}) async {
    final r = await _authed('GET', _u('/sync/pull', {'cursor': '$cursor', 'limit': '$limit'}));
    if (r.statusCode != 200) _fail(r);
    return _json(r);
  }

  // ── AI Structuring Engine (этап 6) ──

  /// Ставит задачу в очередь структурирования. Результат придёт через синхронизацию.
  /// [sourceRevision] — подтверждённая сервером ревизия исходника (409 stale_source, если устарела).
  Future<String> requestStructure(String nodeId, int sourceRevision) async {
    final r = await _authed('POST', _u('/ai/structure'), body: {'node_id': nodeId, 'source_revision': sourceRevision});
    if (r.statusCode != 202) _fail(r);
    return _json(r)['request_id'] as String;
  }

  // ── AiCrew (этапы 7.2–7.6): канбан и решения человека ──

  Future<List<Map<String, dynamic>>> _list(String path, String key, [Map<String, String>? query]) async {
    final r = await _authed('GET', _u(path, query));
    if (r.statusCode != 200) _fail(r);
    return (_json(r)[key] as List? ?? const []).whereType<Map<String, dynamic>>().toList();
  }

  Future<void> _post(String path, [Object? body]) async {
    final r = await _authed('POST', _u(path), body: body ?? const <String, dynamic>{});
    if (r.statusCode >= 300) _fail(r);
  }

  /// Канбан целиком: задачи, планы, запросы на решение, открытые инциденты, исполнители.
  Future<AiCrewBoard> aicrewBoard() async {
    final res = await Future.wait([
      _list('/exec/tasks', 'tasks'),
      _list('/exec/plans', 'plans'),
      _list('/exec/approvals', 'approvals', {'status': 'pending'}),
      _list('/exec/incidents', 'incidents', {'open': '1'}),
      _list('/exec/workers', 'workers'),
    ]);
    return AiCrewBoard(
      tasks: res[0].map(ExecTask.fromJson).toList(),
      plans: res[1].map(ExecPlan.fromJson).toList(),
      approvals: res[2].map(Approval.fromJson).toList(),
      incidents: res[3].map(Incident.fromJson).toList(),
      workers: res[4].map(WorkerView.fromJson).toList(),
    );
  }

  /// Репозитории AiCrew (привязанные к проектам на ПК с aicrew-host).
  Future<List<ExecRepository>> execRepositories() async =>
      (await _list('/exec/repositories', 'repositories')).map(ExecRepository.fromJson).toList();

  /// AI Task из дерева → задача AiCrew. Текст сервер берёт из итоговой задачи узла.
  /// Исполнитель — всегда Claude: Junior работает только по контракту внутри плана Manager'а.
  Future<String> createExecTask({
    required String nodeId,
    required String repositoryId,
    String mode = 'manual',
    String reviewLevel = 'logic',
    bool touchesUI = false,
  }) async {
    final r = await _authed('POST', _u('/exec/tasks'), body: {
      'node_id': nodeId,
      'repository_id': repositoryId,
      'execution_mode': mode,
      'review_level': reviewLevel,
      'touches_ui': touchesUI,
    });
    if (r.statusCode != 201) _fail(r);
    return _json(r)['id'] as String;
  }

  /// AI Task из дерева → план Manager'а (декомпозиция на подзадачи).
  Future<String> createExecPlan({required String nodeId, required String repositoryId, String mode = 'manual'}) async {
    final r = await _authed('POST', _u('/exec/plans'),
        body: {'node_id': nodeId, 'repository_id': repositoryId, 'execution_mode': mode, 'request': ''});
    if (r.statusCode != 201) _fail(r);
    return _json(r)['id'] as String;
  }

  /// Ответ на запрос: approve | reject (с комментарием) | partial (задачи плана).
  Future<void> resolveApproval(String id, String decision, {String comment = '', List<int>? tasks}) =>
      _post('/exec/approvals/$id', {'decision': decision, 'comment': comment, if (tasks != null) 'tasks': tasks});

  Future<void> retryTask(String id) => _post('/exec/tasks/$id/retry');
  Future<void> cancelTask(String id) => _post('/exec/tasks/$id/cancel');

  /// «Остановить всё»: AiCrew на паузе до «Возобновить».
  Future<void> stopAll(String reason) => _post('/exec/incidents', {'reason': reason});
  Future<void> resolveIncident(String id, String comment) => _post('/exec/incidents/$id/resolve', {'comment': comment});

  void close() => _http.close();
}
