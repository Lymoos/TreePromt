import 'package:prompttree_shared/prompttree_shared.dart';

import 'providers.dart';

/// Восстанавливает режим работы при запуске: вход на сервер, локальный режим или экран входа.
Future<Session> loadSession(LocalStore store, TokenStore tokens) async {
  final url = await store.getValue(serverUrlKey);
  final mode = await store.getValue(sessionModeKey);
  final serverUrl = url == null ? null : Uri.tryParse(url);
  if (serverUrl != null && await tokens.read() != null) {
    return Session(mode: SessionMode.server, serverUrl: serverUrl);
  }
  if (mode == SessionMode.local.name) return Session(mode: SessionMode.local, serverUrl: serverUrl);
  return Session(mode: SessionMode.none, serverUrl: serverUrl);
}
