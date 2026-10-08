import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

/// Токены — в Keystore (Android) / Keychain (iOS), не в базе приложения.
class SecureTokenStore implements TokenStore {
  SecureTokenStore([FlutterSecureStorage? storage])
      : _s = storage ?? const FlutterSecureStorage(aOptions: AndroidOptions(encryptedSharedPreferences: true));

  final FlutterSecureStorage _s;
  static const _key = 'prompttree.tokens';

  @override
  Future<AuthTokens?> read() async {
    final v = await _s.read(key: _key);
    if (v == null) return null;
    try {
      return AuthTokens.fromJson(jsonDecode(v) as Map<String, dynamic>);
    } catch (_) {
      return null;
    }
  }

  @override
  Future<void> write(AuthTokens tokens) => _s.write(key: _key, value: jsonEncode(tokens.toJson()));

  @override
  Future<void> clear() => _s.delete(key: _key);
}
