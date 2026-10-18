import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';
import '../widgets/skill_picker.dart';

// Окно проекта (этап 7.9, design/project-v1.html, решения 19.10.2026): описание для ИИ и файлы,
// код (папка на ПК и git-репозиторий вместе), скиллы проекта. Работает только с подключённым сервером.

/// Проект, открытый в окне проекта на ПК (на месте редактора); null — окно закрыто.
final projectWindowProvider = StateProvider<String?>((ref) => null);

/// Телефон: окно проекта отдельным экраном.
class ProjectWindowScreen extends StatelessWidget {
  const ProjectWindowScreen({super.key, required this.projectId});
  final String projectId;

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(),
        body: SafeArea(top: false, child: ProjectWindow(projectId: projectId, compact: true)),
      );
}

enum _Tab { about, code, skills }

class ProjectWindow extends ConsumerStatefulWidget {
  const ProjectWindow({super.key, required this.projectId, this.compact = false, this.onClose});
  final String projectId;
  final bool compact;
  final VoidCallback? onClose;

  @override
  ConsumerState<ProjectWindow> createState() => _ProjectWindowState();
}

class _ProjectWindowState extends ConsumerState<ProjectWindow> {
  _Tab _tab = _Tab.about;
  ProjectContextData? _context;
  List<ExecRepository> _repos = const [];
  List<WorkerView> _workers = const [];
  String? _error;
  bool _busy = false;

