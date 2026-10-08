import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';
import 'dialogs.dart';

/// Пустое дерево: первое действие — создать проект.
class EmptyTree extends StatelessWidget {
  const EmptyTree({super.key, required this.onCreate});
  final VoidCallback onCreate;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 360),
          child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
            AppIcon(size: 36, tile: c.lineStrong, inner: c.bg),
            const SizedBox(height: 18),
            Text('Пока пусто', style: ui(20, weight: FontWeight.w600, color: c.fg)),
            const SizedBox(height: 6),
            Text('Создайте проект. В нём будут папки, черновики и задачи для Claude.',
                style: ui(15, color: c.muted, height: 1.5)),
            const SizedBox(height: 20),
            FilledButton(key: const Key('create-first'), onPressed: onCreate, child: const Text('Создать проект')),
          ]),
        ),
      ),
    );
  }
}

/// Сессия на сервере истекла: данные целы, нужно войти снова.
class SessionNotice extends StatelessWidget {
  const SessionNotice({super.key, required this.onLogin});
  final VoidCallback onLogin;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Container(
      margin: const EdgeInsets.fromLTRB(12, 4, 12, 8),
      padding: const EdgeInsets.fromLTRB(14, 10, 8, 10),
      decoration: BoxDecoration(border: Border.all(color: c.fg), borderRadius: BorderRadius.circular(8)),
      child: Row(children: [
        Expanded(
          child: Text('Сессия истекла. Заметки сохранены на устройстве, войдите снова, чтобы они ушли на сервер.',
              style: ui(13, color: c.fg)),
        ),
        TextButton(onPressed: onLogin, child: const Text('Войти')),
      ]),
    );
  }
}

/// Выход: предупреждаем, если в очереди остались неотправленные изменения.
Future<void> logout(BuildContext context, WidgetRef ref) async {
  final pending = ref.read(pendingCountProvider).valueOrNull ?? 0;
  if (pending > 0) {
    final ok = await confirm(
      context,
      title: 'Выйти?',
      message: 'Ещё $pending изменений не ушли на сервер. Они останутся на устройстве и отправятся после следующего входа.',
      action: 'Выйти',
    );
    if (!ok) return;
  }
  await ref.read(apiProvider)?.logout();
  final session = ref.read(sessionProvider);
  ref.read(sessionProvider.notifier).state = Session(mode: SessionMode.none, serverUrl: session.serverUrl);
}
