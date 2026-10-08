/// Дробные ключи порядка (как LexoRank): вставка между соседями не трогает
/// остальных, поэтому перестановки с двух устройств не конфликтуют.
///
/// Ключ — строка из [digits], без завершающего '0'. Сравнение — побайтовое,
/// как в PostgreSQL с collation "C" и в Dart [String.compareTo].
library;

const digits = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz';

/// Ключ строго между [a] и [b]. `null` означает «нет соседа» с этой стороны.
String keyBetween(String? a, String? b) {
  final lo = a ?? '';
  if (b != null && lo.compareTo(b) >= 0) {
    throw ArgumentError('keyBetween: $a must be < $b');
  }
  return _midpoint(lo, b);
}

/// Ключ после последнего элемента списка.
String keyAfter(String? last) => keyBetween(last, null);

String _midpoint(String a, String? b) {
  if (b != null) {
    // Общий префикс (недостающие цифры у a считаем нулями).
    var n = 0;
    while (n < b.length && (n < a.length ? a[n] : '0') == b[n]) {
      n++;
    }
    if (n > 0) {
      return b.substring(0, n) + _midpoint(a.length > n ? a.substring(n) : '', b.substring(n));
    }
  }
  final da = a.isEmpty ? 0 : digits.indexOf(a[0]);
  final db = b == null ? digits.length : digits.indexOf(b[0]);
  if (db - da > 1) {
    return digits[((da + db) / 2).round()];
  }
  if (b != null && b.length > 1) {
    return b[0];
  }
  return digits[da] + _midpoint(a.length > 1 ? a.substring(1) : '', null);
}
