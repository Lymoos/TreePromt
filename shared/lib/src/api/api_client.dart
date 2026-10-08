import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;

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

  void close() => _http.close();
}
