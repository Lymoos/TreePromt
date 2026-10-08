import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:web_socket_channel/web_socket_channel.dart';

import '../api/api_client.dart';
import '../domain/local_store.dart';
import 'server_state.dart';

/// Что синхронизатору нужно от сервера. Реализация — [ApiClient]; в тестах — фейк.
abstract interface class SyncApi {
  Future<List<Map<String, dynamic>>> push(List<Map<String, dynamic>> operations);
  Future<Map<String, dynamic>> pull(int cursor, {int limit});
  Future<String> freshAccessToken();
  Uri get wsUrl;
}

class _ApiAdapter implements SyncApi {
  _ApiAdapter(this.api);
  final ApiClient api;
  @override
  Future<List<Map<String, dynamic>>> push(List<Map<String, dynamic>> operations) => api.push(operations);
  @override
  Future<Map<String, dynamic>> pull(int cursor, {int limit = 500}) => api.pull(cursor, limit: limit);
  @override
  Future<String> freshAccessToken() => api.freshAccessToken();
  @override
  Uri get wsUrl => api.wsUrl;
}

enum SyncPhase { idle, syncing, offline, authRequired, error }

class SyncState {
  const SyncState({this.phase = SyncPhase.idle, this.pending = 0, this.lastSyncedAt, this.error});
  final SyncPhase phase;
  final int pending;
  final DateTime? lastSyncedAt;
  final String? error;

  SyncState copyWith({SyncPhase? phase, int? pending, DateTime? lastSyncedAt, String? error}) => SyncState(
        phase: phase ?? this.phase,
        pending: pending ?? this.pending,
        lastSyncedAt: lastSyncedAt ?? this.lastSyncedAt,
        error: error,
      );
}

const cursorKey = 'sync.cursor';

/// Отправляет очередь, забирает изменения, держит WebSocket. Интерфейс о нём
/// не знает: он только читает локальную БД (архитектура п. 1).
class SyncEngine {
  SyncEngine({
    required this.store,
    required this.api,
    this.connectWs,
    this.debounce = const Duration(milliseconds: 300),
    this.batchSize = 100,
  }) : _applier = ServerStateApplier(store);

  factory SyncEngine.withClient({required LocalStore store, required ApiClient client}) => SyncEngine(
        store: store,
        api: _ApiAdapter(client),
        connectWs: WebSocketChannel.connect,
      );

  final LocalStore store;
  final SyncApi api;
  final WebSocketChannel Function(Uri)? connectWs;
  final Duration debounce;
  final int batchSize;
  final ServerStateApplier _applier;

  final _states = StreamController<SyncState>.broadcast();
  SyncState _state = const SyncState();
  SyncState get state => _state;
  Stream<SyncState> get states => _states.stream;

  StreamSubscription<int>? _pendingSub;
  Timer? _timer;
  Timer? _periodic;
  Future<void>? _running;
  bool _again = false;
  int _failures = 0;
  bool _stopped = true;

  WebSocketChannel? _ws;
  StreamSubscription<dynamic>? _wsSub;
  Timer? _wsRetry;
  int _wsFailures = 0;

  void _emit(SyncState s) {
    _state = s;
    if (!_states.isClosed) _states.add(s);
  }

  void start() {
    if (!_stopped) return;
    _stopped = false;
    _pendingSub = store.watchPendingCount().listen((n) {
      final grew = n > _state.pending;
      _emit(_state.copyWith(pending: n, error: _state.error));
      if (grew) schedule(debounce);
    });
    _periodic = Timer.periodic(const Duration(seconds: 60), (_) => schedule(Duration.zero));
    _connectWs();
    schedule(Duration.zero);
  }

  Future<void> stop() async {
    _stopped = true;
    _timer?.cancel();
    _periodic?.cancel();
    _wsRetry?.cancel();
    await _pendingSub?.cancel();
    await _wsSub?.cancel();
    await _ws?.sink.close();
    _ws = null;
    await _running;
  }

  Future<void> dispose() async {
    await stop();
    await _states.close();
  }

  void schedule(Duration delay) {
    if (_stopped) return;
    _timer?.cancel();
    _timer = Timer(delay, () => syncNow());
  }

  /// Один проход: отправить очередь, забрать изменения. Повторный вызов во время
  /// прохода не запускает второй, а просит сделать ещё один круг.
  Future<void> syncNow() {
    if (_running != null) {
      _again = true;
      return _running!;
    }
    return _running = _run().whenComplete(() => _running = null);
  }

