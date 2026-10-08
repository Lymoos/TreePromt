import 'dart:convert';

import 'package:prompttree_shared/prompttree_shared.dart';
import 'package:web/web.dart' as web;

/// Токены в браузере (архитектура п. 5):
/// - refresh-токен — только в httpOnly-cookie, скрипты страницы его не видят;
/// - access-токен — только в памяти вкладки, после перезагрузки берётся новый через cookie;
/// - в localStorage — лишь id пользователя и устройства, чтобы помнить, что вход был.
class WebTokenStore implements TokenStore {
  static const _key = 'prompttree.session';
  AuthTokens? _current;

  @override
  Future<AuthTokens?> read() async {
    if (_current != null) return _current;
    final raw = web.window.localStorage.getItem(_key);
    if (raw == null) return null;
    try {
      final j = jsonDecode(raw) as Map<String, dynamic>;
      return _current = AuthTokens(
        accessToken: '', // первый запрос получит 401 и обновит токен через cookie
        refreshToken: 'cookie',
        userId: j['user_id'] as String,
        deviceId: j['device_id'] as String,
      );
    } catch (_) {
      return null;
    }
  }

  @override
  Future<void> write(AuthTokens tokens) async {
    _current = tokens;
    web.window.localStorage.setItem(_key, jsonEncode({'user_id': tokens.userId, 'device_id': tokens.deviceId}));
  }

  @override
  Future<void> clear() async {
    _current = null;
    web.window.localStorage.removeItem(_key);
  }
}
