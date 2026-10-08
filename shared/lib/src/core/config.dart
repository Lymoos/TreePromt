/// Адрес нашего сервера PromptTree, вшитый в сборку:
///   flutter build apk --dart-define=PT_SERVER_URL=https://prompttree.example.com
/// Пусто — сборка для разработки: адрес вводится на экране входа.
const builtInServerUrl = String.fromEnvironment('PT_SERVER_URL');

Uri? get builtInServer => builtInServerUrl.isEmpty ? null : Uri.parse(builtInServerUrl);
