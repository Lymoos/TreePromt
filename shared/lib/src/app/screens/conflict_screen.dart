import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';
import '../widgets/common.dart';

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
      body: FadeSwitcher(
        expand: true,
        child: first == null || second == null
            ? const EmptyState(
                key: ValueKey('loading'),
                icon: Icons.cloud_sync_outlined,
                title: 'Загружаем версии',
                text: 'Версии ещё загружаются с сервера.',
              )
            : ListView(key: const ValueKey('pair'), padding: const EdgeInsets.fromLTRB(16, 8, 16, 32), children: [
                Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 760),
                    child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                      FadeSlideIn(
                        child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
                          Container(
                            width: 36,
                            height: 36,
                            decoration: BoxDecoration(
                              color: c.panel,
                              borderRadius: BorderRadius.circular(PtRadius.sm + 2),
                              border: Border.all(color: c.line),
                            ),
                            child: Icon(Icons.call_split_rounded, size: 18, color: c.fg),
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: Text(
                              'Текст изменили на двух устройствах, пока они не видели друг друга. Ничего не потеряно: выберите версию или соберите итог.',
                              style: ui(14, color: c.muted, height: 1.5),
                            ),
                          ),
                        ]),
                      ),
                      const SizedBox(height: 18),
                      FadeSlideIn(
                        delay: const Duration(milliseconds: 60),
                        child: _VersionCard(
                          key: const Key('version-first'),
                          index: 1,
                          title: 'Принята первой',
                          text: first.content,
                          onKeep: () => _resolve(first.content),
                        ),
                      ),
                      const SizedBox(height: 12),
                      FadeSlideIn(
                        delay: const Duration(milliseconds: 120),
                        child: _VersionCard(
                          key: const Key('version-second'),
                          index: 2,
                          title: 'Пришла второй',
                          text: second.content,
                          onKeep: () => _resolve(second.content),
                        ),
                      ),
                      const SizedBox(height: 20),
                      AnimatedSize(
                        duration: PtMotion.normal,
                        curve: PtMotion.curve,
                        alignment: Alignment.topCenter,
                        child: _manual == null
                            ? FadeSlideIn(
                                delay: const Duration(milliseconds: 180),
                                child: OutlinedButton.icon(
                                  onPressed: () => setState(() => _manual = TextEditingController(
                                        text: '${first.content}\n\n— — —\n\n${second.content}',
                                      )),
                                  icon: const Icon(Icons.merge_rounded, size: 18),
                                  label: const Text('Собрать вручную'),
                                ),
                              )
                            : FadeSlideIn(
                                child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                                  Text('ИТОГ', style: mono(11, color: c.muted, letterSpacing: 0.8, weight: FontWeight.w500)),
                                  const SizedBox(height: 8),
                                  TextField(
                                    controller: _manual,
                                    maxLines: null,
                                    minLines: 8,
                                    autofocus: true,
                                    style: ui(15, color: c.fg, height: 1.55),
                                    decoration: const InputDecoration(contentPadding: EdgeInsets.all(14)),
                                  ),
                                  const SizedBox(height: 12),
                                  FilledButton.icon(
                                    onPressed: () => _resolve(_manual!.text),
                                    icon: const Icon(Icons.check_rounded, size: 18),
                                    label: const Text('Сохранить итог'),
                                  ),
                                ]),
                              ),
                      ),
                    ]),
                  ),
                ),
              ]),
      ),
    );
  }
}

class _VersionCard extends StatelessWidget {
  const _VersionCard({super.key, required this.index, required this.title, required this.text, required this.onKeep});
  final int index;
  final String title, text;
  final VoidCallback onKeep;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return PtCard(
      padding: const EdgeInsets.fromLTRB(16, 14, 16, 14),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Row(children: [
          Container(
            width: 20,
            height: 20,
            alignment: Alignment.center,
            decoration: BoxDecoration(color: c.fg, shape: BoxShape.circle),
            child: Text('$index', style: mono(10.5, color: c.bg, weight: FontWeight.w600)),
          ),
          const SizedBox(width: 10),
          Text(title.toUpperCase(), style: mono(11, color: c.muted, letterSpacing: 0.8, weight: FontWeight.w500)),
        ]),
        const SizedBox(height: 10),
        SelectableText(text.isEmpty ? '(пусто)' : text, style: ui(15, color: c.fg, height: 1.55)),
        const SizedBox(height: 14),
        Align(
          alignment: Alignment.centerRight,
          child: FilledButton(onPressed: onKeep, child: const Text('Оставить эту')),
        ),
      ]),
    );
  }
}
