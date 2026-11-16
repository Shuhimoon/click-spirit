package com.clickspirit.click_spirit

import android.annotation.SuppressLint
import android.content.Context
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.graphics.PixelFormat
import android.graphics.RectF
import android.graphics.Typeface
import android.os.Build
import android.view.Gravity
import android.view.MotionEvent
import android.view.View
import android.view.WindowManager
import kotlin.math.hypot

/**
 * 小浮窗: an always-on-top, draggable bubble drawn by the accessibility
 * service as a touchable TYPE_ACCESSIBILITY_OVERLAY. Tapping the knob
 * expands three buttons: 開始錄製/停止錄製, 開始腳本, 停止腳本.
 * [onAction] gets "record", "start", or "stop". [onRect] gets the window
 * rectangle in screen pixels so scripts never tap it.
 */
@SuppressLint("ViewConstructor")
class BubbleOverlay(
    context: Context,
    private val onAction: (String) -> Unit,
    private val onRect: (Int, Int, Int, Int, Boolean) -> Unit,
) : View(context) {
    private val density = resources.displayMetrics.density
    private val knob = (56 * density).toInt()
    private val button = (84 * density).toInt()
    private val pad = (6 * density).toInt()
    private val barH = (64 * density).toInt()
    private val slop = 6 * density

    private val wm = context.getSystemService(Context.WINDOW_SERVICE) as WindowManager
    private val params = WindowManager.LayoutParams(
        knob,
        knob,
        WindowManager.LayoutParams.TYPE_ACCESSIBILITY_OVERLAY,
        WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
            WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN or
            WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS,
        PixelFormat.TRANSLUCENT,
    ).apply {
        gravity = Gravity.TOP or Gravity.START
        title = "click-spirit-bubble"
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            layoutInDisplayCutoutMode = WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES
        }
    }

    /** Knob position (top-left) in screen pixels. */
    private var knobX = 0
    private var knobY = 0
    private var expanded = false
    private var opensLeft = false
    var recording = false
        set(value) {
            field = value
            invalidate()
        }
    var running = false
        set(value) {
            field = value
            invalidate()
        }
    var attached = false
        private set

    private val bg = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = Color.argb(225, 28, 28, 30) }
    private val knobPaint = Paint(Paint.ANTI_ALIAS_FLAG)
    private val ring = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.STROKE
        strokeWidth = 3 * density
        color = Color.WHITE
    }
    private val text = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        color = Color.WHITE
        textSize = 13 * density
        typeface = Typeface.DEFAULT_BOLD
        textAlign = Paint.Align.CENTER
    }
    private val sep = Paint().apply { color = Color.argb(60, 255, 255, 255) }

    private fun screenSize(): Pair<Int, Int> {
        val m = resources.displayMetrics
        return Pair(m.widthPixels, m.heightPixels)
    }

    fun show() {
        if (attached) return
        val (sw, sh) = screenSize()
        if (knobX == 0 && knobY == 0) {
            knobX = sw - knob - (12 * density).toInt()
            knobY = sh / 2 - knob / 2
        }
        expanded = false
        layoutWindow()
        try {
            wm.addView(this, params)
            attached = true
        } catch (t: Throwable) {
            attached = false
        }
        publish()
    }

    fun hide() {
        if (!attached) return
        try {
            wm.removeView(this)
        } catch (ignored: Throwable) {
        }
        attached = false
        onRect(0, 0, 0, 0, false)
    }

    /** Removes and re-adds the window so it sits above a newer overlay. */
    fun raise() {
        if (!attached) return
        try {
            wm.removeView(this)
            wm.addView(this, params)
        } catch (ignored: Throwable) {
        }
        publish()
    }

    private fun expandedWidth() = knob + 3 * button + 2 * pad

    private fun layoutWindow() {
        val (sw, _) = screenSize()
        if (expanded) {
            opensLeft = knobX + knob / 2 > sw / 2
            params.width = expandedWidth()
            params.height = barH
            params.x = if (opensLeft) knobX + knob - params.width else knobX
            params.y = knobY - (barH - knob) / 2
        } else {
            params.width = knob
            params.height = knob
            params.x = knobX
            params.y = knobY
        }
    }

    private fun relayout() {
        layoutWindow()
        if (attached) {
            try {
                wm.updateViewLayout(this, params)
            } catch (ignored: Throwable) {
            }
        }
        invalidate()
        publish()
    }

    private fun publish() {
        if (!attached) return
        onRect(params.x, params.y, params.width, params.height, true)
        // Correct for any system offset once the window is laid out.
        post {
            if (!attached) return@post
            val loc = IntArray(2)
            getLocationOnScreen(loc)
            onRect(loc[0], loc[1], params.width, params.height, true)
        }
    }

    override fun onDraw(canvas: Canvas) {
        val w = width.toFloat()
        val h = height.toFloat()
        val knobLeft: Float
        if (expanded) {
            canvas.drawRoundRect(RectF(0f, 0f, w, h), h / 2, h / 2, bg)
            knobLeft = if (opensLeft) w - knob else 0f
            val labels = listOf(if (recording) "■ 停止錄製" else "● 開始錄製", "▶ 開始腳本", "■ 停止腳本")
            val first = if (opensLeft) pad.toFloat() else (knob + pad).toFloat()
            for (i in 0 until 3) {
                val left = first + i * button
                val cx = left + button / 2f
                val cy = h / 2 - (text.descent() + text.ascent()) / 2
                text.color = when {
                    i == 0 && recording -> Color.rgb(255, 105, 97)
                    i == 2 && !running -> Color.argb(150, 255, 255, 255)
                    else -> Color.WHITE
                }
                canvas.drawText(labels[i], cx, cy, text)
                if (i > 0) canvas.drawRect(left, h * 0.25f, left + density, h * 0.75f, sep)
            }
        } else {
            knobLeft = 0f
        }
        val top = (h - knob) / 2
        val r = knob / 2f
        knobPaint.color = when {
            recording -> Color.rgb(255, 59, 48)
            running -> Color.rgb(0, 122, 255)
            else -> Color.argb(235, 60, 60, 67)
        }
        canvas.drawCircle(knobLeft + r, top + r, r - 2 * density, knobPaint)
        canvas.drawCircle(knobLeft + r, top + r, r * 0.45f, ring)
    }

    private var downRawX = 0f
    private var downRawY = 0f
    private var startKnobX = 0
    private var startKnobY = 0
    private var dragging = false

    @SuppressLint("ClickableViewAccessibility")
    override fun onTouchEvent(event: MotionEvent): Boolean {
        when (event.actionMasked) {
            MotionEvent.ACTION_DOWN -> {
                downRawX = event.rawX
                downRawY = event.rawY
                startKnobX = knobX
                startKnobY = knobY
                dragging = false
            }
            MotionEvent.ACTION_MOVE -> {
                val dx = event.rawX - downRawX
                val dy = event.rawY - downRawY
                if (!dragging && hypot(dx, dy) > slop) dragging = true
                if (dragging) {
                    knobX = startKnobX + dx.toInt()
                    knobY = startKnobY + dy.toInt()
                    relayout()
                }
            }
            MotionEvent.ACTION_UP -> {
                if (!dragging) tap(event.x)
                dragging = false
            }
            MotionEvent.ACTION_CANCEL -> dragging = false
        }
        return true
    }

    private fun tap(localX: Float) {
        if (!expanded) {
            expanded = true
            relayout()
            return
        }
        val first = if (opensLeft) pad.toFloat() else (knob + pad).toFloat()
        val index = ((localX - first) / button).toInt()
        val inButtons = localX >= first && index in 0..2
        expanded = false
        relayout()
        if (inButtons) {
            onAction(listOf("record", "start", "stop")[index])
        }
    }
}