  Future<void> _run() async {
    _emit(_state.copyWith(phase: SyncPhase.syncing));
    try {
      do {
        _again = false;
        await _push();
        await _pull();
      } while (_again);
      _failures = 0;
      _emit(_state.copyWith(phase: SyncPhase.idle, lastSyncedAt: DateTime.now()));
    } on AuthRequiredException {
      _emit(_state.copyWith(phase: SyncPhase.authRequired, error: 'Нужно войти заново'));
    } on NetworkException catch (e) {
      _emit(_state.copyWith(phase: SyncPhase.offline, error: '$e'));
      _retryLater();
    } catch (e) {
      _emit(_state.copyWith(phase: SyncPhase.error, error: '$e'));
      _retryLater();
    } finally {
      await store.releaseInFlight();
    }
  }

  void _retryLater() {
    _failures++;
    final secs = min(60, pow(2, _failures - 1).toInt());
    schedule(Duration(seconds: secs));
  }

  Future<void> _push() async {
    while (true) {
      final batch = await store.takeBatch(batchSize);
      if (batch.isEmpty) return;
      final results = await api.push([
        for (final o in batch)
          {
            'operation_id': o.operationId,
            'client_seq': o.seq,
            'type': o.type,
            'entity_id': o.entityId,
            'base_revision': o.baseRevision,
            'payload': jsonDecode(o.payload),
          }
      ]);
      final byId = {for (final o in batch) o.operationId: o};
      await store.transaction(() async {
        for (final r in results) {
          final op = byId[r['operation_id']];
          if (op == null) continue;
          await store.removeOp(op.operationId);
          if (r['project'] is Map<String, dynamic>) await _applier.project(r['project'] as Map<String, dynamic>);
          if (r['node'] is Map<String, dynamic>) await _applier.node(r['node'] as Map<String, dynamic>);
          if (r['result'] == 'rejected') {
            final code = (r['error'] as Map?)?['code'] as String? ?? 'rejected';
            await _applier.markRejected(op.type, op.entityId, code);
          }
        }
      });
      await store.releaseInFlight();
      if (results.length < batch.length) {
        throw NetworkException('server processed ${results.length} of ${batch.length} operations');
      }
    }
  }

  Future<void> _pull() async {
    var cursor = int.tryParse(await store.getValue(cursorKey) ?? '') ?? 0;
    while (true) {
      final page = await api.pull(cursor);
      await store.transaction(() async {
        for (final p in (page['projects'] as List? ?? const [])) {
          await _applier.project(p as Map<String, dynamic>);
        }
        for (final n in (page['nodes'] as List? ?? const [])) {
          await _applier.node(n as Map<String, dynamic>);
        }
        for (final v in (page['versions'] as List? ?? const [])) {
          await _applier.version(v as Map<String, dynamic>);
        }
        cursor = page['cursor'] as int? ?? cursor;
        // Курсор сдвигается в той же транзакции, что и данные (архитектура п. 4.4, шаг 5).
        await store.setValue(cursorKey, '$cursor');
      });
      if (page['has_more'] != true) return;
    }
  }

  // ── WebSocket: только сигнал «есть новая ревизия» ──

  Future<void> _connectWs() async {
    final connect = connectWs;
    if (connect == null || _stopped) return;
    try {
      final token = await api.freshAccessToken();
      if (_stopped) return;
      final ch = connect(api.wsUrl);
      await ch.ready;
      ch.sink.add(jsonEncode({'type': 'auth', 'token': token}));
      _ws = ch;
      _wsSub = ch.stream.listen(
        (msg) {
          _wsFailures = 0;
          try {
            final m = jsonDecode(msg as String) as Map<String, dynamic>;
            if (m['type'] == 'revision' || m['type'] == 'ready') schedule(Duration.zero);
          } catch (_) {}
        },
        onDone: _wsLost,
        onError: (_) => _wsLost(),
        cancelOnError: true,
      );
    } on AuthRequiredException {
      _emit(_state.copyWith(phase: SyncPhase.authRequired, error: 'Нужно войти заново'));
    } catch (_) {
      _wsLost();
    }
  }

  void _wsLost() {
    _ws = null;
    if (_stopped) return;
    _wsFailures++;
    _wsRetry?.cancel();
    _wsRetry = Timer(Duration(seconds: min(60, 5 * _wsFailures)), _connectWs);
  }
}
