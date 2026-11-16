import 'dart:io';
import 'dart:convert';

import 'package:flutter/material.dart';

import '../engine/click_engine.dart';
import '../model/script.dart';
import 'glass.dart';
import 'theme.dart';

/// 錄影轉腳本 review: every recorded click with a video thumbnail and its
/// timecode. Steps can be deleted before saving; saving returns the Script.
Future<Script?> showVideoReview(
  BuildContext context, {
  required ClickEngine engine,
  required ScreenRecordingResult recording,
  required String name,
  String? thumbDir,
}) {
  return Navigator.of(context).push<Script>(
    MaterialPageRoute(
      builder: (_) => VideoReviewPage(engine: engine, recording: recording, name: name, thumbDir: thumbDir),
    ),
  );
}

class VideoReviewPage extends StatefulWidget {
  const VideoReviewPage({
    super.key,
    required this.engine,
    required this.recording,
    required this.name,
    this.thumbDir,
  });

  final ClickEngine engine;
  final ScreenRecordingResult recording;
  final String name;

  /// Where thumbnails are written. Null uses the system temp folder.
  final String? thumbDir;

  @override
  State<VideoReviewPage> createState() => _VideoReviewPageState();
}

class _ReviewItem {
  _ReviewItem(this.click, this.index);
  final TimedClick click;

  /// 1-based position in the original recording.
  final int index;
  String? thumb;
}

