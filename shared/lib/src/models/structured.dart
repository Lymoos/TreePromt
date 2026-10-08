import 'dart:convert';

/// Результат структурирования (node_content.structured_content), docs/stage6-ai-structuring.md.
class StructuredDoc {
  const StructuredDoc({
    required this.role,
    required this.formattedText,
    this.facts = const [],
    this.constraints = const [],
    this.openQuestions = const [],
    this.humanParagraphs = const [],
  });

  final String role;
  final String formattedText;
  final List<String> facts;
  final List<String> constraints;
  final List<String> openQuestions;

  /// Абзацы, написанные человеком: повторное структурирование их не перезапишет.
  final List<String> humanParagraphs;

  static List<String> _list(Object? v) => (v as List?)?.cast<String>() ?? const [];

  static StructuredDoc? tryParse(String? json) {
    if (json == null || json.isEmpty || json == 'null') return null;
    try {
      final j = jsonDecode(json) as Map<String, dynamic>;
      return StructuredDoc(
        role: j['role'] as String? ?? '',
        formattedText: j['formatted_text'] as String? ?? '',
        facts: _list(j['facts']),
        constraints: _list(j['constraints']),
        openQuestions: _list(j['open_questions']),
        humanParagraphs: _list(j['human_paragraphs']),
      );
    } catch (_) {
      return null;
    }
  }

  /// Итоговая задача для Claude: роль в начале, затем текст (ТЗ п. 6.4, 6.5).
  String get finalTask => role.trim().isEmpty ? formattedText : '${role.trim()}\n\n$formattedText';
}

class Finding {
  const Finding(this.code, this.detail);
  final String code;
  final String detail;
}

/// Результат ИИ, который сервер не применил сам.
class StructureProposal {
  const StructureProposal({required this.requestId, required this.reason, this.findings = const [], this.error, this.generated});

  final String requestId;

  /// stale — исходник изменился, пока ИИ работал; flagged — замечания валидатора; failed — ошибка.
  final String reason;
  final List<Finding> findings;
  final String? error;
  final StructuredDoc? generated;

  static const stale = 'stale';
  static const flagged = 'flagged';
  static const failed = 'failed';

  static StructureProposal? tryParse(String? json) {
    if (json == null || json.isEmpty || json == 'null') return null;
    try {
      final j = jsonDecode(json) as Map<String, dynamic>;
      final g = j['generated'];
      return StructureProposal(
        requestId: j['request_id'] as String,
        reason: j['reason'] as String,
        error: j['error'] as String?,
        findings: [
          for (final f in (j['findings'] as List? ?? const []))
            Finding((f as Map)['code'] as String? ?? '', f['detail'] as String? ?? ''),
        ],
        generated: g == null ? null : StructuredDoc.tryParse(jsonEncode(g)),
      );
    } catch (_) {
      return null;
    }
  }
}
