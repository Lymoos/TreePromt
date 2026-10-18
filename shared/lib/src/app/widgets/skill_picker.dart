import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';

// Скиллы задачи и проекта (этап 7.9, design/project-v1.html): чипы и список с поиском.
// Обязательный скилл проекта — чип в рамке, запрещённый — зачёркнутый пунктирный, выбранный — залитый.

enum SkillChipKind { picked, locked, forbidden }

class SkillChip extends StatelessWidget {
  const SkillChip(this.name, {super.key, this.kind = SkillChipKind.picked, this.onRemove});
  final String name;
  final SkillChipKind kind;
  final VoidCallback? onRemove;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    final text = Text(
      name,
      style: mono(12, color: kind == SkillChipKind.forbidden ? c.muted : c.fg).copyWith(
        decoration: kind == SkillChipKind.forbidden ? TextDecoration.lineThrough : null,
        decorationColor: c.muted,
      ),
    );
    final child = Row(mainAxisSize: MainAxisSize.min, children: [
      text,
      if (onRemove != null) ...[
        const SizedBox(width: 6),
        InkWell(
          onTap: onRemove,
          borderRadius: BorderRadius.circular(PtRadius.sm),
          child: Icon(Icons.close_rounded, size: 13, color: c.muted, semanticLabel: 'Убрать $name'),
        ),
      ],
    ]);
    const pad = EdgeInsets.symmetric(horizontal: 8, vertical: 6);
    if (kind == SkillChipKind.forbidden) {
      return CustomPaint(
        painter: DashedBorder(color: c.lineStrong),
        child: Padding(padding: pad, child: child),
      );
    }
    return Container(
      padding: pad,
      decoration: BoxDecoration(
        color: kind == SkillChipKind.picked ? c.selected : Colors.transparent,
        border: kind == SkillChipKind.locked ? Border.all(color: c.lineStrong) : null,
        borderRadius: BorderRadius.circular(PtRadius.sm),
      ),
      child: child,
    );
  }
}

/// «+ скилл» — пунктирный чип-кнопка.
class AddSkillChip extends StatelessWidget {
  const AddSkillChip({super.key, required this.onTap, this.label = '+ скилл'});
  final VoidCallback onTap;
  final String label;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return InkWell(
      key: const Key('add-skill'),
      onTap: onTap,
      borderRadius: BorderRadius.circular(PtRadius.sm),
      child: CustomPaint(
        painter: DashedBorder(color: c.lineStrong),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
          child: Text(label, style: mono(12, color: c.muted)),
        ),
      ),
    );
  }
}

class DashedBorder extends CustomPainter {
  const DashedBorder({required this.color, this.radius = PtRadius.sm});
  final Color color;
  final double radius;

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1;
    final path = Path()..addRRect(RRect.fromRectAndRadius(Offset.zero & size, Radius.circular(radius)));
    for (final m in path.computeMetrics()) {
      for (var d = 0.0; d < m.length; d += 7) {
        canvas.drawPath(m.extractPath(d, (d + 4).clamp(0, m.length)), paint);
      }
    }
  }

  @override
  bool shouldRepaint(DashedBorder oldDelegate) => oldDelegate.color != color;
}

/// Выбор скиллов: список с поиском (ПК — окно, телефон — лист снизу). [locked] отмечены и не снимаются.
/// Возвращает новый выбор или null, если закрыли без изменений.
Future<List<String>?> pickSkills(BuildContext context, WidgetRef ref,
    {required List<String> selected, List<String> locked = const [], List<String> hidden = const [], String title = 'Скиллы задачи'}) async {
  final api = ref.read(apiProvider);
  if (api == null) return null;
  List<SkillInfo> catalog;
  try {
    catalog = await api.skillCatalog();
  } catch (e) {
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e is NetworkException ? 'Нет связи с сервером.' : 'Каталог скиллов не загрузился: $e')));
    }
    return null;
  }
  if (!context.mounted) return null;
  catalog = catalog.where((s) => !hidden.contains(s.name)).toList();
  final body = _SkillList(catalog: catalog, selected: selected, locked: locked, title: title);
  final wide = MediaQuery.sizeOf(context).width >= 700;
  return wide
      ? showDialog<List<String>>(context: context, builder: (_) => Dialog(child: SizedBox(width: 460, height: 520, child: body)))
      : showModalBottomSheet<List<String>>(
          context: context,
          isScrollControlled: true,
          showDragHandle: true,
          builder: (_) => SizedBox(height: MediaQuery.sizeOf(context).height * 0.7, child: body),
        );
}

class _SkillList extends StatefulWidget {
  const _SkillList({required this.catalog, required this.selected, required this.locked, required this.title});
  final List<SkillInfo> catalog;
  final List<String> selected, locked;
  final String title;

  @override
  State<_SkillList> createState() => _SkillListState();
}

