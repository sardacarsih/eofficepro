package com.kskgroup.eoffice_mobile.richtext

import android.graphics.Typeface
import android.text.Spannable
import android.text.Spanned
import android.text.style.CharacterStyle
import android.text.style.StyleSpan
import android.text.style.UnderlineSpan

/** Selection-based formatting operations on the editor's Spannable. */
object SpanFormatter {

    enum class InlineKind { BOLD, ITALIC, UNDERLINE }

    /** True when every character of [start, end) carries the style. */
    fun isInlineActive(text: Spanned, start: Int, end: Int, kind: InlineKind): Boolean {
        if (start == end) {
            return matchingSpans(text, start, start, kind).isNotEmpty()
        }
        var i = start
        while (i < end) {
            val next = text.nextSpanTransition(i, end, CharacterStyle::class.java)
            if (matchingSpans(text, i, next, kind).none { span ->
                    text.getSpanStart(span) <= i && text.getSpanEnd(span) >= next
                }
            ) {
                return false
            }
            i = next
        }
        return true
    }

    /** Toggles the style over a non-empty selection. */
    fun toggleInline(text: Spannable, start: Int, end: Int, kind: InlineKind) {
        if (isInlineActive(text, start, end, kind)) {
            removeInline(text, start, end, kind)
        } else {
            applyInline(text, start, end, kind)
        }
    }

    fun applyInline(text: Spannable, start: Int, end: Int, kind: InlineKind) {
        if (start >= end) return
        var mergedStart = start
        var mergedEnd = end
        for (span in matchingSpans(text, start, end, kind)) {
            mergedStart = minOf(mergedStart, text.getSpanStart(span))
            mergedEnd = maxOf(mergedEnd, text.getSpanEnd(span))
            text.removeSpan(span)
        }
        text.setSpan(
            newSpan(kind),
            mergedStart,
            mergedEnd,
            Spannable.SPAN_EXCLUSIVE_INCLUSIVE,
        )
    }

    fun removeInline(text: Spannable, start: Int, end: Int, kind: InlineKind) {
        for (span in matchingSpans(text, start, end, kind)) {
            val spanStart = text.getSpanStart(span)
            val spanEnd = text.getSpanEnd(span)
            text.removeSpan(span)
            if (spanStart < start) {
                text.setSpan(newSpan(kind), spanStart, start, Spannable.SPAN_EXCLUSIVE_EXCLUSIVE)
            }
            if (spanEnd > end) {
                text.setSpan(newSpan(kind), end, spanEnd, Spannable.SPAN_EXCLUSIVE_INCLUSIVE)
            }
        }
    }

    /**
     * Stops a style from continuing past a collapsed cursor: the styled run is
     * closed off so the next typed character is unstyled.
     */
    fun splitInlineAt(text: Spannable, position: Int, kind: InlineKind) {
        for (span in matchingSpans(text, position, position, kind)) {
            val spanStart = text.getSpanStart(span)
            val spanEnd = text.getSpanEnd(span)
            text.removeSpan(span)
            if (spanStart < position) {
                text.setSpan(newSpan(kind), spanStart, position, Spannable.SPAN_EXCLUSIVE_EXCLUSIVE)
            }
            if (spanEnd > position) {
                text.setSpan(newSpan(kind), position, spanEnd, Spannable.SPAN_EXCLUSIVE_INCLUSIVE)
            }
        }
    }

    /** Toggles bullet/numbered list membership for the selected paragraphs. */
    fun toggleList(
        text: Spannable,
        selStart: Int,
        selEnd: Int,
        type: ListType,
        density: Float,
    ) {
        val paragraphs = paragraphRanges(text.toString(), selStart, selEnd)
        val allTyped = paragraphs.all { (start, end) ->
            text.getSpans(start, end, ListItemSpan::class.java).any { it.type == type }
        }
        for ((start, end) in paragraphs) {
            for (span in text.getSpans(start, end, ListItemSpan::class.java)) {
                text.removeSpan(span)
            }
            if (!allTyped) {
                text.setSpan(
                    ListItemSpan(type, density),
                    start,
                    end,
                    LetterHtmlCodec.listSpanFlags(start, end),
                )
            }
        }
    }

    fun listStateAt(text: Spanned, selStart: Int, selEnd: Int, type: ListType): Boolean {
        val paragraphs = paragraphRanges(text.toString(), selStart, selEnd)
        return paragraphs.isNotEmpty() && paragraphs.all { (start, end) ->
            text.getSpans(start, end, ListItemSpan::class.java).any { it.type == type }
        }
    }

    /** Paragraph [start, end) ranges (newline excluded) covering the selection. */
    fun paragraphRanges(str: String, selStart: Int, selEnd: Int): List<Pair<Int, Int>> {
        val from = selStart.coerceIn(0, str.length)
        val to = selEnd.coerceIn(from, str.length)
        val ranges = mutableListOf<Pair<Int, Int>>()
        var start = str.lastIndexOf('\n', from - 1) + 1
        while (true) {
            val end = str.indexOf('\n', start).let { if (it == -1) str.length else it }
            ranges.add(start to end)
            if (end >= to || end == str.length) break
            start = end + 1
        }
        return ranges
    }

    private fun matchingSpans(
        text: Spanned,
        start: Int,
        end: Int,
        kind: InlineKind,
    ): List<CharacterStyle> {
        return text.getSpans(start, end, CharacterStyle::class.java).filter { span ->
            (text.getSpanFlags(span) and Spanned.SPAN_COMPOSING) == 0 && when (kind) {
                InlineKind.BOLD -> span is StyleSpan && (span.style and Typeface.BOLD) != 0
                InlineKind.ITALIC -> span is StyleSpan && (span.style and Typeface.ITALIC) != 0
                InlineKind.UNDERLINE -> span is UnderlineSpan
            }
        }
    }

    private fun newSpan(kind: InlineKind): CharacterStyle = when (kind) {
        InlineKind.BOLD -> StyleSpan(Typeface.BOLD)
        InlineKind.ITALIC -> StyleSpan(Typeface.ITALIC)
        InlineKind.UNDERLINE -> UnderlineSpan()
    }
}
