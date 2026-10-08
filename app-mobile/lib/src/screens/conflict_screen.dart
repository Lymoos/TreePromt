import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../state/providers.dart';

/// Разбор конфликта: обе версии видны целиком, пользователь выбирает одну
/// или собирает итог вручную (архитектура п. 4.3, решение «всегда обе версии»).
class ConflictScreen extends ConsumerStatefulWidget {
  const ConflictScreen({super.key, required this.nodeId});
  final String nodeId;

  @override
  ConsumerState<ConflictScreen> createState() => _ConflictScreenState();
}

class _ConflictScreenState extends ConsumerState<ConflictScreen> {
  TextEditingController? _manual;

  @override
  void dispose() {
    _manual?.dispose();
    super.dispose();
  }

  Future<void> _resolve(String text) async {
    await ref.read(treeServiceProvider).resolveConflict(widget.nodeId, text);
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Конфликт разобран')));
    Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    final versions = ref.watch(versionsProvider(widget.nodeId)).valueOrNull ?? const <ContentVersion>[];
    final conflicts = versions
        .where((v) => v.reason == VersionReason.conflictServer || v.reason == VersionReason.conflictLocal)
        .toList();
    final lastRev = conflicts.isEmpty ? null : conflicts.map((v) => v.serverRevision).reduce((a, b) => a > b ? a : b);
    final pair = conflicts.where((v) => v.serverRevision == lastRev);
    final first = pair.where((v) => v.reason == VersionReason.conflictServer).firstOrNull;
    final second = pair.where((v) => v.reason == VersionReason.conflictLocal).firstOrNull;

    return Scaffold(
      appBar: AppBar(title: const Text('Две версии текста')),
      body: first == null || second == null
          ? Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Text('Версии ещё загружаются с сервера.', style: ui(15, color: c.muted)),
              ),
            )
          : ListView(padding: const EdgeInsets.fromLTRB(16, 8, 16, 32), children: [
              Text(
                'Текст изменили на двух устройствах, пока они не видели друг друга. Ничего не потеряно: выберите версию или соберите итог.',
                style: ui(14, color: c.muted, height: 1.5),
              ),
              const SizedBox(height: 16),
              _VersionCard(
                key: const Key('version-first'),
                title: 'Принята первой',
                text: first.content,
                onKeep: () => _resolve(first.content),
              ),
              const SizedBox(height: 12),
              _VersionCard(
                key: const Key('version-second'),
                title: 'Пришла второй',
                text: second.content,
                onKeep: () => _resolve(second.content),
              ),
              const SizedBox(height: 20),
              if (_manual == null)
                OutlinedButton(
                  onPressed: () => setState(() => _manual = TextEditingController(
                        text: '${first.content}\n\n— — —\n\n${second.content}',
                      )),
                  child: const Text('Собрать вручную'),
                )
              else ...[
                Text('ИТОГ', style: mono(11, color: c.muted, letterSpacing: 0.8)),
                const SizedBox(height: 8),
                TextField(
                  controller: _manual,
                  maxLines: null,
                  minLines: 8,
                  style: ui(15, color: c.fg, height: 1.55),
                  decoration: const InputDecoration(contentPadding: EdgeInsets.all(12)),
                ),
                const SizedBox(height: 12),
                FilledButton(onPressed: () => _resolve(_manual!.text), child: const Text('Сохранить итог')),
              ],
            ]),
    );
  }
}

class _VersionCard extends StatelessWidget {
  const _VersionCard({super.key, required this.title, required this.text, required this.onKeep});
  final String title, text;
  final VoidCallback onKeep;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Container(
      decoration: BoxDecoration(border: Border.all(color: c.lineStrong), borderRadius: BorderRadius.circular(10)),
      padding: const EdgeInsets.fromLTRB(14, 12, 14, 12),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Text(title.toUpperCase(), style: mono(11, color: c.muted, letterSpacing: 0.8)),
        const SizedBox(height: 8),
        SelectableText(text.isEmpty ? '(пусто)' : text, style: ui(15, color: c.fg, height: 1.55)),
        const SizedBox(height: 12),
        Align(
          alignment: Alignment.centerRight,
          child: FilledButton(onPressed: onKeep, child: const Text('Оставить эту')),
        ),
      ]),
    );
  }
}
