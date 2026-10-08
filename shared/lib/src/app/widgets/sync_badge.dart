import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';

/// Состояние синхронизации в шапке: коротко, моноширинным, без цвета.
class SyncBadge extends ConsumerWidget {
  const SyncBadge({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.pt;
    final session = ref.watch(sessionProvider);
    final pending = ref.watch(pendingCountProvider).valueOrNull ?? 0;
    final state = ref.watch(syncStateProvider).valueOrNull ?? const SyncState();

    final (String label, bool filled) = switch (session.mode) {
      SessionMode.local => ('локально', false),
      _ => switch (state.phase) {
          SyncPhase.syncing => ('синхр…', true),
          SyncPhase.offline => (pending > 0 ? 'офлайн · $pending' : 'офлайн', false),
          SyncPhase.authRequired => ('нужен вход', false),
          SyncPhase.error => ('ошибка · $pending', false),
          SyncPhase.idle => (pending > 0 ? 'в очереди · $pending' : 'синхр.', true),
        },
    };

    final syncing = session.mode == SessionMode.server && state.phase == SyncPhase.syncing;

    return Semantics(
      label: 'Синхронизация: $label',
      button: true,
      child: Tooltip(
        message: session.mode == SessionMode.local ? 'Заметки хранятся только на этом устройстве' : 'Синхронизировать сейчас',
        child: Pressable(
          key: const Key('sync-badge'),
          onTap: () => ref.read(syncEngineProvider)?.syncNow(),
          borderRadius: BorderRadius.circular(99),
          color: c.panel,
          border: Border.all(color: c.line),
          pressedScale: 0.95,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
            child: Row(mainAxisSize: MainAxisSize.min, children: [
              SizedBox(
                width: 8,
                height: 8,
                child: syncing
                    ? CircularProgressIndicator(strokeWidth: 1.4, color: c.fg)
                    : AnimatedContainer(
                        duration: PtMotion.normal,
                        curve: PtMotion.curve,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          color: filled ? c.fg : Colors.transparent,
                          border: Border.all(color: c.fg, width: 1.3),
                        ),
                      ),
              ),
              const SizedBox(width: 7),
              AnimatedSize(
                duration: PtMotion.normal,
                curve: PtMotion.curve,
                child: AnimatedSwitcher(
                  duration: PtMotion.normal,
                  transitionBuilder: (child, a) => FadeTransition(opacity: a, child: child),
                  child: Text(label, key: ValueKey(label), style: mono(11, color: c.muted)),
                ),
              ),
            ]),
          ),
        ),
      ),
    );
  }
}
