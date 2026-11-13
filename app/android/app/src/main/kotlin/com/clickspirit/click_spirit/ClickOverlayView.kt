package com.clickspirit.click_spirit

import android.content.Context
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.graphics.RectF
import android.os.SystemClock
import android.view.View
import kotlin.math.max
import kotlin.math.min

/**
 * Full-screen, transparent view that draws a ripple and a step label at each
 * click. It lives in a TYPE_ACCESSIBILITY_OVERLAY window with
 * FLAG_NOT_TOUCHABLE, so it never receives touches; gestures and the user's
 * fingers go to the app underneath.
 *
 * Coordinates passed to [addRipple] are screen pixels (the same ones the
 * gesture used). The view subtracts its own on-screen offset before drawing.
 */
class ClickOverlayView(context: Context, private val onIdle: () -> Unit) : View(context) {
    private data class Ripple(val x: Float, val y: Float, val label: String, val start: Long)

    private val ripples = ArrayList<Ripple>()
    private val density = resources.displayMetrics.density
    private val loc = IntArray(2)
    private val pill = RectF()

    private val ring = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.STROKE
        strokeWidth = 3.5f * density
        color = BLUE
    }
    private val halo = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.STROKE
        strokeWidth = 6f * density
        color = Color.WHITE
    }
    private val fill = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = BLUE }
    private val dotHalo = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = Color.WHITE }
    private val pillPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = Color.argb(225, 28, 28, 30) }
    private val text = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        color = Color.WHITE
        textSize = 13f * density
        isFakeBoldText = true
    }

    init {
        // Draw only; never take focus or accessibility focus.
        isFocusable = false
        isClickable = false
        importantForAccessibility = IMPORTANT_FOR_ACCESSIBILITY_NO_HIDE_DESCENDANTS
    }

    /** Must be called on the main thread. */
    fun addRipple(x: Float, y: Float, label: String) {
        ripples.add(Ripple(x, y, label, SystemClock.uptimeMillis()))
        while (ripples.size > MAX_RIPPLES) ripples.removeAt(0)
        postInvalidateOnAnimation()
    }

    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
        val now = SystemClock.uptimeMillis()
        ripples.removeAll { now - it.start > LIFE_MS }
        if (ripples.isEmpty()) {
            onIdle()
            return
        }
        getLocationOnScreen(loc)
        val newest = ripples.last()
        for (r in ripples) {
            val x = r.x - loc[0]
            val y = r.y - loc[1]
            val age = (now - r.start).toFloat()
            val t = min(1f, age / RIPPLE_MS)
            val ease = 1f - (1f - t) * (1f - t)
            val radius = (10f + 30f * ease) * density
            val alpha = if (age <= RIPPLE_MS) 255 else max(0, (255 * (1f - (age - RIPPLE_MS) / (LIFE_MS - RIPPLE_MS))).toInt())
            halo.alpha = alpha
            ring.alpha = alpha
            canvas.drawCircle(x, y, radius, halo)
            canvas.drawCircle(x, y, radius, ring)
            dotHalo.alpha = alpha
            fill.alpha = alpha
            canvas.drawCircle(x, y, 6f * density, dotHalo)
            canvas.drawCircle(x, y, 4f * density, fill)
            if (r === newest && r.label.isNotEmpty()) {
                drawLabel(canvas, x, y, r.label, alpha)
            }
        }
        postInvalidateOnAnimation()
    }

    private fun drawLabel(canvas: Canvas, x: Float, y: Float, label: String, alpha: Int) {
        val padH = 10f * density
        val h = 26f * density
        val w = text.measureText(label) + 2 * padH
        var left = x - w / 2
        var top = y + 48f * density
        if (top + h > height) top = y - 48f * density - h
        left = left.coerceIn(4f * density, max(4f * density, width - w - 4f * density))
        pill.set(left, top, left + w, top + h)
        pillPaint.alpha = (225 * alpha) / 255
        text.alpha = alpha
        canvas.drawRoundRect(pill, h / 2, h / 2, pillPaint)
        val baseline = top + h / 2 - (text.descent() + text.ascent()) / 2
        canvas.drawText(label, left + padH, baseline, text)
    }

    companion object {
        private val BLUE = Color.rgb(0x0A, 0x84, 0xFF)
        private const val RIPPLE_MS = 380f
        private const val LIFE_MS = 1100f
        private const val MAX_RIPPLES = 6
    }
}
