package com.clickspirit.click_spirit

import android.annotation.SuppressLint
import android.content.Context
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.graphics.RectF
import android.view.KeyEvent
import android.view.MotionEvent
import android.view.View
import kotlin.math.hypot
import kotlin.math.roundToInt

/**
 * Full-screen touchable pick layer for the accessibility overlay window.
 *
 * Point mode: the crosshair follows the finger with a live "x, y" readout and
 * the point is taken on finger-up. Circle mode: finger-down sets the centre,
 * dragging sets the radius ("圓心 x, y · r=R"), finger-up confirms. The ✕
 * button (top right) or Back cancels.
 *
 * Coordinates come from MotionEvent.rawX/rawY: physical screen pixels, the
 * same space that GestureDescription taps use, so no scaling is needed.
 */
@SuppressLint("ViewConstructor")
class ScreenPickOverlay(
    context: Context,
    private val circle: Boolean,
    private val onResult: (Map<String, Any?>) -> Unit,
) : View(context) {
    private val density = resources.displayMetrics.density
    private val loc = IntArray(2)
    private var done = false
    private var fingerX = -1f
    private var fingerY = -1f
    private var centreX = -1f
    private var centreY = -1f
    private var pressed = false

    private val shade = Paint().apply { color = Color.argb(70, 0, 0, 0) }
    private val lineDark = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        color = Color.argb(200, 16, 16, 20)
        strokeWidth = 3f * density
    }
    private val lineLight = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        color = Color.WHITE
        strokeWidth = 1.2f * density
    }
    private val blue = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = Color.rgb(0x0A, 0x84, 0xFF) }
    private val blueFill = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = Color.argb(70, 0x0A, 0x84, 0xFF) }
    private val ring = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.STROKE
        strokeWidth = 2.5f * density
        color = Color.rgb(0x0A, 0x84, 0xFF)
    }
    private val pill = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = Color.argb(230, 28, 28, 30) }
    private val text = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        color = Color.WHITE
        textSize = 15f * density
        isFakeBoldText = true
    }
    private val rect = RectF()
    private val cancelRect = RectF()

    init {
        isFocusable = true
        isFocusableInTouchMode = true
    }

    private fun screenX(e: MotionEvent) = e.rawX
    private fun screenY(e: MotionEvent) = e.rawY

    @SuppressLint("ClickableViewAccessibility")
    override fun onTouchEvent(e: MotionEvent): Boolean {
        if (done) return true
        getLocationOnScreen(loc)
        when (e.actionMasked) {
            MotionEvent.ACTION_DOWN -> {
                if (cancelRect.contains(e.x, e.y)) {
                    finish(mapOf("ok" to false, "canceled" to true))
                    return true
                }
                fingerX = screenX(e); fingerY = screenY(e)
                pressed = true
                centreX = fingerX; centreY = fingerY
            }
            MotionEvent.ACTION_MOVE -> {
                fingerX = screenX(e); fingerY = screenY(e)
            }
            MotionEvent.ACTION_UP -> {
                fingerX = screenX(e); fingerY = screenY(e)
                if (!pressed) return true
                pressed = false
                if (circle) {
                    val r = hypot(fingerX - centreX, fingerY - centreY).roundToInt()
                    if (r >= 1) {
                        finish(mapOf("ok" to true, "x" to centreX.roundToInt(), "y" to centreY.roundToInt(), "r" to r))
                    }
                } else {
                    finish(mapOf("ok" to true, "x" to fingerX.toInt(), "y" to fingerY.toInt(), "r" to 0))
                }
            }
            MotionEvent.ACTION_CANCEL -> pressed = false
        }
        invalidate()
        return true
    }

    override fun onKeyUp(keyCode: Int, event: KeyEvent?): Boolean {
        if (keyCode == KeyEvent.KEYCODE_BACK || keyCode == KeyEvent.KEYCODE_ESCAPE) {
            finish(mapOf("ok" to false, "canceled" to true))
            return true
        }
        return super.onKeyUp(keyCode, event)
    }

    override fun dispatchKeyEvent(event: KeyEvent): Boolean {
        if (event.keyCode == KeyEvent.KEYCODE_BACK && event.action == KeyEvent.ACTION_UP) {
            finish(mapOf("ok" to false, "canceled" to true))
            return true
        }
        return super.dispatchKeyEvent(event)
    }

    private fun finish(result: Map<String, Any?>) {
        if (done) return
        done = true
        onResult(result)
    }

    private fun pillText(s: String, cx: Float, top: Float, canvas: Canvas) {
        val pad = 12f * density
        val h = 32f * density
        val w = text.measureText(s) + 2 * pad
        val left = (cx - w / 2).coerceIn(8f * density, maxOf(8f * density, width - w - 8f * density))
        rect.set(left, top, left + w, top + h)
        canvas.drawRoundRect(rect, h / 2, h / 2, pill)
        canvas.drawText(s, left + pad, top + h / 2 - (text.descent() + text.ascent()) / 2, text)
    }

    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
        getLocationOnScreen(loc)
        canvas.drawRect(0f, 0f, width.toFloat(), height.toFloat(), shade)
        val hint = if (circle) "按住圓心並拖曳出半徑，放開完成" else "點一下取得座標（放開時記錄）"
        pillText(hint, width / 2f, 48f * density, canvas)

        // Cancel button.
        val s = 44f * density
        cancelRect.set(width - s - 16f * density, 44f * density, width - 16f * density, 44f * density + s)
        canvas.drawRoundRect(cancelRect, s / 2, s / 2, pill)
        canvas.drawText("✕", cancelRect.centerX() - text.measureText("✕") / 2, cancelRect.centerY() - (text.descent() + text.ascent()) / 2, text)

        if (fingerX < 0) return
        val x = fingerX - loc[0]
        val y = fingerY - loc[1]
        var label = "${fingerX.toInt()}, ${fingerY.toInt()}"
        if (circle && pressed) {
            val cx = centreX - loc[0]
            val cy = centreY - loc[1]
            val r = hypot(fingerX - centreX, fingerY - centreY)
            canvas.drawCircle(cx, cy, r, blueFill)
            canvas.drawCircle(cx, cy, r, ring)
            canvas.drawCircle(cx, cy, 5f * density, blue)
            label = "圓心 ${centreX.roundToInt()}, ${centreY.roundToInt()} · r=${r.roundToInt()}"
        }
        canvas.drawLine(0f, y, width.toFloat(), y, lineDark)
        canvas.drawLine(x, 0f, x, height.toFloat(), lineDark)
        canvas.drawLine(0f, y, width.toFloat(), y, lineLight)
        canvas.drawLine(x, 0f, x, height.toFloat(), lineLight)
        // Readout above the finger so the finger does not cover it.
        val top = if (y > 140f * density) y - 96f * density else y + 56f * density
        pillText(label, x, top, canvas)
    }
}
