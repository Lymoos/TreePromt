// Сквозной тест с настоящим Go-сервером. Запуск:
//   PT_SERVER_URL=http://127.0.0.1:58081 flutter test test/server_integration_test.dart
// Сервер должен быть запущен с ALLOW_REGISTRATION=true на пустой базе.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'support.dart';

class Device {
  Device(Uri server, this.name)
      : store = memoryStore(),
        client = ApiClient(baseUrl: server, tokens: MemoryTokenStore()) {
    tree = TreeService(store);
    engine = SyncEngine.withClient(store: store, client: client);
    id = newId();
  }

  final DriftLocalStore store;
  final ApiClient client;
  late final TreeService tree;
  late final SyncEngine engine;
  late final String id;
  final String name;

  Future<void> login(String email, String password) =>
      client.login(email, password, deviceId: id, deviceName: name, platform: 'test');

  Future<void> sync() async {
    await engine.syncNow();
    expect(engine.state.phase, SyncPhase.idle, reason: '$name: ${engine.state.error}');
  }
}

void main() {
  final url = Platform.environment['PT_SERVER_URL'];

  test('two devices: offline edits of the same text keep both versions', () async {
    final server = Uri.parse(url!);
    final email = 'it-${newId()}@example.com';
    const password = 'длинный-пароль-1';
    await ApiClient(baseUrl: server, tokens: MemoryTokenStore()).register(email, password);

    final phone = Device(server, 'phone');
    final pc = Device(server, 'pc');
    await phone.login(email, password);
    await pc.login(email, password);

    final p = await phone.tree.createProject('PromptTree');
    final n = await phone.tree.createNode(projectId: p, kind: NodeKind.aiTask, name: 'Дерево', text: 'v0');
    await phone.sync();
    await pc.sync();
    expect((await pc.store.content(n))!.rawContent, 'v0');

    // Оба офлайн правят один текст.
    await phone.tree.setText(n, 'правка с телефона');
    await pc.tree.setText(n, 'правка с ПК 💻');
    // ПК параллельно переименовывает — это другое поле, оно должно просто примениться.
    await pc.tree.rename(n, 'Дерево (ПК)');

    await phone.sync();
    await pc.sync();
    await phone.sync();

    for (final d in [phone, pc]) {
      final node = (await d.store.node(n))!;
      expect(node.hasConflict, isTrue, reason: d.name);
      expect(node.name, 'Дерево (ПК)', reason: d.name);
      expect((await d.store.content(n))!.rawContent, 'правка с телефона', reason: d.name);
      final versions = await d.store.watchVersions(n).first;
      expect(versions.map((v) => v.content).toSet(), containsAll(['правка с телефона', 'правка с ПК 💻']),
          reason: '${d.name} must have both versions');
    }

    await pc.tree.resolveConflict(n, 'итог');
    await pc.sync();
    await phone.sync();
    final node = (await phone.store.node(n))!;
    expect(node.hasConflict, isFalse);
    expect((await phone.store.content(n))!.rawContent, 'итог');

    for (final d in [phone, pc]) {
      await d.engine.dispose();
      await d.store.db.close();
    }
  }, skip: url == null ? 'PT_SERVER_URL is not set' : false);
}
