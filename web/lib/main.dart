import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/app.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'web_token_store.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final DriftLocalStore store;
  final Session session;
  final tokens = WebTokenStore();
  try {
    debugPrint('[prompttree] opening local storage');
    store = openLocalStore(); // SQLite в WebAssembly: OPFS или IndexedDB
    session = await loadSession(store, tokens).timeout(const Duration(seconds: 15));
    debugPrint('[prompttree] storage ready, session: ${session.mode.name}');
  } catch (e) {
    debugPrint('[prompttree] storage failed: $e');
    runApp(StorageErrorApp(error: e));
    return;
  }
  runApp(ProviderScope(
    overrides: [
      storeProvider.overrideWithValue(store),
      tokenStoreProvider.overrideWithValue(tokens),
      deviceInfoProvider.overrideWithValue((name: 'Браузер', platform: 'web')),
      apiHeadersProvider.overrideWithValue(const {'X-PT-Client': 'web'}),
      // Веб открывается с того же адреса, что и API (Caddy: /api → backend).
      defaultServerUrlProvider.overrideWithValue(Uri.parse(Uri.base.origin)),
      sessionProvider.overrideWith((ref) => session),
    ],
    child: const PromptTreeApp(layout: ShellLayout.adaptive),
  ));
}

/// Браузер не дал открыть локальное хранилище (приватный режим, запрет сайта и т. п.).
/// Без него приложение работать не может: данные пишутся сначала локально.
class StorageErrorApp extends StatelessWidget {
  const StorageErrorApp({super.key, required this.error});
  final Object error;

  @override
  Widget build(BuildContext context) => MaterialApp(
        title: 'PromptTree',
        debugShowCheckedModeBanner: false,
        theme: buildTheme(Brightness.light),
        darkTheme: buildTheme(Brightness.dark),
        home: Builder(builder: (context) {
          final c = context.pt;
          return Scaffold(
            body: Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 420),
                  child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
                    const AppIcon(size: 36),
                    const SizedBox(height: 18),
                    Text('Не удалось открыть хранилище браузера', style: ui(18, weight: FontWeight.w600, color: c.fg)),
                    const SizedBox(height: 8),
                    Text(
                      'PromptTree сначала сохраняет всё в браузере. Проверьте, что сайт не открыт в приватном режиме '
                      'и что хранение данных для него разрешено, затем обновите страницу.',
                      style: ui(14, color: c.muted, height: 1.5),
                    ),
                    const SizedBox(height: 12),
                    Text('$error', style: mono(11, color: c.faint)),
                  ]),
                ),
              ),
            ),
          );
        }),
      );
}
