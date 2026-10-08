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
  String? _error;

  @override
  void initState() {
    super.initState();
    final url = ref.read(sessionProvider).serverUrl;
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
    final client = ApiClient(baseUrl: url, tokens: ref.read(tokenStoreProvider));
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
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 400),
              child: AutofillGroup(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    const Align(alignment: Alignment.centerLeft, child: AppIcon(size: 40)),
                    const SizedBox(height: 20),
                    Text('PromptTree', style: ui(28, weight: FontWeight.w700, color: c.fg, letterSpacing: -0.5)),
                    const SizedBox(height: 6),
                    Text('Идеи на ходу — задачи для Claude.', style: ui(15, color: c.muted)),
                    const SizedBox(height: 32),
                    TextField(
                      key: const Key('server'),
                      controller: _server,
                      keyboardType: TextInputType.url,
                      autocorrect: false,
                      decoration: const InputDecoration(labelText: 'Сервер', hintText: 'prompttree.example.com'),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      key: const Key('email'),
                      controller: _email,
                      keyboardType: TextInputType.emailAddress,
                      autocorrect: false,
                      autofillHints: const [AutofillHints.email],
                      decoration: const InputDecoration(labelText: 'Email'),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      key: const Key('password'),
                      controller: _password,
                      obscureText: true,
                      autofillHints: const [AutofillHints.password],
                      decoration: const InputDecoration(labelText: 'Пароль'),
                      onSubmitted: (_) => _busy ? null : _submit(register: false),
                    ),
                    if (_error != null) ...[
                      const SizedBox(height: 12),
                      Text(_error!, style: ui(14, color: c.fg, weight: FontWeight.w500)),
                    ],
                    const SizedBox(height: 20),
                    FilledButton(
                      onPressed: _busy ? null : () => _submit(register: false),
                      child: _busy
                          ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                          : const Text('Войти'),
                    ),
                    const SizedBox(height: 4),
                    TextButton(
                      onPressed: _busy ? null : () => _submit(register: true),
                      child: const Text('Первый запуск: создать владельца'),
                    ),
                    const SizedBox(height: 24),
                    Divider(color: c.line),
                    const SizedBox(height: 24),
                    OutlinedButton(onPressed: _busy ? null : _workLocally, child: const Text('Работать без сервера')),
                    const SizedBox(height: 8),
                    Text(
                      'Всё хранится на этом устройстве. Войти можно позже: накопленное уйдёт на сервер.',
                      style: ui(13, color: c.muted),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
