package com.clickspirit.click_spirit

import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.media.MediaMetadataRetriever
import android.media.projection.MediaProjectionManager
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.provider.Settings
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.io.FileOutputStream
import com.clickspirit.mobile.EventListener
import com.clickspirit.mobile.Mobile
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodChannel
import kotlin.concurrent.thread

/**
 * Flutter shell. MethodChannel "click_spirit/engine" calls the gomobile AAR.
 * EventChannel "click_spirit/events" forwards Go progress. Blocking Go calls
 * run off the main thread.
 */
class MainActivity : FlutterActivity() {
    private val mainHandler = Handler(Looper.getMainLooper())
    private val pending = ArrayDeque<Map<String, Any?>>()
    private var eventSink: EventChannel.EventSink? = null

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        val messenger = flutterEngine.dartExecutor.binaryMessenger

        Mobile.setListener(object : EventListener {
            override fun onEvent(kind: String?, loop: Long, step: Long, message: String?) {
                when (kind) {
                    "started", "step", "countdown" -> ClickAccessibilityService.instance?.setBubbleRunning(true)
                    "finished", "stopped", "error", "emergency_stop" ->
                        ClickAccessibilityService.instance?.setBubbleRunning(false)
                }
                // Clicks arrive with their position through onClick below.
                if (kind == "click" || kind == "countdown") return
                emit(
                    mapOf(
                        "kind" to (kind ?: ""),
                        "loop" to loop,
                        "step" to step,
                        "message" to (message ?: ""),
                    ),
                )
            }

            override fun onCountdown(remaining: Long) {
                emit(
                    mapOf(
                        "kind" to "countdown",
                        "loop" to 0L,
                        "step" to -1L,
                        "remaining" to remaining,
                        "message" to "$remaining 秒後開始",
                    ),
                )
            }

            override fun onClick(x: Long, y: Long, step: Long, repetition: Long, repeats: Long) {
                emit(
                    mapOf(
                        "kind" to "click",
                        "loop" to 0L,
                        "step" to step,
                        "repetition" to repetition,
                        "repeats" to repeats,
                        "x" to x,
                        "y" to y,
                        "message" to "",
                    ),
                )
            }
        })
        ClickAccessibilityService.bubbleListener = { action ->
            emit(mapOf("kind" to "bubble", "loop" to 0L, "step" to -1L, "message" to action))
        }
        ClickAccessibilityService.instance?.let { service ->
            try {
                Mobile.setDispatcher(service)
            } catch (ignored: Throwable) {
                // The AAR is loaded with the engine; ignore a missing class during early startup.
            }
        }

