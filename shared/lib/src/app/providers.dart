import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

/// Переопределяются в main() после открытия БД.
final storeProvider = Provider<LocalStore>((ref) => throw UnimplementedError('storeProvider'));
final tokenStoreProvider = Provider<TokenStore>((ref) => throw UnimplementedError('tokenStoreProvider'));

/// Имя и платформа устройства для списка сессий на сервере. Задаёт оболочка приложения.
typedef DeviceInfo = ({String name, String platform});
final deviceInfoProvider = Provider<DeviceInfo>((ref) => (name: 'unknown', platform: 'unknown'));

/// Заголовки для всех запросов к серверу. Веб: {'X-PT-Client': 'web'} — refresh-токен в cookie.
final apiHeadersProvider = Provider<Map<String, String>>((ref) => const {});

/// Сервер PromptTree. Приложения: адрес, вшитый в сборку (PT_SERVER_URL);
/// веб: тот же адрес, откуда открыта страница. null — сборка для разработки.
final defaultServerUrlProvider = Provider<Uri?>((ref) => builtInServer);

/// Сервер один и задан сборкой — поле «Сервер» на экране входа не показывается.
final serverFixedProvider = Provider<bool>((ref) => ref.watch(defaultServerUrlProvider) != null);

enum SessionMode { none, local, server }

class Session {
  const Session({required this.mode, this.serverUrl});
  final SessionMode mode;
  final Uri? serverUrl;
}

const serverUrlKey = 'session.server_url';
const sessionModeKey = 'session.mode';
const sessionUserKey = 'session.user';

final sessionProvider = StateProvider<Session>((ref) => const Session(mode: SessionMode.none));

final apiProvider = Provider<ApiClient?>((ref) {
  final s = ref.watch(sessionProvider);
  if (s.serverUrl == null) return null;
  final c = ApiClient(
    baseUrl: s.serverUrl!,
    tokens: ref.watch(tokenStoreProvider),
    headers: ref.watch(apiHeadersProvider),
  );
  ref.onDispose(c.close);
  return c;
});

final syncEngineProvider = Provider<SyncEngine?>((ref) {
  final s = ref.watch(sessionProvider);
  final api = ref.watch(apiProvider);
  if (s.mode != SessionMode.server || api == null) return null;
  final e = SyncEngine.withClient(store: ref.watch(storeProvider), client: api)..start();
  ref.onDispose(e.dispose);
  return e;
});

final syncStateProvider = StreamProvider<SyncState>((ref) async* {
  final e = ref.watch(syncEngineProvider);
  if (e == null) {
    yield const SyncState();
    return;
  }
  yield e.state;
  yield* e.states;
});

final pendingCountProvider = StreamProvider<int>((ref) => ref.watch(storeProvider).watchPendingCount());

final treeServiceProvider = Provider<TreeService>((ref) => TreeService(ref.watch(storeProvider)));

final projectsProvider = StreamProvider<List<Project>>((ref) => ref.watch(storeProvider).watchProjects());
final nodesProvider = StreamProvider<List<TreeNode>>((ref) => ref.watch(storeProvider).watchNodes());

final expandedProvider = StateProvider<Set<String>>((ref) => <String>{});
final selectedProvider = StateProvider<String?>((ref) => null);

final treeRowsProvider = Provider<List<TreeRow>>((ref) {
  final projects = ref.watch(projectsProvider).valueOrNull ?? const [];
  final nodes = ref.watch(nodesProvider).valueOrNull ?? const [];
  return buildTree(
    projects: projects,
    nodes: nodes,
    expanded: ref.watch(expandedProvider),
    selectedId: ref.watch(selectedProvider),
  );
});

final nodeProvider = StreamProvider.family<TreeNode?, String>((ref, id) => ref.watch(storeProvider).watchNode(id));
final contentProvider =
    StreamProvider.family<NodeContent?, String>((ref, id) => ref.watch(storeProvider).watchContent(id));
final versionsProvider =
    StreamProvider.family<List<ContentVersion>, String>((ref, id) => ref.watch(storeProvider).watchVersions(id));
