import 'dart:math';

import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

void main() {
  group('sort keys', () {
    test('keyAfter grows strictly', () {
      String? last;
      for (var i = 0; i < 500; i++) {
        final k = keyAfter(last);
        if (last != null) expect(k.compareTo(last) > 0, isTrue, reason: '$k > $last');
        last = k;
      }
    });

    test('random inserts keep order and stay valid', () {
      final rnd = Random(42);
      final keys = <String>[keyAfter(null)];
      for (var i = 0; i < 2000; i++) {
        final pos = rnd.nextInt(keys.length + 1);
        final a = pos == 0 ? null : keys[pos - 1];
        final b = pos == keys.length ? null : keys[pos];
        final k = keyBetween(a, b);
        if (a != null) expect(a.compareTo(k) < 0, isTrue, reason: '$a < $k');
        if (b != null) expect(k.compareTo(b) < 0, isTrue, reason: '$k < $b');
        expect(k.endsWith('0'), isFalse);
        expect(RegExp(r'^[0-9A-Za-z]{1,128}$').hasMatch(k), isTrue, reason: k);
        keys.insert(pos, k);
      }
    });

    test('rejects reversed bounds', () {
      expect(() => keyBetween('b', 'a'), throwsArgumentError);
    });
  });
}
