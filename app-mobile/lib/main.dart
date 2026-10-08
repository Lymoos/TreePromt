import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'src/app.dart';
import 'src/secure_token_store.dart';
import 'src/state/providers.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final store = openLocalStore();
  final tokens = SecureTokenStore();
  final session = await loadSession(store, tokens);
  runApp(ProviderScope(
    overrides: [
      storeProvider.overrideWithValue(store),
      tokenStoreProvider.overrideWithValue(tokens),
      sessionProvider.overrideWith((ref) => session),
    ],
    child: const PromptTreeApp(),
  ));
}