  final _desc = TextEditingController();
  final _path = TextEditingController();
  final _git = TextEditingController();
  final _branch = TextEditingController(text: 'main');
  String? _hostId;
  String _template = '';
  bool _descDirty = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    for (final c in [_desc, _path, _git, _branch]) {
      c.dispose();
    }
    super.dispose();
  }

  ApiClient? get _api => ref.read(apiProvider);

  ExecRepository? get _repo => _repos.where((r) => r.projectId == widget.projectId).firstOrNull;

  String _explain(Object e) => switch (e) {
        NetworkException() => 'Нет связи с сервером.',
        ApiException(code: 'not_found') => 'Сервер ещё не знает этот проект: дождитесь синхронизации и откройте окно снова.',
        ApiException(code: 'forbidden') => 'Менять проект может только владелец или редактор.',
        ApiException(code: 'file_too_large') => 'Файл больше 20 МБ или все файлы проекта вместе больше 200 МБ.',
        ApiException(code: 'invalid_input') => 'Сервер не принял данные: проверьте путь и git-адрес (https://… или git@…).',
        ApiException(:final message) => 'Сервер отказал: $message',
        _ => '$e',
      };

  Future<void> _load() async {
    final api = _api;
    if (api == null) return;
    try {
      final results = await Future.wait([api.projectContext(widget.projectId), api.execRepositories(), api.execWorkers()]);
      if (!mounted) return;
      final ctx = results[0] as ProjectContextData;
      final repos = results[1] as List<ExecRepository>;
      final workers = results[2] as List<WorkerView>;
      setState(() {
        _context = ctx;
        _repos = repos;
        _workers = workers;
        _error = null;
        if (!_descDirty) _desc.text = ctx.description;
        final repo = _repo;
        if (repo != null) {
          _path.text = repo.localPath;
          _git.text = repo.gitUrl;
          _branch.text = repo.defaultBranch;
          _template = repo.template;
          _hostId = repo.hostId.isEmpty ? null : repo.hostId;
        }
        _hostId ??= _hosts.firstOrNull?.$1;
      });
    } catch (e) {
      if (mounted) setState(() => _error = _explain(e));
    }
  }

  /// ПК со службой AiCrew: (id, имя, в сети).
  List<(String, String, bool)> get _hosts {
    final seen = <String>{};
    return [
      for (final w in _workers)
        if (w.hostId.isNotEmpty && seen.add(w.hostId)) (w.hostId, w.hostName, w.online),
    ];
  }

  Future<void> _run(Future<void> Function(ApiClient api) action, {String? done}) async {
    final api = _api;
    if (api == null) return;
    setState(() => _busy = true);
    try {
      await action(api);
      if (done != null && mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(done)));
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(_explain(e))));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _addFiles() async {
    final files = await openFiles();
    if (files.isEmpty) return;
    await _run((api) async {
      for (final f in files) {
        await api.uploadProjectFile(widget.projectId, f.name, await f.readAsBytes(), contentType: f.mimeType);
      }
    }, done: files.length == 1 ? 'Файл добавлен' : 'Файлы добавлены');
  }

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    final projects = ref.watch(projectsProvider).valueOrNull ?? const <Project>[];
    final project = projects.where((p) => p.id == widget.projectId).firstOrNull;
    final hPad = widget.compact ? 16.0 : 28.0;

    final header = Padding(
      padding: EdgeInsets.fromLTRB(hPad, widget.compact ? 4 : 22, widget.compact ? 8 : 16, 0),
      child: Row(children: [
        const Marker(MarkerType.project, size: 10),
        const SizedBox(width: 10),
        Expanded(
          child: Text(project?.name ?? 'Проект',
              overflow: TextOverflow.ellipsis, style: ui(widget.compact ? 20 : 22, weight: FontWeight.w700, color: c.fg)),
        ),
        if (_context != null)
          Text('проект · ${plural(_context!.files.length, 'файл', 'файла', 'файлов')}', style: mono(12, color: c.muted)),
        if (widget.onClose != null)
          IconButton(tooltip: 'Закрыть', onPressed: widget.onClose, icon: const Icon(Icons.close_rounded, size: 20)),
      ]),
    );

    final tabs = Padding(
      padding: EdgeInsets.symmetric(horizontal: hPad),
      child: DecoratedBox(
        decoration: BoxDecoration(border: Border(bottom: BorderSide(color: c.line))),
        child: Row(children: [
          for (final (t, label) in [(_Tab.about, 'О проекте'), (_Tab.code, 'Код'), (_Tab.skills, 'Скиллы')])
            InkWell(
              key: Key('project-tab-${t.name}'),
              onTap: () => setState(() => _tab = t),
              child: AnimatedContainer(
                duration: PtMotion.fast,
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                decoration: BoxDecoration(
                  border: Border(bottom: BorderSide(color: _tab == t ? c.fg : Colors.transparent, width: 2)),
                ),
                child: Text(label,
                    style: ui(14, weight: _tab == t ? FontWeight.w500 : FontWeight.w400, color: _tab == t ? c.fg : c.muted)),
              ),
            ),
        ]),
      ),
    );

    final Widget body;
    if (_error != null) {
      body = _Section(title: 'Окно проекта', child: Text(_error!, style: ui(14, color: c.fg)));
    } else if (_context == null) {
      body = const Padding(padding: EdgeInsets.all(32), child: Center(child: CircularProgressIndicator(strokeWidth: 2)));
    } else {
      body = switch (_tab) {
        _Tab.about => _about(c),
        _Tab.code => _code(c),
        _Tab.skills => _skills(c),
      };
    }

    return Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      header,
      const SizedBox(height: 10),
      tabs,
      Expanded(
        child: ListView(
          padding: EdgeInsets.fromLTRB(hPad, 18, hPad, 28),
          children: [
            Align(
              alignment: Alignment.topLeft,
              child: ConstrainedBox(constraints: const BoxConstraints(maxWidth: 820), child: body),
            ),
          ],
        ),
      ),
    ]);
  }

  Widget _about(PtColors c) {
    final ctx = _context!;
    return Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      _Section(
        title: 'Описание для ИИ',
        note: 'уходит в каждую задачу проекта',
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          TextField(
            key: const Key('project-description'),
            controller: _desc,
            onChanged: (_) => setState(() => _descDirty = true),
            minLines: 5,
            maxLines: 14,
            style: ui(14.5, color: c.fg, height: 1.5),
            decoration: const InputDecoration(hintText: 'Что это за проект, для кого, на чём сделан и чего делать нельзя'),
          ),
          const SizedBox(height: 8),
          Row(children: [
            Expanded(
              child: Text('Агенты читают это перед каждой задачей. Пароли и ключи сюда не пишите.',
                  style: ui(12.5, color: c.muted, height: 1.4)),
            ),
            const SizedBox(width: 12),
            FilledButton(
              key: const Key('save-description'),
              onPressed: _busy || !_descDirty
                  ? null
                  : () => _run((api) async {
                        await api.setProjectDescription(widget.projectId, _desc.text);
                        _descDirty = false;
                      }, done: 'Описание сохранено'),
              child: const Text('Сохранить'),
            ),
          ]),
        ]),
      ),
      const SizedBox(height: 16),
      _Section(
        title: 'Файлы проекта',
        note: ctx.files.isEmpty ? 'пока нет' : '${ctx.files.length} · ${_size(ctx.files.fold(0, (a, f) => a + f.size))}',
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          for (final f in ctx.files) ...[
            _FileRow(file: f, onDelete: _busy ? null : () => _run((api) => api.deleteProjectFile(widget.projectId, f.id), done: 'Файл удалён')),
            const SizedBox(height: 6),
          ],
          OutlinedButton.icon(
            key: const Key('add-files'),
            onPressed: _busy ? null : _addFiles,
            icon: const Icon(Icons.upload_file_rounded, size: 18),
            label: const Text('Выбрать файлы'),
          ),
          const SizedBox(height: 6),
          Text('до 20 МБ на файл · агенты видят их только для чтения', style: mono(11.5, color: c.muted)),
        ]),
      ),
    ]);
  }

  Widget _code(PtColors c) {
    final repo = _repo;
    final hosts = _hosts;
    InputDecoration deco(String hint) => InputDecoration(hintText: hint);
    Widget field(String label, Widget input) => Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          Text(label, style: ui(13, color: c.muted)),
          const SizedBox(height: 6),
          input,
        ]);
    Widget pair(Widget a, Widget b) => widget.compact
        ? Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [a, const SizedBox(height: 12), b])
        : Row(crossAxisAlignment: CrossAxisAlignment.start, children: [Expanded(child: a), const SizedBox(width: 16), Expanded(child: b)]);

    return _Section(
      title: 'Код проекта',
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        if (hosts.isEmpty)
          Text('Нет ПК со службой AiCrew. Запустите службу на компьютере, где будет работать код (aicrew-host run).',
              style: ui(14, color: c.fg))
        else ...[
          if (hosts.length > 1) ...[
            field(
              'ПК со службой AiCrew',
              DropdownButtonFormField<String>(
                value: _hostId,
                isExpanded: true,
                items: [for (final h in hosts) DropdownMenuItem(value: h.$1, child: Text(h.$3 ? h.$2 : '${h.$2} · не в сети'))],
                onChanged: (v) => setState(() => _hostId = v),
              ),
            ),
            const SizedBox(height: 12),
          ],
          pair(
            field('Папка на ПК с AiCrew',
                TextField(key: const Key('code-path'), controller: _path, decoration: deco(r'D:\aicrew-work\мой-проект'))),
            field('Git-репозиторий',
                TextField(key: const Key('code-git'), controller: _git, decoration: deco('https://github.com/…/….git'))),
          ),
          const SizedBox(height: 8),
          Text(
            r'Можно указать и то, и другое. Папка есть — служба работает в ней и сверяет её репозиторий с указанным. '
            r'Папки нет — служба скачает репозиторий в неё (пусто — в D:\aicrew-work\<проект>).',
            style: ui(12.5, color: c.muted, height: 1.4),
          ),
          const SizedBox(height: 12),
          pair(
            field('Основная ветка', TextField(key: const Key('code-branch'), controller: _branch, decoration: deco('main'))),
            field(
              'Как проверять код',
              DropdownButtonFormField<String>(
                key: const Key('code-template'),
                value: _template,
                isExpanded: true,
                items: const [
                  DropdownMenuItem(value: 'flutter', child: Text('Flutter')),
                  DropdownMenuItem(value: 'go', child: Text('Go')),
                  DropdownMenuItem(value: 'web', child: Text('Web')),
                  DropdownMenuItem(value: 'roblox', child: Text('Roblox (StyLua, Selene)')),
                  DropdownMenuItem(value: '', child: Text('Свои команды (настраиваются на ПК)')),
                ],
                onChanged: (v) => setState(() => _template = v ?? ''),
              ),
            ),
          ),
          const SizedBox(height: 16),
          if (repo != null) ...[_BindStatus(repo: repo, hosts: hosts), const SizedBox(height: 12)],
          Align(
            alignment: Alignment.centerRight,
            child: FilledButton(
              key: const Key('bind-code'),
              onPressed: _busy || _hostId == null
                  ? null
                  : () => _run(
                        (api) => api.bindRepository(
                          projectId: widget.projectId,
                          hostId: _hostId!,
                          localPath: _path.text.trim(),
                          gitUrl: _git.text.trim(),
                          defaultBranch: _branch.text.trim().isEmpty ? 'main' : _branch.text.trim(),
                          template: _template,
                        ),
                        done: 'Заявка отправлена: служба ПК привяжет код',
                      ),
              child: Text(repo == null ? 'Привязать' : 'Сохранить привязку'),
            ),
          ),
        ],
      ]),
    );
  }

  Widget _skills(PtColors c) {
    final repo = _repo;
    if (repo == null || repo.status != 'ready') {
      return _Section(
        title: 'Скиллы проекта',
        child: Text('Сначала привяжите код проекта на вкладке «Код»: скиллы задаются для его репозитория.',
            style: ui(14, color: c.fg, height: 1.45)),
      );
    }
    Future<void> save(List<String> required, List<String> forbidden) =>
        _run((api) => api.setSkillPolicy(repo.id, required: required, forbidden: forbidden));
    return _Section(
      title: 'Скиллы проекта',
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Wrap(spacing: 6, runSpacing: 6, children: [
          for (final s in repo.requiredSkills)
            InkWell(
              onLongPress: () => save(repo.requiredSkills.where((x) => x != s).toList(), repo.forbiddenSkills),
              child: SkillChip(s, kind: SkillChipKind.locked),
            ),
          for (final s in repo.forbiddenSkills) SkillChip(s, kind: SkillChipKind.forbidden),
          AddSkillChip(
            label: '+ обязательный',
            onTap: () async {
              final picked = await pickSkills(context, ref,
                  selected: repo.requiredSkills, hidden: repo.forbiddenSkills, title: 'Обязательные скиллы проекта');
              if (picked != null) await save(picked, repo.forbiddenSkills);
            },
          ),
          AddSkillChip(
            label: '+ запрещённый',
            onTap: () async {
              final picked = await pickSkills(context, ref,
                  selected: repo.forbiddenSkills, hidden: repo.requiredSkills, title: 'Запрещённые скиллы проекта');
              if (picked != null) await save(repo.requiredSkills, picked);
            },
          ),
        ]),
        const SizedBox(height: 10),
        Text(
          'В рамке — обязательные для каждой задачи проекта, зачёркнутые — запрещённые. '
          'Остальные подбирает Manager или вы в самой задаче.',
          style: ui(12.5, color: c.muted, height: 1.4),
        ),
      ]),
    );
  }

  static String _size(int b) => b >= 1 << 20
      ? '${(b / (1 << 20)).toStringAsFixed(1).replaceAll('.', ',')} МБ'
      : b >= 1 << 10
          ? '${b >> 10} КБ'
          : '$b Б';
}

