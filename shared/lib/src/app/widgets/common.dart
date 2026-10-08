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
    Widget step(int i, Widget child) => FadeSlideIn(delay: FadeSlideIn.stagger(i, step: 70), child: child);
    Widget legend(MarkerType m, String name, String text) => Padding(
          padding: const EdgeInsets.symmetric(vertical: 5),
          child: Row(children: [
            SizedBox(width: 14, child: Center(child: Marker(m, size: m == MarkerType.project ? 9 : 10, color: c.fg))),
            const SizedBox(width: 10),
            Text(name, style: ui(13.5, weight: FontWeight.w600, color: c.fg)),
            const SizedBox(width: 6),
            Expanded(child: Text(text, style: ui(13.5, color: c.muted), overflow: TextOverflow.ellipsis)),
          ]),
        );

    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(28),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 360),
          child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
            Container(
              width: 76,
              height: 76,
              decoration: BoxDecoration(
                color: c.panel,
                borderRadius: BorderRadius.circular(PtRadius.xl),
                border: Border.all(color: c.line),
              ),
              child: const Center(child: AnimatedAppIcon(size: 40, delay: Duration(milliseconds: 150))),
            ),
            const SizedBox(height: 22),
            step(1, Text('Пока пусто', style: ui(22, weight: FontWeight.w700, color: c.fg, letterSpacing: -0.3))),
            const SizedBox(height: 8),
            step(
              2,
              Text('Создайте проект. В нём будут папки, черновики и задачи для Claude.',
                  style: ui(15, color: c.muted, height: 1.5)),
            ),
            const SizedBox(height: 18),
            step(
              3,
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                decoration: BoxDecoration(
                  color: c.panel,
                  borderRadius: BorderRadius.circular(PtRadius.md + 2),
                  border: Border.all(color: c.line),
                ),
                child: Column(children: [
                  legend(MarkerType.project, 'Проект', 'большая тема'),
                  legend(MarkerType.aiNone, 'AI Task', 'задача для Claude'),
                  legend(MarkerType.rawNote, 'Raw Note', 'мысль на ходу'),
                ]),
              ),
            ),
            const SizedBox(height: 22),
            step(
              4,
              FilledButton.icon(
                key: const Key('create-first'),
                onPressed: onCreate,
                icon: const Icon(Icons.add_rounded, size: 18),
                label: const Text('Создать проект'),
              ),
            ),
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
    return FadeSlideIn(
      offset: const Offset(0, -8),
      child: Container(
        margin: const EdgeInsets.fromLTRB(12, 4, 12, 8),
        padding: const EdgeInsets.fromLTRB(14, 12, 8, 12),
        decoration: BoxDecoration(
          color: c.card,
          border: Border.all(color: c.lineStrong),
          borderRadius: BorderRadius.circular(PtRadius.md + 2),
          boxShadow: c.elevation(0.6),
        ),
        child: Row(children: [
          Icon(Icons.lock_clock_outlined, size: 18, color: c.fg),
          const SizedBox(width: 10),
          Expanded(
            child: Text('Сессия истекла. Заметки сохранены на устройстве, войдите снова, чтобы они ушли на сервер.',
                style: ui(13, color: c.fg, height: 1.4)),
          ),
          TextButton(onPressed: onLogin, child: const Text('Войти')),
        ]),
      ),
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

/// Пустой экран: значок в круге, заголовок и пояснение.
class EmptyState extends StatelessWidget {
  const EmptyState({super.key, required this.icon, required this.title, required this.text});
  final IconData icon;
  final String title, text;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(32),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 340),
          child: Column(mainAxisSize: MainAxisSize.min, children: [
            FadeSlideIn(
              scale: 0.8,
              offset: Offset.zero,
              child: Container(
                width: 72,
                height: 72,
                decoration: BoxDecoration(color: c.panel, shape: BoxShape.circle, border: Border.all(color: c.line)),
                child: Icon(icon, size: 30, color: c.muted),
              ),
            ),
            const SizedBox(height: 20),
            FadeSlideIn(
              delay: const Duration(milliseconds: 80),
              child: Text(title,
                  textAlign: TextAlign.center, style: ui(19, weight: FontWeight.w700, color: c.fg, letterSpacing: -0.2)),
            ),
            const SizedBox(height: 8),
            FadeSlideIn(
              delay: const Duration(milliseconds: 140),
              child: Text(text, textAlign: TextAlign.center, style: ui(14.5, color: c.muted, height: 1.5)),
            ),
          ]),
        ),
      ),
    );
  }
}

/// Карточка: поверхность с тонкой рамкой и мягкой тенью.
class PtCard extends StatelessWidget {
  const PtCard({super.key, required this.child, this.padding = const EdgeInsets.all(16), this.onTap});
  final Widget child;
  final EdgeInsets padding;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Pressable(
      onTap: onTap,
      color: c.card,
      hoverColor: onTap == null ? c.card : Color.alphaBlend(c.hover.withValues(alpha: 0.6), c.card),
      border: Border.all(color: c.line),
      shadow: c.elevation(0.5),
      borderRadius: BorderRadius.circular(PtRadius.lg),
      pressedScale: onTap == null ? 1 : 0.985,
      child: Padding(padding: padding, child: child),
    );
  }
}