        MethodChannel(messenger, "click_spirit/engine").setMethodCallHandler { call, result ->
            when (call.method) {
                "version" -> result.success("click-spirit 0.1.0")
                "sample" -> async(result) { Mobile.sampleScript() }
                "validate" -> async(result) {
                    val json = call.argument<String>("json") ?: ""
                    try {
                        Mobile.validateScript(json)
                        ""
                    } catch (e: Exception) {
                        e.message ?: e.toString()
                    }
                }
                "start" -> async(result) {
                    val json = call.argument<String>("json") ?: ""
                    try {
                        Mobile.startScript(json)
                        ""
                    } catch (e: Exception) {
                        e.message ?: e.toString()
                    }
                }
                "stop" -> async(result) {
                    Mobile.stop()
                    null
                }
                "emergencyStop" -> async(result) {
                    Mobile.emergencyStop()
                    null
                }
                "setHotkey" -> result.success("Android 沒有全域熱鍵")
                "startRecording" -> startTapRecording(result)
                "stopRecording" -> stopTapRecording(result)
                "hasBubble" -> result.success(true)
                "setBubbleVisible" -> {
                    val service = ClickAccessibilityService.instance
                    if (service == null) {
                        result.error("click_spirit", "請先在設定中啟用點擊精靈的無障礙服務", null)
                    } else {
                        service.setBubbleVisible(call.argument<Boolean>("on") ?: false)
                        result.success(null)
                    }
                }
                "hasVideoRecorder" -> result.success(true)
                "startScreenRecording" -> startScreenRecording(call.argument<String>("path") ?: "", result)
                "stopScreenRecording" -> stopScreenRecording(result)
                "videoFrame" -> async(result) {
                    videoFrame(
                        call.argument<String>("video") ?: "",
                        asDouble(call.argument<Any>("at_ms")).toLong(),
                        call.argument<String>("out") ?: "",
                        asDouble(call.argument<Any>("width")).toInt(),
                    )
                }
                "clicksToScript" -> async(result) {
                    try {
                        Mobile.clicksToScript(call.argument<String>("clicks") ?: "[]", call.argument<String>("name") ?: "")
                    } catch (e: Exception) {
                        JSONObject().put("error", e.message ?: e.toString()).toString()
                    }
                }
                "matchImage" -> async(result) {
                    val template = asBytes(call.argument<Any>("template"))
                    val screen = asBytes(call.argument<Any>("screen"))
                    val threshold = asDouble(call.argument<Any>("threshold"))
                    if (template == null || screen == null) {
                        "{\"found\":false,\"x\":0,\"y\":0,\"score\":0,\"error\":\"Android 找圖需要螢幕與模板的 PNG 位元組，目前尚未接上截圖\"}"
                    } else {
                        Mobile.matchImage(screen, template, threshold)
                    }
                }
                "isRunning" -> async(result) { Mobile.isRunning() }
                "accessibilityTrusted" -> result.success(accessibilityEnabled())
                "openAccessibilitySettings" -> {
                    startActivity(Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS))
                    result.success(null)
                }
                "cursorPos" -> result.success(null)
                "setShowClicks" -> {
                    Mobile.setShowClicks(call.argument<Boolean>("on") ?: true)
                    result.success(null)
                }
                // The overlay is drawn by the accessibility service, which taps need anyway.
                "hasClickOverlay" -> result.success(true)
                "hasScreenPicker" -> result.success(true)
                "pickOnScreen" -> pickOnScreen(call.argument<String>("mode") ?: "point", result)
                else -> result.notImplemented()
            }
        }

        EventChannel(messenger, "click_spirit/events").setStreamHandler(object : EventChannel.StreamHandler {
            override fun onListen(arguments: Any?, sink: EventChannel.EventSink?) {
                eventSink = sink
                while (pending.isNotEmpty()) {
                    sink?.success(pending.removeFirst())
                }
            }

            override fun onCancel(arguments: Any?) {
                eventSink = null
            }
        })
    }

    /**
     * Moves the app to the back so the real screen shows, lets the
     * accessibility service run its full-screen pick overlay, then brings
     * the app back and replies {ok, canceled, x, y, r, error}.
     */
    private fun pickOnScreen(mode: String, result: MethodChannel.Result) {
        val service = ClickAccessibilityService.instance
        if (service == null) {
            result.success(mapOf("ok" to false, "error" to "請先在設定中啟用點擊精靈的無障礙服務"))
            return
        }
        moveTaskToBack(true)
        mainHandler.postDelayed({
            service.startPick(mode) { reply ->
                val back = packageManager.getLaunchIntentForPackage(packageName)?.apply {
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_REORDER_TO_FRONT)
                }
                try {
                    // Launch from the system-bound accessibility service: it is exempt
                    // from background-activity-start limits.
                    if (back != null) service.startActivity(back)
                } catch (ignored: Throwable) {
                }
                result.success(reply)
            }
        }, 350L)
    }

    private fun noService(): String =
        JSONObject().put("error", "請先在設定中啟用點擊精靈的無障礙服務（錄製需要它）").toString()

    /**
     * 開始錄製: a touchable full-screen overlay records each tap and replays
     * it with dispatchGesture (see TapRecorderOverlay). The app moves back so
     * the user can tap the real screen.
     */
    private fun startTapRecording(result: MethodChannel.Result) {
        val service = ClickAccessibilityService.instance
        if (service == null) {
            result.success(noService())
            return
        }
        if (!service.startTapRecording(SystemClock.uptimeMillis())) {
            result.success(JSONObject().put("error", "無法顯示錄製浮層").toString())
            return
        }
        result.success("{\"ok\":true}")
        moveTaskToBack(true)
    }

    private fun stopTapRecording(result: MethodChannel.Result) {
        val taps = ClickAccessibilityService.instance?.stopTapRecording()
        if (taps == null) {
            result.success("{\"error\":\"not recording\"}")
            return
        }
        if (JSONArray(taps).length() == 0) {
            result.success(JSONObject().put("error", "沒有錄到點擊").toString())
            return
        }
        async(result) {
            try {
                Mobile.clicksToScript(taps, "")
            } catch (e: Exception) {
                JSONObject().put("error", e.message ?: e.toString()).toString()
            }
        }
    }

    private var pendingScreen: MethodChannel.Result? = null
    private var screenPath = ""

    /**
     * 錄影轉腳本: asks for MediaProjection consent, starts the foreground
     * ScreenRecordService (MediaRecorder mp4) and the tap recorder with the
     * same time origin, then moves the app back.
     */
    private fun startScreenRecording(path: String, result: MethodChannel.Result) {
        val service = ClickAccessibilityService.instance
        if (service == null) {
            result.success("請先在設定中啟用點擊精靈的無障礙服務（記錄點擊需要它）")
            return
        }
        if (pendingScreen != null || ScreenRecordService.instance != null) {
            result.success("已經在錄影")
            return
        }
        screenPath = path.ifEmpty {
            val dir = File(getExternalFilesDir(null) ?: filesDir, "recordings").apply { mkdirs() }
            File(dir, "錄影-${System.currentTimeMillis()}.mp4").absolutePath
        }
        File(screenPath).parentFile?.mkdirs()
        val mpm = getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
        pendingScreen = result
        @Suppress("DEPRECATION")
        startActivityForResult(mpm.createScreenCaptureIntent(), REQ_SCREEN)
    }

    @Deprecated("FlutterActivity still routes results here")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        @Suppress("DEPRECATION")
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode != REQ_SCREEN) return
        val reply = pendingScreen ?: return
        pendingScreen = null
        if (resultCode != RESULT_OK || data == null) {
            reply.success("已取消錄影授權")
            return
        }
        ScreenRecordService.pendingStart = { start, err ->
            if (err != null) {
                reply.success("無法開始錄影：$err")
            } else {
                val service = ClickAccessibilityService.instance
                if (service == null || !service.startTapRecording(start)) {
                    ScreenRecordService.instance?.finish()
                    reply.success("無法顯示錄製浮層")
                } else {
                    reply.success("")
                    moveTaskToBack(true)
                }
            }
        }
        val intent = Intent(this, ScreenRecordService::class.java)
            .putExtra(ScreenRecordService.EXTRA_CODE, resultCode)
            .putExtra(ScreenRecordService.EXTRA_DATA, data)
            .putExtra(ScreenRecordService.EXTRA_PATH, screenPath)
        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) startForegroundService(intent) else startService(intent)
        } catch (t: Throwable) {
            ScreenRecordService.pendingStart = null
            reply.success("無法啟動錄影服務：${t.message}")
        }
    }

    private fun stopScreenRecording(result: MethodChannel.Result) {
        val taps = ClickAccessibilityService.instance?.stopTapRecording() ?: "[]"
        val err = ScreenRecordService.instance?.finish()
        val file = File(screenPath)
        val hasVideo = screenPath.isNotEmpty() && file.exists() && file.length() > 0
        val (sw, sh) = ScreenRecordService.realScreenSize(this)
        val out = JSONObject()
            .put("video", if (hasVideo) screenPath else "")
            .put("note", err ?: if (hasVideo) "" else "沒有產生影片，只記錄點擊")
            .put("clicks", JSONArray(taps))
            .put("screen_w", sw)
            .put("screen_h", sh)
        result.success(out.toString())
    }

    /** PNG thumbnail of [video] at [atMs]; "" on success. */
    private fun videoFrame(video: String, atMs: Long, out: String, width: Int): String {
        val retriever = MediaMetadataRetriever()
        return try {
            retriever.setDataSource(video)
            val frame = retriever.getFrameAtTime(atMs * 1000, MediaMetadataRetriever.OPTION_CLOSEST)
                ?: return "影片中沒有這個時間點"
            val w = if (width > 0) width else 320
            val h = (frame.height.toLong() * w / frame.width.coerceAtLeast(1)).toInt().coerceAtLeast(1)
            val scaled = Bitmap.createScaledBitmap(frame, w, h, true)
            FileOutputStream(out).use { scaled.compress(Bitmap.CompressFormat.PNG, 100, it) }
            ""
        } catch (t: Throwable) {
            t.message ?: t.toString()
        } finally {
            try {
                retriever.release()
            } catch (ignored: Throwable) {
            }
        }
    }

    override fun onDestroy() {
        ClickAccessibilityService.bubbleListener = null
        super.onDestroy()
    }

    private fun emit(event: Map<String, Any?>) {
        mainHandler.post {
            val sink = eventSink
            if (sink == null) {
                pending.addLast(event)
                while (pending.size > 50) pending.removeFirst()
            } else {
                sink.success(event)
            }
        }
    }

    private fun async(result: MethodChannel.Result, block: () -> Any?) {
        thread(name = "click-spirit") {
            try {
                val value = block()
                mainHandler.post { result.success(value) }
            } catch (e: Exception) {
                mainHandler.post {
                    result.error("click_spirit", e.message ?: e.toString(), null)
                }
            }
        }
    }

    private fun asBytes(value: Any?): ByteArray? = value as? ByteArray

    companion object {
        private const val REQ_SCREEN = 7301
    }

    private fun asDouble(value: Any?): Double = when (value) {
        is Double -> value
        is Float -> value.toDouble()
        is Int -> value.toDouble()
        is Long -> value.toDouble()
        is Number -> value.toDouble()
        else -> 0.0
    }

    private fun accessibilityEnabled(): Boolean {
        if (ClickAccessibilityService.instance != null) return true
        val expected = ComponentName(this, ClickAccessibilityService::class.java).flattenToString()
        val enabled = Settings.Secure.getString(
            contentResolver,
            Settings.Secure.ENABLED_ACCESSIBILITY_SERVICES,
        ) ?: return false
        return enabled.split(':').any { it.equals(expected, ignoreCase = true) }
    }
}