class _SkillListState extends State<_SkillList> {
  late final Set<String> _picked = {...widget.selected};
  String _q = '';

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    final q = _q.trim().toLowerCase();
    final items = widget.catalog
        .where((s) => q.isEmpty || s.name.contains(q) || s.description.toLowerCase().contains(q))
        .toList();
    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 12, 12, 12),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(4, 0, 4, 10),
          child: Text(widget.title, style: ui(16, weight: FontWeight.w600, color: c.fg)),
        ),
        TextField(
          key: const Key('skill-search'),
          autofocus: MediaQuery.sizeOf(context).width >= 700,
          onChanged: (v) => setState(() => _q = v),
          decoration: const InputDecoration(hintText: 'Найти скилл', prefixIcon: Icon(Icons.search_rounded, size: 18)),
        ),
        const SizedBox(height: 8),
        Expanded(
          child: items.isEmpty
              ? Center(child: Text('Ничего не найдено', style: ui(13, color: c.muted)))
              : ListView.builder(
                  itemCount: items.length,
                  itemBuilder: (_, i) {
                    final s = items[i];
                    final locked = widget.locked.contains(s.name);
                    final on = locked || _picked.contains(s.name);
                    return InkWell(
                      key: Key('skill-${s.name}'),
                      borderRadius: BorderRadius.circular(10),
                      onTap: locked ? null : () => setState(() => on ? _picked.remove(s.name) : _picked.add(s.name)),
                      child: Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 8),
                        child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
                          Padding(
                            padding: const EdgeInsets.only(top: 1),
                            child: Icon(
                              locked ? Icons.lock_outline_rounded : (on ? Icons.check_box_rounded : Icons.check_box_outline_blank_rounded),
                              size: 18,
                              color: c.fg,
                            ),
                          ),
                          const SizedBox(width: 10),
                          Expanded(
                            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                              Text(s.name, style: mono(13, weight: FontWeight.w500, color: c.fg)),
                              const SizedBox(height: 2),
                              Text(
                                locked ? 'обязательный для проекта' : s.description,
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                                style: ui(12.5, color: c.muted, height: 1.35),
                              ),
                            ]),
                          ),
                        ]),
                      ),
                    );
                  },
                ),
        ),
        const SizedBox(height: 8),
        Row(mainAxisAlignment: MainAxisAlignment.end, children: [
          TextButton(onPressed: () => Navigator.pop(context), child: const Text('Отмена')),
          const SizedBox(width: 8),
          FilledButton(
            key: const Key('skills-done'),
            onPressed: () => Navigator.pop(context, widget.catalog.map((s) => s.name).where(_picked.contains).toList()),
            child: const Text('Готово'),
          ),
        ]),
      ]),
    );
  }
}

/// Строка «Скиллы:» под заголовком AI Task. Только при подключённом сервере.
class TaskSkillBar extends ConsumerStatefulWidget {
  const TaskSkillBar({super.key, required this.nodeId, required this.projectId});
  final String nodeId, projectId;

  @override
  ConsumerState<TaskSkillBar> createState() => _TaskSkillBarState();
}

class _TaskSkillBarState extends ConsumerState<TaskSkillBar> {
  List<String>? _skills;
  List<String> _required = const [], _forbidden = const [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final api = ref.read(apiProvider);
    if (api == null) return;
    try {
      final skills = await api.nodeSkills(widget.nodeId);
      final repos = await api.execRepositories();
      final repo = repos.where((r) => r.projectId == widget.projectId).firstOrNull;
      if (!mounted) return;
      setState(() {
        _skills = skills;
        _required = repo?.requiredSkills ?? const [];
        _forbidden = repo?.forbiddenSkills ?? const [];
      });
    } catch (_) {
      // Заметка ещё не на сервере или нет связи: строка просто не показывается.
    }
  }

  Future<void> _save(List<String> skills) async {
    final api = ref.read(apiProvider);
    if (api == null) return;
    final before = _skills;
    setState(() => _skills = skills);
    try {
      final saved = await api.setNodeSkills(widget.nodeId, skills);
      if (mounted) setState(() => _skills = saved);
    } catch (e) {
      if (!mounted) return;
      setState(() => _skills = before);
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Скиллы не сохранились: ${e is NetworkException ? 'нет связи с сервером' : e}')));
    }
  }

  @override
  Widget build(BuildContext context) {
    final skills = _skills;
    if (skills == null) return const SizedBox.shrink();
    final c = context.pt;
    final own = skills.where((s) => !_required.contains(s)).toList();
    return Wrap(spacing: 6, runSpacing: 6, crossAxisAlignment: WrapCrossAlignment.center, children: [
      Text('Скиллы:', style: ui(13, color: c.muted)),
      for (final s in _required) SkillChip(s, kind: SkillChipKind.locked),
      for (final s in own) SkillChip(s, onRemove: () => _save(skills.where((x) => x != s).toList())),
      AddSkillChip(onTap: () async {
        final picked = await pickSkills(context, ref, selected: skills, locked: _required, hidden: _forbidden);
        if (picked != null) await _save(picked.where((s) => !_required.contains(s)).toList());
      }),
    ]);
  }
}
