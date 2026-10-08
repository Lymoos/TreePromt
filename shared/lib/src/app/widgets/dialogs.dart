import 'package:flutter/material.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

Future<bool> confirm(
  BuildContext context, {
  required String title,
  required String message,
  required String action,
  String cancel = 'Отмена',
}) async {
  final ok = await showPtDialog<bool>(
    context,
    builder: (context) => AlertDialog(
      title: Text(title),
      content: ConstrainedBox(constraints: const BoxConstraints(maxWidth: 360), child: Text(message)),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context, false), child: Text(cancel)),
        FilledButton(onPressed: () => Navigator.pop(context, true), child: Text(action)),
      ],
    ),
  );
  return ok ?? false;
}

/// Поле для имени. Возвращает null, если отменили или ввели пустое.
Future<String?> askName(BuildContext context, {required String title, String initial = '', String action = 'Готово'}) async {
  final name = await showPtDialog<String>(
    context,
    builder: (_) => _NameDialog(title: title, initial: initial, action: action),
  );
  final trimmed = name?.trim();
  return (trimmed == null || trimmed.isEmpty) ? null : trimmed;
}

/// Контроллер живёт столько же, сколько диалог (включая анимацию закрытия).
class _NameDialog extends StatefulWidget {
  const _NameDialog({required this.title, required this.initial, required this.action});
  final String title, initial, action;

  @override
  State<_NameDialog> createState() => _NameDialogState();
}

class _NameDialogState extends State<_NameDialog> {
  late final _controller = TextEditingController(text: widget.initial)
    ..selection = TextSelection(baseOffset: 0, extentOffset: widget.initial.length);

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
        title: Text(widget.title),
        content: SizedBox(
          width: 360,
          child: TextField(
            key: const Key('name-field'),
            controller: _controller,
            autofocus: true,
            maxLength: 200,
            textCapitalization: TextCapitalization.sentences,
            style: ui(16, color: context.pt.fg),
            decoration: const InputDecoration(counterText: '', hintText: 'Название'),
            onSubmitted: (v) => Navigator.pop(context, v),
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: const Text('Отмена')),
          FilledButton(onPressed: () => Navigator.pop(context, _controller.text), child: Text(widget.action)),
        ],
      );
}

/// Строка меню в нижнем листе: значок в плитке, название и пояснение.
class SheetItem extends StatelessWidget {
  const SheetItem({super.key, required this.label, required this.onTap, this.leading, this.caption});
  final String label;
  final String? caption;
  final Widget? leading;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 1),
      child: Pressable(
        onTap: onTap,
        borderRadius: BorderRadius.circular(PtRadius.md + 2),
        pressedScale: 0.98,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
          child: Row(children: [
            Container(
              width: 38,
              height: 38,
              decoration: BoxDecoration(
                color: c.panel,
                borderRadius: BorderRadius.circular(PtRadius.sm + 3),
                border: Border.all(color: c.line),
              ),
              child: Center(child: leading),
            ),
            const SizedBox(width: 14),
            Expanded(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text(label, style: ui(15, weight: FontWeight.w500, color: c.fg)),
                if (caption != null) ...[
                  const SizedBox(height: 1),
                  Text(caption!, style: ui(13, color: c.muted, height: 1.35)),
                ],
              ]),
            ),
          ]),
        ),
      ),
    );
  }
}

/// Нижний лист: пункты появляются «лесенкой».
Future<T?> showSheet<T>(BuildContext context, {String? title, required List<Widget> children}) =>
    showModalBottomSheet<T>(
      context: context,
      showDragHandle: true,
      // На маленьких экранах меню прокручивается, а не обрезается.
      isScrollControlled: true,
      constraints: BoxConstraints(maxHeight: MediaQuery.sizeOf(context).height * 0.85, maxWidth: 560),
      sheetAnimationStyle: AnimationStyle(duration: const Duration(milliseconds: 380), reverseDuration: PtMotion.normal),
      builder: (context) => SafeArea(
        child: SingleChildScrollView(
          child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            if (title != null)
              Padding(
                padding: const EdgeInsets.fromLTRB(24, 0, 24, 10),
                child: Text(title,
                    style: mono(11, color: context.pt.muted, letterSpacing: 0.8, weight: FontWeight.w500),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis),
              ),
            for (var i = 0; i < children.length; i++)
              FadeSlideIn(
                delay: FadeSlideIn.stagger(i, step: 35),
                duration: const Duration(milliseconds: 380),
                offset: const Offset(0, 10),
                child: children[i],
              ),
            const SizedBox(height: 10),
          ]),
        ),
      ),
    );
