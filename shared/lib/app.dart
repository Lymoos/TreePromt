/// Общий слой приложения (Riverpod): состояние, экраны и действия,
/// одинаковые для телефона, ПК и веба. Оболочки — в app-mobile, app-desktop, web.
library;

export 'src/app/providers.dart';
export 'src/app/screens/conflict_screen.dart';
export 'src/app/screens/editor_screen.dart';
export 'src/app/screens/login_screen.dart';
export 'src/app/screens/trash_screen.dart';
export 'src/app/screens/versions_screen.dart';
export 'src/app/secure_token_store.dart';
export 'src/app/session.dart';
export 'src/app/tree_actions.dart';
export 'src/app/widgets/common.dart';
export 'src/app/widgets/dialogs.dart';
export 'src/app/widgets/sync_badge.dart';
