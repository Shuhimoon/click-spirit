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
                emit(
                    mapOf(
                        "kind" to (kind ?: ""),
                        "loop" to loop,
                        "step" to step,
                        "message" to (message ?: ""),
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
                "isRunning" -> async(result) { Mobile.isRunning() }
                "accessibilityTrusted" -> result.success(accessibilityEnabled())
                "openAccessibilitySettings" -> {
                    startActivity(Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS))
                    result.success(null)
                }
                "cursorPos" -> result.success(null)
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
