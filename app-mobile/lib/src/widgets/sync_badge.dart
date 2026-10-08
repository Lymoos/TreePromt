import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../state/providers.dart';

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

    return Semantics(
      label: 'Синхронизация: $label',
      button: true,
      child: InkWell(
        key: const Key('sync-badge'),
        borderRadius: BorderRadius.circular(6),
        onTap: () => ref.read(syncEngineProvider)?.syncNow(),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
          child: Row(mainAxisSize: MainAxisSize.min, children: [
            Container(
              width: 7,
              height: 7,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: filled ? c.fg : Colors.transparent,
                border: Border.all(color: c.fg, width: 1.2),
              ),
            ),
            const SizedBox(width: 6),
            Text(label, style: mono(11, color: c.muted)),
          ]),
        ),
      ),
    );
  }
}
