import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';
import '../widgets/common.dart';
import '../widgets/dialogs.dart';

String reasonLabel(String r) => switch (r) {
      VersionReason.checkpoint => 'Контрольная точка',
      VersionReason.beforeRestore => 'До отката',
      VersionReason.beforeStructure => 'До структурирования',
      VersionReason.conflictServer => 'Конфликт: принята первой',
      VersionReason.conflictLocal => 'Конфликт: пришла второй',
      _ => r,
    };

String formatTime(DateTime t) {
  final l = t.toLocal();
  String two(int v) => v.toString().padLeft(2, '0');
  return '${two(l.day)}.${two(l.month)}.${l.year} ${two(l.hour)}:${two(l.minute)}';
}

/// История версий — отдельно от Ctrl+Z редактора (ТЗ п. 6.3).
class VersionsScreen extends ConsumerWidget {
  const VersionsScreen({super.key, required this.nodeId});
  final String nodeId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.pt;
    final versions = ref.watch(versionsProvider(nodeId)).valueOrNull ?? const <ContentVersion>[];
    return Scaffold(
      appBar: AppBar(title: const Text('История версий')),
      body: FadeSwitcher(
        expand: true,
        child: versions.isEmpty
            ? const EmptyState(
                key: ValueKey('empty'),
                icon: Icons.history_rounded,
                title: 'Версий пока нет',
                text: 'Они появляются перед откатом, при конфликте и после паузы в редактировании больше 5 минут. '
                    'История приходит с сервера.',
              )
            : ListView.builder(
                key: const ValueKey('list'),
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 32),
                itemCount: versions.length,
                itemBuilder: (context, i) {
                  final v = versions[i];
                  final last = i == versions.length - 1;
                  return FadeSlideIn(
                    key: ValueKey(v.id),
                    delay: FadeSlideIn.stagger(i),
                    child: Center(
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(maxWidth: 720),
                        // Лента времени: точка и линия слева, карточка версии справа.
                        child: IntrinsicHeight(
                          child: Row(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                            SizedBox(
                              width: 24,
                              child: Column(children: [
                                const SizedBox(height: 20),
                                Container(
                                  width: 9,
                                  height: 9,
                                  decoration: BoxDecoration(
                                    shape: BoxShape.circle,
                                    color: i == 0 ? c.fg : c.bg,
                                    border: Border.all(color: c.fg, width: 1.5),
                                  ),
                                ),
                                if (!last) Expanded(child: Container(width: 1.5, color: c.line)),
                              ]),
                            ),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Padding(
                                padding: const EdgeInsets.only(bottom: 10),
                                child: PtCard(
                                  onTap: () => _preview(context, ref, v),
                                  padding: const EdgeInsets.fromLTRB(14, 12, 14, 12),
                                  child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                                    Row(children: [
                                      if (v.field == 'structured') ...[
                                        Container(
                                          padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 1),
                                          decoration: BoxDecoration(
                                            color: c.fg,
                                            borderRadius: BorderRadius.circular(99),
                                          ),
                                          child: Text('Структура',
                                              style: ui(11, weight: FontWeight.w600, color: c.bg)),
                                        ),
                                        const SizedBox(width: 8),
                                      ],
                                      Expanded(
                                        child: Text(reasonLabel(v.reason),
                                            style: ui(14, weight: FontWeight.w600, color: c.fg)),
                                      ),
                                      Text(formatTime(v.createdAt), style: mono(11, color: c.faint)),
                                    ]),
                                    const SizedBox(height: 6),
                                    Text(v.content.isEmpty ? '(пусто)' : v.content,
                                        maxLines: 2,
                                        overflow: TextOverflow.ellipsis,
                                        style: ui(14, color: c.muted, height: 1.45)),
                                  ]),
                                ),
                              ),
                            ),
                          ]),
                        ),
                      ),
                    ),
                  );
                },
              ),
      ),
    );
  }

  Future<void> _preview(BuildContext context, WidgetRef ref, ContentVersion v) async {
    final restore = await showPtDialog<bool>(
      context,
      builder: (context) => AlertDialog(
        title: Text(reasonLabel(v.reason)),
        content: SizedBox(
          width: 560,
          child: SingleChildScrollView(child: SelectableText(v.content, style: ui(14, color: context.pt.fg, height: 1.5))),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Закрыть')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Восстановить')),
        ],
      ),
    );
    if (restore != true || !context.mounted) return;
    if (!await confirm(context,
        title: 'Восстановить эту версию?',
        message: 'Текущий текст сохранится в истории, его можно будет вернуть.',
        action: 'Восстановить')) {
      return;
    }
    await ref.read(treeServiceProvider).restoreVersion(nodeId, v.id);
    if (context.mounted) Navigator.pop(context);
  }
}
