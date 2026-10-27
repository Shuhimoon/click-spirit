import 'package:flutter/material.dart' hide Step;

import '../model/script.dart';
import 'glass.dart';
import 'theme.dart';

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
      return Icons.place_outlined;
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
      final secondary = ClickSpiritColors.secondary(context);
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.touch_app_outlined, size: 28, color: secondary),
            const SizedBox(height: 8),
            Text('尚未加入步驟', style: TextStyle(fontSize: 14, color: secondary, height: 1.3)),
          ],
        ),
      );
    }
    return ReorderableListView.builder(
      padding: const EdgeInsets.fromLTRB(10, 4, 10, 12),
      itemCount: rows.length,
      onReorderItem: onReorder,
      buildDefaultDragHandles: false,
      proxyDecorator: (child, index, animation) {
        return AnimatedBuilder(
          animation: animation,
          builder: (context, child) {
            final t = Curves.easeOutCubic.transform(animation.value);
            return Transform.scale(scale: 1 + (0.015 * t), child: child);
          },
          child: child,
        );
      },
      itemBuilder: (context, index) {
        final row = rows[index];
        final step = row.step;
        final active = activeIndex == index;
        final picked = selected == index;
        final Color? tint;
        if (active) {
          tint = ClickSpiritColors.systemBlue.withValues(alpha: 0.16);
        } else if (picked) {
          tint = ClickSpiritColors.systemBlue.withValues(alpha: 0.08);
        } else {
          tint = null;
        }
        return Padding(
          key: ValueKey(row.id),
          padding: const EdgeInsets.symmetric(vertical: 4),
          child: GlassPanel(
            radius: 16,
            sigma: 12,
            shadow: false,
            tint: tint,
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 8),
            child: InkWell(
              onTap: () => onSelect(index),
              borderRadius: BorderRadius.circular(12),
              child: Row(
                children: [
                  _badge(index + 1, active),
                  const SizedBox(width: 10),
                  Icon(stepKindIcon(step), size: 18, color: ClickSpiritColors.secondary(context)),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          '步驟 ${index + 1}',
                          key: ValueKey('step-title-$index'),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            fontSize: 15,
                            fontWeight: FontWeight.w600,
                            letterSpacing: -0.3,
                            height: 1.25,
                            color: ClickSpiritColors.label(context),
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          stepWhere(step),
                          key: ValueKey('step-summary-$index'),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            fontSize: 13,
                            height: 1.3,
                            color: ClickSpiritColors.secondary(context),
                          ),
                        ),
                        Text(
                          stepDetails(step),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            fontSize: 12,
                            height: 1.3,
                            color: ClickSpiritColors.systemGray,
                          ),
                        ),
                      ],
                    ),
                  ),
                  if (active) ...[
                    const SizedBox(width: 6),
                    _repetitionBadge(activeRepetition + 1, step.effectiveRepeat),
                  ],
                  _action('編輯', Icons.edit_outlined, ClickSpiritColors.secondary(context), () => onEdit(index)),
                  _action('複製', Icons.content_copy_outlined, ClickSpiritColors.secondary(context), () => onDuplicate(index)),
                  _action('刪除', Icons.delete_outline, ClickSpiritColors.systemRed, () => onDelete(index)),
                  ReorderableDragStartListener(
                    index: index,
                    child: Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 4),
                      child: Icon(
                        Icons.drag_handle_rounded,
                        key: ValueKey('drag-$index'),
                        color: ClickSpiritColors.tertiary(context),
                      ),
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

  Widget _badge(int number, bool active) {
    return Container(
      width: 28,
      height: 28,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: active ? ClickSpiritColors.systemBlue : ClickSpiritColors.systemBlue.withValues(alpha: 0.12),
        shape: BoxShape.circle,
      ),
      child: Text(
        '$number',
        style: TextStyle(
          fontSize: 13,
          fontWeight: FontWeight.w600,
          height: 1,
          color: active ? Colors.white : ClickSpiritColors.systemBlue,
        ),
      ),
    );
  }

  Widget _repetitionBadge(int current, int total) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: ClickSpiritColors.systemBlue,
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        '第 $current/$total 次',
        key: const ValueKey('running-repetition'),
        style: const TextStyle(
          color: Colors.white,
          fontSize: 11,
          fontWeight: FontWeight.w600,
          height: 1.1,
        ),
      ),
    );
  }

  Widget _action(String tooltip, IconData icon, Color color, VoidCallback onPressed) {
    return IconButton(
      tooltip: tooltip,
      onPressed: onPressed,
      visualDensity: VisualDensity.compact,
      padding: EdgeInsets.zero,
      constraints: const BoxConstraints.tightFor(width: 32, height: 32),
      icon: Icon(icon, size: 18, color: color),
    );
  }
}
