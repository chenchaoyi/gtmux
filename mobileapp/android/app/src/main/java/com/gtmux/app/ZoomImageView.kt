package com.gtmux.app

import android.content.Context
import android.graphics.Matrix
import android.graphics.RectF
import android.os.Bundle
import android.view.GestureDetector
import android.view.MotionEvent
import android.view.ScaleGestureDetector
import android.view.accessibility.AccessibilityNodeInfo
import androidx.appcompat.widget.AppCompatImageView

/** Image-only reader: fit, pinch, double-tap and bounded pan; no horizontal page navigation. */
class ZoomImageView(context: Context) : AppCompatImageView(context) {
  private val transform = Matrix()
  private var fit = 1f
  private var zoom = 1f
  private var lastX = 0f
  private var lastY = 0f
  private val scale = ScaleGestureDetector(context, object : ScaleGestureDetector.SimpleOnScaleGestureListener() {
    override fun onScale(detector: ScaleGestureDetector): Boolean { magnify(detector.scaleFactor, detector.focusX, detector.focusY); return true }
  })
  private val gestures = GestureDetector(context, object : GestureDetector.SimpleOnGestureListener() {
    override fun onDown(event: MotionEvent) = true
    override fun onSingleTapConfirmed(event: MotionEvent): Boolean { performClick(); return true }
    override fun onDoubleTap(event: MotionEvent): Boolean {
      magnify((if (zoom > 1.5f) 1f else 2.5f) / zoom, event.x, event.y); return true
    }
  })
  init { scaleType = ScaleType.MATRIX; isFocusable = true; isClickable = true }
  override fun onSizeChanged(w: Int, h: Int, oldw: Int, oldh: Int) {
    super.onSizeChanged(w, h, oldw, oldh)
    val image = drawable ?: return
    if (w == 0 || h == 0 || image.intrinsicWidth <= 0 || image.intrinsicHeight <= 0) return
    fit = minOf(w.toFloat() / image.intrinsicWidth, h.toFloat() / image.intrinsicHeight)
    zoom = 1f; transform.setScale(fit, fit); clamp()
  }
  private fun magnify(factor: Float, x: Float, y: Float) {
    val next = (zoom * factor).coerceIn(1f, 6f)
    transform.postScale(next / zoom, next / zoom, x, y); zoom = next; clamp()
  }
  private fun clamp() {
    val image = drawable ?: return
    val box = RectF(0f, 0f, image.intrinsicWidth.toFloat(), image.intrinsicHeight.toFloat()); transform.mapRect(box)
    fun delta(start: Float, end: Float, limit: Int) = if (end - start <= limit) (limit - (end - start)) / 2 - start else if (start > 0) -start else if (end < limit) limit - end else 0f
    transform.postTranslate(delta(box.left, box.right, width), delta(box.top, box.bottom, height)); imageMatrix = transform
  }
  override fun onTouchEvent(event: MotionEvent): Boolean {
    scale.onTouchEvent(event); gestures.onTouchEvent(event)
    when (event.actionMasked) {
      MotionEvent.ACTION_DOWN -> {lastX = event.x; lastY = event.y; parent?.requestDisallowInterceptTouchEvent(true)}
      MotionEvent.ACTION_MOVE -> {
        if (!scale.isInProgress && event.pointerCount == 1) {transform.postTranslate(event.x - lastX, event.y - lastY); clamp()}
        lastX = event.x; lastY = event.y
      }
      MotionEvent.ACTION_POINTER_UP -> { val index = if (event.actionIndex == 0 && event.pointerCount > 1) 1 else 0; lastX = event.getX(index); lastY = event.getY(index) }
      MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> parent?.requestDisallowInterceptTouchEvent(false)
    }
    return true
  }
  override fun performClick(): Boolean {super.performClick(); return true}
  override fun onInitializeAccessibilityNodeInfo(info: AccessibilityNodeInfo) {
    super.onInitializeAccessibilityNodeInfo(info)
    val zh = resources.configuration.locales[0].language == "zh"
    if (zoom < 6f) info.addAction(AccessibilityNodeInfo.AccessibilityAction(AccessibilityNodeInfo.ACTION_SCROLL_FORWARD, if (zh) "放大图片" else "Zoom in"))
    if (zoom > 1f) info.addAction(AccessibilityNodeInfo.AccessibilityAction(AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD, if (zh) "缩小图片" else "Zoom out"))
  }
  override fun performAccessibilityAction(action: Int, arguments: Bundle?): Boolean = when (action) {
    AccessibilityNodeInfo.ACTION_SCROLL_FORWARD -> {magnify(1.5f, width / 2f, height / 2f); true}
    AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD -> {magnify(1 / 1.5f, width / 2f, height / 2f); true}
    else -> super.performAccessibilityAction(action, arguments)
  }
}