class _Section extends StatelessWidget {
  const _Section({required this.title, required this.child, this.note});
  final String title;
  final String? note;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: c.bg,
        border: Border.all(color: c.line),
        borderRadius: BorderRadius.circular(PtRadius.lg),
      ),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Row(crossAxisAlignment: CrossAxisAlignment.baseline, textBaseline: TextBaseline.alphabetic, children: [
          Expanded(child: Text(title, style: ui(15, weight: FontWeight.w600, color: c.fg))),
          if (note != null) Text(note!, style: mono(11.5, color: c.muted)),
        ]),
        const SizedBox(height: 12),
        child,
      ]),
    );
  }
}

class _FileRow extends StatelessWidget {
  const _FileRow({required this.file, this.onDelete});
  final ProjectFileInfo file;
  final VoidCallback? onDelete;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    final ext = file.name.contains('.') ? file.name.split('.').last.toUpperCase() : '';
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(border: Border.all(color: c.line), borderRadius: BorderRadius.circular(PtRadius.md)),
      child: Row(children: [
        Container(
          width: 26,
          height: 32,
          alignment: Alignment.bottomCenter,
          padding: const EdgeInsets.only(bottom: 3),
          decoration: BoxDecoration(
            border: Border.all(color: c.fg, width: 1.5),
            borderRadius: const BorderRadius.only(
                topLeft: Radius.circular(3), topRight: Radius.circular(8), bottomLeft: Radius.circular(3), bottomRight: Radius.circular(3)),
          ),
          child: Text(ext.length > 4 ? ext.substring(0, 4) : ext, style: mono(8, weight: FontWeight.w500, color: c.fg)),
        ),
        const SizedBox(width: 10),
        Expanded(child: Text(file.name, overflow: TextOverflow.ellipsis, style: ui(14, color: c.fg))),
        Text(_ProjectWindowState._size(file.size), style: mono(12, color: c.muted)),
        if (onDelete != null)
          IconButton(
            tooltip: 'Удалить файл',
            onPressed: onDelete,
            icon: Icon(Icons.delete_outline_rounded, size: 18, color: c.muted),
          ),
      ]),
    );
  }
}

