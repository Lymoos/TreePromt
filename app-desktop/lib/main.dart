import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/app.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'src/app.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final store = openLocalStore();
  final tokens = SecureTokenStore();
  final session = await loadSession(store, tokens);
  runApp(ProviderScope(
    overrides: [
      storeProvider.overrideWithValue(store),
      tokenStoreProvider.overrideWithValue(tokens),
      deviceInfoProvider.overrideWithValue((name: Platform.localHostname, platform: Platform.operatingSystem)),
      sessionProvider.overrideWith((ref) => session),
    ],
    child: const PromptTreeDesktopApp(),
  ));
}
