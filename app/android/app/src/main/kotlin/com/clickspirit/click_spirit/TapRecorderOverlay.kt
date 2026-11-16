package com.clickspirit.click_spirit

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.GestureDescription
import android.annotation.SuppressLint
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.graphics.Path
import android.graphics.PixelFormat
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.view.Gravity
import android.view.MotionEvent
import android.view.View
import android.view.WindowManager
import org.json.JSONArray
import org.json.JSONObject
import kotlin.math.hypot
import kotlin.math.max
import kotlin.math.min

/**
 * 開始錄製 on Android. Android has no global input hook, so this is a
 * full-screen, touchable TYPE_ACCESSIBILITY_OVERLAY: it records where each
 * finger goes down and up (raw screen pixels, timed from [startUptime]),
 * then turns itself untouchable, replays the same tap or swipe through
 * dispatchGesture so the app underneath still gets it, and becomes
 * touchable again. Only taps become script steps; swipes are passed through
 * but not recorded. The floating bubble is re-added above this overlay, so
 * presses on the bubble are never recorded.
 */
@SuppressLint("ViewConstructor")
class TapRecorderOverlay(
    private val service: AccessibilityService,
    private val startUptime: Long,
) : View(service) {
    data class Tap(val x: Int, val y: Int, val atMs: Long, val holdMs: Long)

    private val main = Handler(Looper.getMainLooper())
    private val wm = service.getSystemService(android.content.Context.WINDOW_SERVICE) as WindowManager
    private val taps = mutableListOf<Tap>()
    private val slop = 24 * resources.displayMetrics.density
    private var downX = 0f
    private var downY = 0f
    private var downAt = 0L
    private val path = Path()
    private var passing = false
    private val dot = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = Color.argb(150, 255, 59, 48) }
    private val frame = Paint().apply {
        style = Paint.Style.STROKE
        strokeWidth = 6f
        color = Color.argb(160, 255, 59, 48)
    }
    private var lastX = -1f
    private var lastY = -1f

    val params = WindowManager.LayoutParams(
        WindowManager.LayoutParams.MATCH_PARENT,
        WindowManager.LayoutParams.MATCH_PARENT,
        WindowManager.LayoutParams.TYPE_ACCESSIBILITY_OVERLAY,
        WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
            WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN or
            WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS,
        PixelFormat.TRANSLUCENT,
    ).apply {
        gravity = Gravity.TOP or Gravity.START
        title = "click-spirit-recorder"
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            layoutInDisplayCutoutMode = WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES
        }
    }

    fun attach(): Boolean = try {
        wm.addView(this, params)
        true
    } catch (t: Throwable) {
        false
    }

    fun detach() {
        try {
            wm.removeView(this)
        } catch (ignored: Throwable) {
        }
    }

    /** JSON array of {x, y, button, at_ms, hold_ms} for ClicksToScript. */
    fun tapsJson(): String {
        val arr = JSONArray()
        for (t in taps) {
            arr.put(
                JSONObject()
                    .put("x", t.x)
                    .put("y", t.y)
                    .put("button", "left")
                    .put("at_ms", t.atMs)
                    .put("hold_ms", t.holdMs),
            )
        }
        return arr.toString()
    }

    override fun onDraw(canvas: Canvas) {
        // A thin red frame says "recording"; the last tap gets a dot.
        canvas.drawRect(3f, 3f, width - 3f, height - 3f, frame)
        if (lastX >= 0) canvas.drawCircle(lastX, lastY, 18 * resources.displayMetrics.density, dot)
    }

    @SuppressLint("ClickableViewAccessibility")
    override fun onTouchEvent(event: MotionEvent): Boolean {
        if (passing) return true
        when (event.actionMasked) {
            MotionEvent.ACTION_DOWN -> {
                downX = event.rawX
                downY = event.rawY
                downAt = SystemClock.uptimeMillis()
                path.reset()
                path.moveTo(downX, downY)
            }
            MotionEvent.ACTION_MOVE -> path.lineTo(event.rawX, event.rawY)
            MotionEvent.ACTION_UP -> {
                val hold = max(1L, SystemClock.uptimeMillis() - downAt)
                val moved = hypot(event.rawX - downX, event.rawY - downY) > slop
                if (moved) {
                    path.lineTo(event.rawX, event.rawY)
                    replay(Path(path), min(hold, 3000L))
                } else {
                    taps.add(Tap(downX.toInt(), downY.toInt(), max(0L, downAt - startUptime), hold))
                    lastX = event.x - (event.rawX - downX)
                    lastY = event.y - (event.rawY - downY)
                    invalidate()
                    val p = Path().apply { moveTo(downX, downY) }
                    replay(p, min(hold, 1000L))
                }
            }
        }
        return true
    }

    /** Lets the gesture through to the app below, then takes touches again. */
    private fun replay(gesturePath: Path, durationMs: Long) {
        passing = true
        params.flags = params.flags or WindowManager.LayoutParams.FLAG_NOT_TOUCHABLE
        try {
            wm.updateViewLayout(this, params)
        } catch (ignored: Throwable) {
        }
        val restore = Runnable {
            params.flags = params.flags and WindowManager.LayoutParams.FLAG_NOT_TOUCHABLE.inv()
            try {
                wm.updateViewLayout(this, params)
            } catch (ignored: Throwable) {
            }
            passing = false
        }
        // Give the window manager a frame to apply NOT_TOUCHABLE first.
        main.postDelayed({
            val stroke = GestureDescription.StrokeDescription(gesturePath, 0, max(1L, durationMs))
            val ok = service.dispatchGesture(
                GestureDescription.Builder().addStroke(stroke).build(),
                object : AccessibilityService.GestureResultCallback() {
                    override fun onCompleted(gestureDescription: GestureDescription?) {
                        main.postDelayed(restore, 60L)
                    }

                    override fun onCancelled(gestureDescription: GestureDescription?) {
                        main.postDelayed(restore, 60L)
                    }
                },
                null,
            )
            if (!ok) main.post(restore)
        }, 40L)
    }
}
