package com.clickspirit.click_spirit

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.GestureDescription
import android.graphics.Path
import android.graphics.PixelFormat
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.util.Log
import android.view.Gravity
import android.view.WindowManager
import android.view.accessibility.AccessibilityEvent
import com.clickspirit.mobile.ClickOverlay
import com.clickspirit.mobile.GestureDispatcher
import com.clickspirit.mobile.Mobile
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.math.max

/**
 * Performs taps for the gomobile [GestureDispatcher] binding and draws the
 * run-time click overlay for [ClickOverlay].
 *
 * gomobile maps Go `int` to Java `long`. [tap] and [showClick] are invoked
 * from the Go runner goroutine. Gesture dispatch and view work run on the
 * main thread, and these methods never wait on the main thread for drawing.
 */
class ClickAccessibilityService : AccessibilityService(), GestureDispatcher, ClickOverlay {
    private val mainHandler = Handler(Looper.getMainLooper())
    private var overlay: ClickOverlayView? = null
    private val removeOverlay = Runnable { detachOverlay() }

    override fun onServiceConnected() {
        super.onServiceConnected()
        instance = this
        try {
            Mobile.setDispatcher(this)
            Mobile.setOverlay(this)
        } catch (t: Throwable) {
            Log.e(TAG, "setDispatcher failed", t)
        }
    }

    override fun onUnbind(intent: android.content.Intent?): Boolean {
        release()
        return super.onUnbind(intent)
    }

    override fun onDestroy() {
        release()
        super.onDestroy()
    }

    private fun release() {
        if (instance === this) {
            instance = null
            try {
                Mobile.setOverlay(null)
            } catch (ignored: Throwable) {
            }
        }
        mainHandler.post {
            detachOverlay()
            bubble?.hide()
            bubble = null
            tapRecorder?.detach()
            tapRecorder = null
        }
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {}

    override fun onInterrupt() {}

    override fun showClick(x: Long, y: Long, label: String?) {
        val text = label ?: ""
        mainHandler.post {
            val view = attachOverlay() ?: return@post
            mainHandler.removeCallbacks(removeOverlay)
            view.addRipple(x.toFloat(), y.toFloat(), text)
        }
    }

    override fun showCountdown(seconds: Long) {
        mainHandler.post {
            if (seconds <= 0) {
                overlay?.setCountdown(0)
                return@post
            }
            val view = attachOverlay() ?: return@post
            mainHandler.removeCallbacks(removeOverlay)
            view.setCountdown(seconds.toInt())
        }
    }

    private var bubble: BubbleOverlay? = null
    private var tapRecorder: TapRecorderOverlay? = null

    /** 小浮窗 on/off. Safe from any thread. */
    fun setBubbleVisible(on: Boolean) {
        mainHandler.post {
            if (on) {
                val b = bubble ?: BubbleOverlay(
                    this,
                    onAction = { action -> onBubbleAction(action) },
                    onRect = { x, y, w, h, visible ->
                        try {
                            Mobile.setBubbleRect(x.toLong(), y.toLong(), w.toLong(), h.toLong(), visible)
                        } catch (ignored: Throwable) {
                        }
                    },
                ).also { bubble = it }
                b.recording = tapRecorder != null
                b.show()
            } else {
                bubble?.hide()
            }
        }
    }

    private fun onBubbleAction(action: String) {
        val listener = bubbleListener
        if (listener != null) {
            listener(action)
        } else if (action == "stop") {
            // The app is gone; stopping still works without it.
            Thread { Mobile.stop() }.start()
        }
    }

    /** Colours the bubble blue while a script runs. */
    fun setBubbleRunning(running: Boolean) {
        mainHandler.post { bubble?.running = running }
    }

    val isTapRecording: Boolean
        get() = tapRecorder != null

    /**
     * Starts the tap-through recorder overlay (see [TapRecorderOverlay]).
     * Taps are timed from [startUptime] (SystemClock.uptimeMillis). Main thread.
     */
    fun startTapRecording(startUptime: Long): Boolean {
        if (tapRecorder != null) return true
        val rec = TapRecorderOverlay(this, startUptime)
        if (!rec.attach()) return false
        tapRecorder = rec
        // Keep the bubble above the recorder so its presses are not recorded.
        bubble?.let {
            it.recording = true
            it.raise()
        }
        return true
    }

    /** Stops the recorder and returns the taps as JSON, or null. Main thread. */
    fun stopTapRecording(): String? {
        val rec = tapRecorder ?: return null
        tapRecorder = null
        rec.detach()
        bubble?.recording = false
        return rec.tapsJson()
    }

    private var picker: ScreenPickOverlay? = null

    /**
     * 螢幕點選座標 / 從螢幕擷取設定圓形 on Android: adds a full-screen,
     * touchable TYPE_ACCESSIBILITY_OVERLAY that reports the tap (or the
     * dragged circle) in raw screen pixels, the same space dispatchGesture
     * uses, then removes itself. [done] runs on the main thread once.
     */
    fun startPick(mode: String, done: (Map<String, Any?>) -> Unit) {
        mainHandler.post {
            picker?.let { finishPick(it) }
            val wm = getSystemService(WINDOW_SERVICE) as? WindowManager
            if (wm == null) {
                done(mapOf("ok" to false, "error" to "無法取得 WindowManager"))
                return@post
            }
            lateinit var view: ScreenPickOverlay
            view = ScreenPickOverlay(this, mode == "circle") { result ->
                finishPick(view)
                done(result)
            }
            val params = WindowManager.LayoutParams(
                WindowManager.LayoutParams.MATCH_PARENT,
                WindowManager.LayoutParams.MATCH_PARENT,
                WindowManager.LayoutParams.TYPE_ACCESSIBILITY_OVERLAY,
                WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN or
                    WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS,
                PixelFormat.TRANSLUCENT,
            ).apply {
                gravity = Gravity.TOP or Gravity.START
                title = "click-spirit-picker"
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                    layoutInDisplayCutoutMode = WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES
                }
            }
            try {
                wm.addView(view, params)
                picker = view
                view.requestFocus()
            } catch (t: Throwable) {
                Log.e(TAG, "cannot add pick overlay", t)
                done(mapOf("ok" to false, "error" to "無法顯示選取浮層：${t.message}"))
            }
        }
    }

