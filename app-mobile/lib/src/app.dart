import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'screens/login_screen.dart';
import 'screens/tree_screen.dart';
import 'state/providers.dart';

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

class PromptTreeApp extends ConsumerStatefulWidget {
  const PromptTreeApp({super.key});

  @override
  ConsumerState<PromptTreeApp> createState() => _PromptTreeAppState();
}

class _PromptTreeAppState extends ConsumerState<PromptTreeApp> with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // Вернулись в приложение — сразу синхронизируемся.
    if (state == AppLifecycleState.resumed) ref.read(syncEngineProvider)?.schedule(Duration.zero);
  }

  @override
  Widget build(BuildContext context) {
    final session = ref.watch(sessionProvider);
    return MaterialApp(
      title: 'PromptTree',
      debugShowCheckedModeBanner: false,
      theme: buildTheme(Brightness.light),
      darkTheme: buildTheme(Brightness.dark),
      home: session.mode == SessionMode.none ? const LoginScreen() : const TreeScreen(),
    );
  }
}
