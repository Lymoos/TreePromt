import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'providers.dart';
import 'screens/login_screen.dart';
import 'shells/desktop_shell.dart';
import 'shells/mobile_shell.dart';

/// Раскладка: телефон — дерево и редактор отдельными экранами,
/// ПК — две панели, адаптивная (веб) — по ширине окна.
enum ShellLayout { mobile, desktop, adaptive }

/// Ширина, начиная с которой показываем две панели.
const desktopBreakpoint = 840.0;

/// Корневой виджет приложения, общий для всех платформ.
class PromptTreeApp extends ConsumerStatefulWidget {
  const PromptTreeApp({super.key, required this.layout});
  final ShellLayout layout;

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
    // Вернулись в приложение (или окно/вкладка снова в фокусе) — сразу синхронизируемся.
    if (state == AppLifecycleState.resumed) ref.read(syncEngineProvider)?.schedule(Duration.zero);
  }

  Widget _shell(BuildContext context) => switch (widget.layout) {
        ShellLayout.mobile => const MobileShell(),
        ShellLayout.desktop => const DesktopShell(),
        ShellLayout.adaptive => MediaQuery.sizeOf(context).width >= desktopBreakpoint
            ? const DesktopShell()
            : const MobileShell(),
      };

  @override
  Widget build(BuildContext context) {
    final session = ref.watch(sessionProvider);
    return MaterialApp(
      title: 'PromptTree',
      debugShowCheckedModeBanner: false,
      theme: buildTheme(Brightness.light),
      darkTheme: buildTheme(Brightness.dark),
      themeAnimationDuration: PtMotion.slow,
      themeAnimationCurve: PtMotion.curve,
      // Вход ↔ приложение: мягкая смена, а не резкий скачок.
      home: AnimatedSwitcher(
        duration: const Duration(milliseconds: 450),
        switchInCurve: PtMotion.curve,
        switchOutCurve: Curves.easeIn,
        transitionBuilder: (child, a) => FadeTransition(
          opacity: a,
          child: ScaleTransition(scale: Tween<double>(begin: 0.985, end: 1).animate(a), child: child),
        ),
        child: session.mode == SessionMode.none
            ? const LoginScreen(key: ValueKey('login'))
            : Builder(key: const ValueKey('app'), builder: _shell),
      ),
    );
  }
}