    private fun finishPick(view: ScreenPickOverlay) {
        if (picker === view) picker = null
        try {
            (getSystemService(WINDOW_SERVICE) as? WindowManager)?.removeView(view)
        } catch (ignored: Throwable) {
        }
    }

    /** Adds the click-through overlay window on first use. Main thread only. */
    private fun attachOverlay(): ClickOverlayView? {
        overlay?.let { return it }
        val wm = getSystemService(WINDOW_SERVICE) as? WindowManager ?: return null
        val view = ClickOverlayView(this) {
            // Keep the window briefly between clicks, then remove it.
            mainHandler.removeCallbacks(removeOverlay)
            mainHandler.postDelayed(removeOverlay, IDLE_REMOVE_MS)
        }
        val params = WindowManager.LayoutParams(
            WindowManager.LayoutParams.MATCH_PARENT,
            WindowManager.LayoutParams.MATCH_PARENT,
            WindowManager.LayoutParams.TYPE_ACCESSIBILITY_OVERLAY,
            WindowManager.LayoutParams.FLAG_NOT_TOUCHABLE or
                WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN or
                WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS,
            PixelFormat.TRANSLUCENT,
        ).apply {
            gravity = Gravity.TOP or Gravity.START
            title = "click-spirit-overlay"
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                layoutInDisplayCutoutMode = WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES
            }
        }
        return try {
            wm.addView(view, params)
            overlay = view
            view
        } catch (t: Throwable) {
            Log.e(TAG, "cannot add click overlay", t)
            null
        }
    }

    private fun detachOverlay() {
        val view = overlay ?: return
        overlay = null
        try {
            (getSystemService(WINDOW_SERVICE) as? WindowManager)?.removeView(view)
        } catch (t: Throwable) {
            Log.w(TAG, "remove overlay", t)
        }
    }

    override fun tap(x: Long, y: Long, durationMs: Long, count: Long): Boolean {
        if (Looper.myLooper() == Looper.getMainLooper()) {
            Log.e(TAG, "Tap called on the main thread; refusing to block")
            return false
        }
        val n = if (count < 1) 1 else count
        val dur = max(1L, durationMs)
        var i = 0L
        while (i < n) {
            if (!dispatchOne(x.toFloat(), y.toFloat(), dur)) return false
            i++
        }
        return true
    }

    private fun dispatchOne(x: Float, y: Float, durationMs: Long): Boolean {
        val latch = CountDownLatch(1)
        val ok = AtomicBoolean(false)
        mainHandler.post {
            val path = Path().apply { moveTo(x, y) }
            val stroke = GestureDescription.StrokeDescription(path, 0, max(1L, durationMs))
            val gesture = GestureDescription.Builder().addStroke(stroke).build()
            val dispatched = dispatchGesture(
                gesture,
                object : GestureResultCallback() {
                    override fun onCompleted(gestureDescription: GestureDescription?) {
                        ok.set(true)
                        latch.countDown()
                    }

                    override fun onCancelled(gestureDescription: GestureDescription?) {
                        latch.countDown()
                    }
                },
                null,
            )
            if (!dispatched) latch.countDown()
        }
        if (!latch.await(durationMs + 3000L, TimeUnit.MILLISECONDS)) return false
        return ok.get()
    }

    companion object {
        private const val TAG = "ClickSpiritA11y"
        private const val IDLE_REMOVE_MS = 3000L

        @Volatile
        var instance: ClickAccessibilityService? = null
            private set

        /** Set by MainActivity: bubble button presses go to Flutter. */
        @Volatile
        var bubbleListener: ((String) -> Unit)? = null
    }
}
