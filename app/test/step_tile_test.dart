import 'package:click_spirit/model/script.dart';
import 'package:click_spirit/ui/step_list.dart';
import 'package:flutter/material.dart' hide Step;
import 'package:flutter_test/flutter_test.dart';

Widget _list(List<StepRow> rows) {
  return MaterialApp(
    home: Scaffold(
      body: StepList(
        rows: rows,
        selected: null,
        activeIndex: 0,
        activeRepetition: 0,
        onSelect: (_) {},
        onEdit: (_) {},
        onDuplicate: (_) {},
        onDelete: (_) {},
        onReorder: (a, b) {},
      ),
    ),
  );
}

void main() {
  testWidgets('step tiles are twice as tall as before (70 -> 140)', (tester) async {
    await tester.binding.setSurfaceSize(const Size(900, 700));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.pumpWidget(_list([
      StepRow(Step(x: 1420, y: 160, button: 'left', clicks: 1, intervalMs: 0, holdMs: 0, repeat: 3)),
      StepRow(Step(
        x: 0,
        y: 0,
        button: 'left',
        clicks: 1,
        intervalMs: 0,
        holdMs: 0,
        target: Target(kind: 'circle', cx: 1450, cy: 520, r: 70),
      )),
    ]));
    expect(kStepTileHeight, 2 * kLegacyStepTileHeight);
    for (final i in [0, 1]) {
      final size = tester.getSize(find.byKey(ValueKey('step-tile-$i')));
      expect(size.height, kStepTileHeight);
      expect(size.height, 140);
    }
    // Content stays readable: title, summary and badge are inside the tile.
    final tile = tester.getRect(find.byKey(const ValueKey('step-tile-0')));
    for (final key in ['step-title-0', 'step-summary-0']) {
      final r = tester.getRect(find.byKey(ValueKey(key)));
      expect(tile.contains(r.topLeft) && tile.contains(r.bottomRight - const Offset(1, 1)), isTrue, reason: key);
    }
    expect(tester.takeException(), isNull);
  });
}
