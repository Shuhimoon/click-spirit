import 'package:flutter/material.dart' hide Step;

import '../model/script.dart';

class StepRow {
  StepRow(this.step) : id = _nextId++;

  static int _nextId = 1;
  final int id;
  Step step;
}

/// Index of [index] after removing [oldIndex] and inserting at [newIndex].
///
/// [newIndex] is the index used by [ReorderableListView.onReorderItem], which
/// is already adjusted for the removed item.
int shiftAfterReorder(int index, int oldIndex, int newIndex) {
  if (index == oldIndex) return newIndex;
  var next = index;
  if (next > oldIndex) next -= 1;
  if (next >= newIndex) next += 1;
  return next;
}

String buttonLabel(String button) {
  switch (button) {
    case 'left':
      return '左鍵';
    case 'right':
      return '右鍵';
    case 'middle':
      return '中鍵';
    default:
      return button.isEmpty ? '未設定按鍵' : button;
  }
}

String templateFileName(Target target) {
  final path = target.template.trim();
  if (path.isNotEmpty) {
    final slash = path.lastIndexOf('/');
    final back = path.lastIndexOf('\\');
    final cut = slash > back ? slash : back;
    return cut >= 0 ? path.substring(cut + 1) : path;
  }
  if (target.templateB64.isNotEmpty) return '內嵌圖片';
  return '未設定模板';
}

IconData stepKindIcon(Step step) {
  switch (step.kindOrPoint) {
    case 'circle':
      return Icons.circle_outlined;
    case 'image':
      return Icons.image_outlined;
    default:
      return Icons.place;
  }
}

String stepWhere(Step step) {
  final target = step.target;
  final kind = step.kindOrPoint;
  final String place;
  if (kind == 'circle' && target != null) {
    place = '圓 (${target.cx}, ${target.cy}) r=${target.r}';
  } else if (kind == 'image' && target != null) {
    place = '模板 ${templateFileName(target)}';
  } else {
    place = '座標 (${step.x}, ${step.y})';
  }
  return '$place  ${buttonLabel(step.button)}';
}

String stepDetails(Step step) {
  final clicks = step.clicks == 0 ? '1（預設）' : '${step.clicks}';
  return '點擊 $clicks  間隔 ${step.intervalMs} ms  重複 ${step.effectiveRepeat}  按住 ${step.holdMs} ms';
}

class StepList extends StatelessWidget {
  const StepList({
    super.key,
    required this.rows,
    required this.selected,
    required this.activeIndex,
    required this.activeRepetition,
    required this.onSelect,
    required this.onEdit,
    required this.onDelete,
    required this.onDuplicate,
    required this.onReorder,
  });

  final List<StepRow> rows;
  final int? selected;
  final int? activeIndex;
  final int activeRepetition;
  final ValueChanged<int> onSelect;
  final ValueChanged<int> onEdit;
  final ValueChanged<int> onDelete;
  final ValueChanged<int> onDuplicate;
  final void Function(int oldIndex, int newIndex) onReorder;

  @override
  Widget build(BuildContext context) {
    if (rows.isEmpty) {
      return const Center(child: Text('尚未加入步驟'));
    }
    final scheme = Theme.of(context).colorScheme;
    return ReorderableListView.builder(
      padding: const EdgeInsets.symmetric(vertical: 8),
      itemCount: rows.length,
      onReorderItem: onReorder,
      buildDefaultDragHandles: false,
      itemBuilder: (context, index) {
        final row = rows[index];
        final step = row.step;
        final active = activeIndex == index;
        final picked = selected == index;
        final Color? color;
        if (active) {
          color = scheme.primaryContainer;
        } else if (picked) {
          color = scheme.secondaryContainer;
        } else {
          color = null;
        }
        return Card(
          key: ValueKey(row.id),
          margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
          color: color,
          child: InkWell(
            onTap: () => onSelect(index),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
              child: Row(
                children: [
                  Icon(stepKindIcon(step)),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('步驟 ${index + 1}', key: ValueKey('step-title-$index')),
                        Text(stepWhere(step), key: ValueKey('step-summary-$index')),
                        Text(stepDetails(step)),
                        if (active)
                          Text(
                            '第 ${activeRepetition + 1}/${step.effectiveRepeat} 次',
                            key: const ValueKey('running-repetition'),
                          ),
                      ],
                    ),
                  ),
                  _action('編輯', Icons.edit, () => onEdit(index)),
                  _action('複製', Icons.copy, () => onDuplicate(index)),
                  _action('刪除', Icons.delete, () => onDelete(index)),
                  ReorderableDragStartListener(
                    index: index,
                    child: Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 4),
                      child: Icon(Icons.drag_handle, key: ValueKey('drag-$index')),
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _action(String tooltip, IconData icon, VoidCallback onPressed) {
    return IconButton(
      tooltip: tooltip,
      onPressed: onPressed,
      visualDensity: VisualDensity.compact,
      icon: Icon(icon, size: 20),
    );
  }
}
