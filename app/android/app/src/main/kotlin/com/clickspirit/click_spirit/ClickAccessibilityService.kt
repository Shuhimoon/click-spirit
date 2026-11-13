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
        mainHandler.post { detachOverlay() }
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
    }
}
