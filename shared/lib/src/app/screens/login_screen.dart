import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';
import '../widgets/dialogs.dart';

const deviceIdKey = 'device.id';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _server = TextEditingController();
  final _email = TextEditingController();
  final _password = TextEditingController();
  bool _busy = false;
  bool _showPassword = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    final url = ref.read(sessionProvider).serverUrl ?? ref.read(defaultServerUrlProvider);
    if (url != null) _server.text = url.toString();
  }

  @override
  void dispose() {
    _server.dispose();
    _email.dispose();
    _password.dispose();
    super.dispose();
  }

  Uri? _serverUri() {
    var s = _server.text.trim();
    if (s.isEmpty) return null;
    if (!s.contains('://')) s = 'https://$s';
    final u = Uri.tryParse(s);
    return (u == null || u.host.isEmpty) ? null : u;
  }

  String _message(Object e) {
    if (e is NetworkException) return 'Сервер недоступен. Проверьте адрес и сеть.';
    if (e is ApiException) {
      return switch (e.code) {
        'invalid_credentials' => 'Неверный email или пароль.',
        'registration_closed' => 'Владелец уже создан. Войдите со своим email и паролем.',
        'email_taken' => 'Такой email уже зарегистрирован.',
        'invalid_input' => 'Проверьте email. Пароль — от 10 символов.',
        'rate_limited' => 'Слишком много попыток. Подождите минуту.',
        _ => 'Сервер ответил ошибкой: ${e.message}',
      };
    }
    return 'Не получилось: $e';
  }

  Future<void> _submit({required bool register}) async {
    final url = _serverUri();
    if (url == null) {
      setState(() => _error = 'Укажите адрес сервера, например prompttree.example.com');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    final store = ref.read(storeProvider);
    final client = ApiClient(baseUrl: url, tokens: ref.read(tokenStoreProvider), headers: ref.read(apiHeadersProvider));
    try {
      if (register) await client.register(_email.text.trim(), _password.text);
      var deviceId = await store.getValue(deviceIdKey) ?? newId();
      AuthTokens tokens;
      try {
        tokens = await _login(client, deviceId);
      } on ApiException catch (e) {
        if (e.code != 'device_conflict') rethrow;
        deviceId = newId(); // id устройства уже занят другим аккаунтом
        tokens = await _login(client, deviceId);
      }
      await store.setValue(deviceIdKey, deviceId);

      final previousUser = await store.getValue(sessionUserKey);
      if (previousUser != null && previousUser != tokens.userId) {
        if (!mounted) return;
        final wipe = await confirm(
          context,
          title: 'Данные другого аккаунта',
          message: 'На телефоне хранятся заметки другого аккаунта. Чтобы войти, их нужно удалить с устройства. '
              'На сервере они останутся.',
          action: 'Удалить и войти',
        );
        if (!wipe) {
          await client.logout();
          return;
        }
        await store.wipe();
        await store.setValue(deviceIdKey, deviceId);
      }
      await store.setValue(sessionUserKey, tokens.userId);
      await store.setValue(serverUrlKey, url.toString());
      await store.setValue(sessionModeKey, SessionMode.server.name);
      ref.read(sessionProvider.notifier).state = Session(mode: SessionMode.server, serverUrl: url);
    } catch (e) {
      setState(() => _error = _message(e));
    } finally {
      client.close();
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<AuthTokens> _login(ApiClient client, String deviceId) {
    final device = ref.read(deviceInfoProvider);
    return client.login(
      _email.text.trim(),
      _password.text,
      deviceId: deviceId,
      deviceName: device.name,
      platform: device.platform,
    );
  }

  Future<void> _workLocally() async {
    await ref.read(storeProvider).setValue(sessionModeKey, SessionMode.local.name);
    ref.read(sessionProvider.notifier).state =
        Session(mode: SessionMode.local, serverUrl: ref.read(sessionProvider).serverUrl);
  }

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    // Широкое окно (ПК, планшет, веб): карточка в две колонки — бренд слева, форма справа.
    final wide = MediaQuery.sizeOf(context).width >= 760;
    var i = 0;
    Widget step(Widget child) => FadeSlideIn(delay: FadeSlideIn.stagger(i++, step: 55, max: 12), child: child);

    final brand = <Widget>[
      const Align(alignment: Alignment.centerLeft, child: AnimatedAppIcon(size: 48)),
      const SizedBox(height: 22),
      step(Text('PromptTree', style: ui(30, weight: FontWeight.w700, color: c.fg, letterSpacing: -0.8))),
      const SizedBox(height: 6),
      step(Text('Идеи на ходу — задачи для Claude.', style: ui(15.5, color: c.muted, height: 1.45))),
    ];

    final fields = <Widget>[
      step(TextField(
        key: const Key('server'),
        controller: _server,
        keyboardType: TextInputType.url,
        autocorrect: false,
        textInputAction: TextInputAction.next,
        decoration: const InputDecoration(
          labelText: 'Сервер',
          hintText: 'prompttree.example.com',
          prefixIcon: Icon(Icons.dns_outlined, size: 19),
        ),
      )),
      const SizedBox(height: 12),
      step(TextField(
        key: const Key('email'),
        controller: _email,
        keyboardType: TextInputType.emailAddress,
        autocorrect: false,
        textInputAction: TextInputAction.next,
        autofillHints: const [AutofillHints.email],
        decoration: const InputDecoration(
          labelText: 'Email',
          prefixIcon: Icon(Icons.alternate_email_rounded, size: 19),
        ),
      )),
      const SizedBox(height: 12),
      step(TextField(
        key: const Key('password'),
        controller: _password,
        obscureText: !_showPassword,
        autofillHints: const [AutofillHints.password],
        decoration: InputDecoration(
          labelText: 'Пароль',
          prefixIcon: const Icon(Icons.lock_outline_rounded, size: 19),
          suffixIcon: IconButton(
            tooltip: _showPassword ? 'Скрыть пароль' : 'Показать пароль',
            onPressed: () => setState(() => _showPassword = !_showPassword),
            icon: AnimatedSwitcher(
              duration: PtMotion.fast,
              transitionBuilder: (child, a) => ScaleTransition(scale: a, child: child),
              child: Icon(
                _showPassword ? Icons.visibility_off_outlined : Icons.visibility_outlined,
                key: ValueKey(_showPassword),
                size: 19,
                color: c.muted,
              ),
            ),
          ),
        ),
        onSubmitted: (_) => _busy ? null : _submit(register: false),
      )),
      AnimatedSize(
        duration: PtMotion.normal,
        curve: PtMotion.curve,
        alignment: Alignment.topCenter,
        child: AnimatedSwitcher(
          duration: PtMotion.normal,
          transitionBuilder: (child, a) => FadeTransition(opacity: a, child: child),
          child: _error == null
              ? const SizedBox(width: double.infinity, key: ValueKey('no-error'))
              : Padding(
                  key: ValueKey(_error),
                  padding: const EdgeInsets.only(top: 14),
                  child: Container(
                    padding: const EdgeInsets.fromLTRB(12, 10, 12, 10),
                    decoration: BoxDecoration(
                      color: c.panel,
                      borderRadius: BorderRadius.circular(PtRadius.md),
                      border: Border.all(color: c.fg),
                    ),
                    child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      Padding(
                        padding: const EdgeInsets.only(top: 1),
                        child: Icon(Icons.error_outline_rounded, size: 18, color: c.fg),
                      ),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Text(_error!, style: ui(13.5, color: c.fg, weight: FontWeight.w500, height: 1.4)),
                      ),
                    ]),
                  ),
                ),
        ),
      ),
      const SizedBox(height: 18),
      step(SizedBox(
        height: 50,
        child: FilledButton(
          onPressed: _busy ? null : () => _submit(register: false),
          child: AnimatedSwitcher(
            duration: PtMotion.fast,
            child: _busy
                ? SizedBox(
                    key: const ValueKey('busy'),
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2, color: c.faint),
                  )
                : const Text('Войти', key: ValueKey('label')),
          ),
        ),
      )),
      const SizedBox(height: 4),
      step(TextButton(
        onPressed: _busy ? null : () => _submit(register: true),
        child: const Text('Первый запуск: создать владельца'),
      )),
    ];

    final local = <Widget>[
      step(SizedBox(
        height: 48,
        child: OutlinedButton.icon(
          onPressed: _busy ? null : _workLocally,
          icon: const Icon(Icons.devices_rounded, size: 18),
          label: const Text('Работать без сервера'),
        ),
      )),
      const SizedBox(height: 10),
      step(Text(
        'Всё хранится на этом устройстве. Войти можно позже: накопленное уйдёт на сервер.',
        style: ui(13, color: c.muted, height: 1.45),
        textAlign: wide ? TextAlign.start : TextAlign.center,
      )),
    ];

    final Widget body;
    if (wide) {
      body = FadeSlideIn(
        scale: 0.98,
        offset: const Offset(0, 16),
        child: Container(
          decoration: BoxDecoration(
            color: c.card,
            borderRadius: BorderRadius.circular(PtRadius.xl + 4),
            border: Border.all(color: c.line),
            boxShadow: c.elevation(2),
          ),
          clipBehavior: Clip.antiAlias,
          child: IntrinsicHeight(
            child: Row(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
              Expanded(
                child: Container(
                  color: c.panel,
                  padding: const EdgeInsets.fromLTRB(32, 32, 32, 28),
                  child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                    ...brand,
                    const SizedBox(height: 28),
                    const Spacer(),
                    ...local,
                  ]),
                ),
              ),
              VerticalDivider(width: 1, color: c.line),
              Expanded(
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(32, 32, 32, 24),
                  child: AutofillGroup(
                    child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                      step(Text('Вход на сервер', style: ui(18, weight: FontWeight.w700, color: c.fg))),
                      const SizedBox(height: 4),
                      step(Text('Заметки синхронизируются между устройствами.', style: ui(13.5, color: c.muted))),
                      const SizedBox(height: 22),
                      ...fields,
                    ]),
                  ),
                ),
              ),
            ]),
          ),
        ),
      );
    } else {
      body = AutofillGroup(
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, mainAxisSize: MainAxisSize.min, children: [
          ...brand,
          const SizedBox(height: 30),
          ...fields,
          const SizedBox(height: 16),
          step(Row(children: [
            Expanded(child: Divider(color: c.line)),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              child: Text('или', style: mono(11, color: c.faint, letterSpacing: 0.6)),
            ),
            Expanded(child: Divider(color: c.line)),
          ])),
          const SizedBox(height: 18),
          ...local,
        ]),
      );
    }

    return Scaffold(
      body: Stack(children: [
        const Positioned.fill(child: _DotGrid()),
        SafeArea(
          child: Center(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(24),
              child: ConstrainedBox(constraints: BoxConstraints(maxWidth: wide ? 820 : 400), child: body),
            ),
          ),
        ),
      ]),
    );
  }
}

/// Фон экрана входа: сетка точек, которая растворяется к краям.
class _DotGrid extends StatelessWidget {
  const _DotGrid();

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return IgnorePointer(
      child: ShaderMask(
        blendMode: BlendMode.dstIn,
        shaderCallback: (rect) => RadialGradient(
          center: const Alignment(0, -0.35),
          radius: 0.9,
          colors: [Colors.white, Colors.white.withValues(alpha: 0)],
        ).createShader(rect),
        child: CustomPaint(painter: _DotsPainter(c.lineStrong)),
      ),
    );
  }
}

class _DotsPainter extends CustomPainter {
  _DotsPainter(this.color);
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    const step = 22.0;
    final p = Paint()..color = color;
    for (var y = step / 2; y < size.height; y += step) {
      for (var x = step / 2; x < size.width; x += step) {
        canvas.drawCircle(Offset(x, y), 1.1, p);
      }
    }
  }

  @override
  bool shouldRepaint(_DotsPainter oldDelegate) => oldDelegate.color != color;
}
