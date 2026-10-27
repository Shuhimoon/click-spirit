package com.clickspirit.click_spirit

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.GestureDescription
import android.graphics.Path
import android.os.Handler
import android.os.Looper
import android.util.Log
import android.view.accessibility.AccessibilityEvent
import com.clickspirit.mobile.GestureDispatcher
import com.clickspirit.mobile.Mobile
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.math.max

/**
 * Performs taps for the gomobile [GestureDispatcher] binding.
 *
 * gomobile maps Go `int` to Java `long`. [tap] is invoked from the Go runner
 * goroutine. Gesture dispatch itself must run on the main thread, and this
 * method never waits on the main thread.
 */
class ClickAccessibilityService : AccessibilityService(), GestureDispatcher {
    private val mainHandler = Handler(Looper.getMainLooper())

    override fun onServiceConnected() {
        super.onServiceConnected()
        instance = this
        try {
            Mobile.setDispatcher(this)
        } catch (t: Throwable) {
            Log.e(TAG, "setDispatcher failed", t)
        }
    }

    override fun onUnbind(intent: android.content.Intent?): Boolean {
        if (instance === this) instance = null
        return super.onUnbind(intent)
    }

    override fun onDestroy() {
        if (instance === this) instance = null
        super.onDestroy()
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {}

    override fun onInterrupt() {}

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

        @Volatile
        var instance: ClickAccessibilityService? = null
            private set
    }
}
