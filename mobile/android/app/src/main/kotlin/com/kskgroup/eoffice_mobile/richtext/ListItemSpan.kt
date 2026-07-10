package com.kskgroup.eoffice_mobile.richtext

import android.graphics.Canvas
import android.graphics.Paint
import android.text.Layout
import android.text.Spanned
import android.text.style.LeadingMarginSpan

enum class ListType { BULLET, NUMBER }

/**
 * Marks one paragraph as a list item and draws its bullet or number in the
 * leading margin. The paragraph range is kept aligned by
 * RichTextEditorView.normalizeListSpans after every edit.
 */
class ListItemSpan(val type: ListType, density: Float) : LeadingMarginSpan {
    private val margin = (28 * density).toInt()
    private val gap = (8 * density).toInt()

    override fun getLeadingMargin(first: Boolean): Int = margin

    override fun drawLeadingMargin(
        canvas: Canvas,
        paint: Paint,
        x: Int,
        dir: Int,
        top: Int,
        baseline: Int,
        bottom: Int,
        text: CharSequence,
        start: Int,
        end: Int,
        first: Boolean,
        layout: Layout?,
    ) {
        if (!first) return
        val spanned = text as? Spanned ?: return
        val marker = when (type) {
            ListType.BULLET -> "•"
            ListType.NUMBER -> "${numberWithin(spanned)}."
        }
        val width = paint.measureText(marker)
        val markerX = if (dir >= 0) {
            x + margin - gap - width
        } else {
            (x - margin + gap).toFloat()
        }
        canvas.drawText(marker, markerX, baseline.toFloat(), paint)
    }

    /** 1-based position of this item within its run of numbered items. */
    private fun numberWithin(spanned: Spanned): Int {
        val str = spanned.toString()
        val spanStart = spanned.getSpanStart(this).coerceAtLeast(0)
        var lineStart = str.lastIndexOf('\n', spanStart - 1) + 1
        var number = 1
        while (lineStart > 0) {
            val prevEnd = lineStart - 1
            val prevStart = str.lastIndexOf('\n', prevEnd - 1) + 1
            val hasNumberItem = spanned
                .getSpans(prevStart, prevEnd, ListItemSpan::class.java)
                .any { it.type == ListType.NUMBER }
            if (!hasNumberItem) break
            number++
            lineStart = prevStart
        }
        return number
    }
}
