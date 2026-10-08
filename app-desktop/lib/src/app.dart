import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/app.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'home_screen.dart';

class PromptTreeDesktopApp extends ConsumerStatefulWidget {
  const PromptTreeDesktopApp({super.key});

  @override
  ConsumerState<PromptTreeDesktopApp> createState() => _State();
}

class _State extends ConsumerState<PromptTreeDesktopApp> with WidgetsBindingObserver {
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
    // Окно снова в фокусе — сразу синхронизируемся.
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
      home: session.mode == SessionMode.none ? const LoginScreen() : const HomeScreen(),
    );
  }
}