class _VideoReviewPageState extends State<VideoReviewPage> {
  late final List<_ReviewItem> _items = [
    for (var i = 0; i < widget.recording.clicks.length; i++) _ReviewItem(widget.recording.clicks[i], i + 1),
  ];
  late final TextEditingController _name = TextEditingController(text: widget.name);
  String _status = '';
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    if (widget.recording.hasVideo) _loadThumbs();
  }

  @override
  void dispose() {
    _name.dispose();
    super.dispose();
  }

  Future<void> _loadThumbs() async {
    final dir = widget.thumbDir ?? Directory.systemTemp.createTempSync('click-spirit-thumbs').path;
    for (final item in List.of(_items)) {
      final out = '$dir${Platform.pathSeparator}click-${item.index}.png';
      // Just after the press so the frame shows what was clicked.
      final err = await widget.engine.videoFrame(widget.recording.video, item.click.atMs, out, width: 320);
      if (!mounted) return;
      if (err.isEmpty) setState(() => item.thumb = out);
    }
  }

  void _delete(_ReviewItem item) {
    setState(() {
      _items.remove(item);
      _status = '已刪除第 ${item.index} 下';
    });
  }

  Future<void> _save() async {
    if (_items.isEmpty) {
      setState(() => _status = '沒有可儲存的點擊');
      return;
    }
    setState(() => _saving = true);
    final raw = await widget.engine.clicksToScript(_items.map((e) => e.click).toList(), _name.text.trim());
    if (!mounted) return;
    final err = payloadError(raw);
    if (err != null) {
      setState(() {
        _saving = false;
        _status = '無法產生腳本：$err';
      });
      return;
    }
    try {
      final script = Script.fromJson(Map<String, dynamic>.from(jsonDecode(raw) as Map));
      Navigator.of(context).pop(script);
    } catch (e) {
      setState(() {
        _saving = false;
        _status = '無法解析腳本：$e';
      });
    }
  }

  /// Gap from this click's release to the next press: the step interval.
  String _gapText(int i) {
    if (i + 1 >= _items.length) return '最後一步';
    final c = _items[i].click;
    final gap = _items[i + 1].click.atMs - c.atMs - c.holdMs;
    return '間隔 ${gap < 1 ? 1 : gap} ms';
  }

  @override
  Widget build(BuildContext context) {
    final secondary = ClickSpiritColors.secondary(context);
    final rec = widget.recording;
    return GlassBackground(
      child: Scaffold(
        backgroundColor: Colors.transparent,
        appBar: AppBar(
          forceMaterialTransparency: true,
          title: const Text('錄影轉腳本 · 檢查點擊'),
        ),
        body: Padding(
          padding: const EdgeInsets.fromLTRB(20, 4, 20, 16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              GlassPanel(
                padding: const EdgeInsets.fromLTRB(18, 14, 18, 14),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      rec.hasVideo ? '影片：${rec.video}' : '沒有影片（只記錄點擊）',
                      key: const Key('review-video'),
                      style:
                          TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: ClickSpiritColors.label(context)),
                    ),
                    if (rec.note.isNotEmpty) ...[
                      const SizedBox(height: 4),
                      Text(rec.note, style: TextStyle(fontSize: 12, color: secondary)),
                    ],
                    const SizedBox(height: 4),
                    Text(
                      '${_items.length} 下點擊 → ${_items.length} 個步驟；兩次點擊的時間差成為步驟間隔。刪掉不要的再儲存。',
                      style: TextStyle(fontSize: 12, color: secondary),
                    ),
                    const SizedBox(height: 10),
                    GlassTextField(fieldKey: const Key('review-name'), controller: _name, label: '腳本名稱'),
                  ],
                ),
              ),
              const SizedBox(height: 12),
              Expanded(
                child: GlassPanel(
                  padding: const EdgeInsets.all(6),
                  child: _items.isEmpty
                      ? Center(child: Text('沒有點擊', style: TextStyle(color: secondary)))
                      : ListView.separated(
                          itemCount: _items.length,
                          separatorBuilder: (_, __) => const SizedBox(height: 6),
                          itemBuilder: (context, i) => _row(i),
                        ),
                ),
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: Text(_status,
                        key: const Key('review-status'), style: TextStyle(fontSize: 13, color: secondary)),
                  ),
                  GlassButton(
                    onPressed: () => Navigator.of(context).pop(),
                    label: '取消',
                    variant: GlassButtonVariant.tinted,
                  ),
                  const SizedBox(width: 10),
                  GlassButton(
                    key: const Key('review-save'),
                    onPressed: _saving ? null : _save,
                    icon: Icons.check_rounded,
                    label: '存成腳本',
                    variant: GlassButtonVariant.filled,
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _row(int i) {
    final item = _items[i];
    final c = item.click;
    final secondary = ClickSpiritColors.secondary(context);
    return Container(
      key: Key('review-row-${item.index}'),
      padding: const EdgeInsets.all(8),
      decoration: BoxDecoration(
        color: ClickSpiritColors.systemBlue.withValues(alpha: 0.05),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Row(
        children: [
          _thumb(item),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '步驟 ${i + 1}',
                  style: TextStyle(fontSize: 17, fontWeight: FontWeight.w700, color: ClickSpiritColors.label(context)),
                ),
                const SizedBox(height: 2),
                Text(
                  timecode(c.atMs),
                  key: Key('review-time-${item.index}'),
                  style: const TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w600,
                    color: ClickSpiritColors.systemBlue,
                    fontFeatures: [FontFeature.tabularFigures()],
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  '(${c.x}, ${c.y}) · ${c.button == 'right' ? '右鍵' : c.button == 'middle' ? '中鍵' : '左鍵'} · 按住 ${c.holdMs} ms · ${_gapText(i)}',
                  style: TextStyle(fontSize: 13, color: secondary),
                ),
              ],
            ),
          ),
          IconButton(
            key: Key('review-delete-${item.index}'),
            tooltip: '刪除這一步',
            onPressed: () => _delete(item),
            icon: const Icon(Icons.delete_outline_rounded, color: ClickSpiritColors.systemRed),
          ),
        ],
      ),
    );
  }

  Widget _thumb(_ReviewItem item) {
    const w = 160.0;
    final rec = widget.recording;
    final h = rec.screenW > 0 && rec.screenH > 0 ? w * rec.screenH / rec.screenW : 90.0;
    final path = item.thumb;
    return ClipRRect(
      borderRadius: BorderRadius.circular(10),
      child: SizedBox(
        width: w,
        height: h,
        child: Stack(
          fit: StackFit.expand,
          children: [
            if (path != null)
              Image.file(File(path), fit: BoxFit.fill, gaplessPlayback: true)
            else
              ColoredBox(
                color: ClickSpiritColors.systemGray.withValues(alpha: 0.18),
                child: const Icon(Icons.videocam_off_outlined, color: ClickSpiritColors.systemGray),
              ),
            if (rec.screenW > 0 && rec.screenH > 0)
              Positioned(
                left: item.click.x * w / rec.screenW - 7,
                top: item.click.y * h / rec.screenH - 7,
                child: Container(
                  width: 14,
                  height: 14,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: ClickSpiritColors.systemRed.withValues(alpha: 0.85),
                    border: Border.all(color: Colors.white, width: 2),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