/// Плашка состояния привязки: квадрат — привязан, пустой квадрат — ждёт службу, перечёркнутый — ошибка.
class _BindStatus extends StatelessWidget {
  const _BindStatus({required this.repo, required this.hosts});
  final ExecRepository repo;
  final List<(String, String, bool)> hosts;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    final host = hosts.where((h) => h.$1 == repo.hostId).firstOrNull?.$2 ?? 'ПК';
    final (marker, title, detail) = switch (repo.status) {
      'ready' => (MarkerType.project, 'Привязан', 'к ПК $host · ${repo.defaultBranch} → aicrew/integration'),
      'failed' => (MarkerType.project, 'Не привязан', repo.statusError),
      _ => (MarkerType.project, 'Ждёт службу ПК', '$host выполнит заявку в течение минуты'),
    };
    return Container(
      key: const Key('bind-status'),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(color: c.panel, border: Border.all(color: c.line), borderRadius: BorderRadius.circular(PtRadius.md)),
      child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Padding(
          padding: const EdgeInsets.only(top: 4),
          child: repo.status == 'ready'
              ? Marker(marker, size: 8)
              : Container(
                  width: 8,
                  height: 8,
                  decoration: BoxDecoration(border: Border.all(color: c.fg, width: 1.5), borderRadius: BorderRadius.circular(2)),
                ),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Text.rich(TextSpan(children: [
            TextSpan(text: '$title  ', style: ui(14, weight: FontWeight.w600, color: c.fg)),
            TextSpan(text: detail, style: ui(13, color: c.muted, height: 1.4)),
          ])),
        ),
        if (repo.status == 'ready') Text('в main переносите вы', style: ui(12, color: c.muted)),
      ]),
    );
  }
}
