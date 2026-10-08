import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';
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
    final versions = (ref.watch(versionsProvider(nodeId)).valueOrNull ?? const <ContentVersion>[])
        .where((v) => v.field == 'raw')
        .toList();
    return Scaffold(
      appBar: AppBar(title: const Text('История версий')),
      body: versions.isEmpty
          ? Padding(
              padding: const EdgeInsets.all(24),
              child: Text(
                'Версий пока нет. Они появляются перед откатом, при конфликте и после паузы в редактировании больше 5 минут. '
                'История приходит с сервера.',
                style: ui(15, color: c.muted, height: 1.5),
              ),
            )
          : ListView.separated(
              padding: const EdgeInsets.symmetric(vertical: 8),
              itemCount: versions.length,
              separatorBuilder: (_, __) => Divider(color: c.line, indent: 20, endIndent: 20),
              itemBuilder: (context, i) {
                final v = versions[i];
                return InkWell(
                  onTap: () => _preview(context, ref, v),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
                    child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      Row(children: [
                        Expanded(child: Text(reasonLabel(v.reason), style: ui(14, weight: FontWeight.w500, color: c.fg))),
                        Text(formatTime(v.createdAt), style: mono(11, color: c.faint)),
                      ]),
                      const SizedBox(height: 4),
                      Text(v.content.isEmpty ? '(пусто)' : v.content,
                          maxLines: 2, overflow: TextOverflow.ellipsis, style: ui(14, color: c.muted)),
                    ]),
                  ),
                );
              },
            ),
    );
  }

  Future<void> _preview(BuildContext context, WidgetRef ref, ContentVersion v) async {
    final restore = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(reasonLabel(v.reason)),
        content: SizedBox(
          width: double.maxFinite,
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
