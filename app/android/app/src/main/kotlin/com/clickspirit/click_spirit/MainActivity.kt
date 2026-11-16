package com.clickspirit.click_spirit

import android.content.ComponentName
import android.content.Intent
import android.os.Handler
import android.os.Looper
import android.provider.Settings
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
                "startRecording" -> result.success(
                    "{\"error\":\"Android 不支援錄製，沒有全域輸入鉤子\"}",
                )
                "stopRecording" -> result.success(
                    "{\"error\":\"Android 不支援錄製，沒有全域輸入鉤子\"}",
                )
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
