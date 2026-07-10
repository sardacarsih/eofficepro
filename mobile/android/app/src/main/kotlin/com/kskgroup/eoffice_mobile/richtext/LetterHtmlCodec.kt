package com.kskgroup.eoffice_mobile.richtext

import android.graphics.Typeface
import android.text.Spannable
import android.text.SpannableStringBuilder
import android.text.Spanned
import android.text.style.CharacterStyle
import android.text.style.StyleSpan
import android.text.style.UnderlineSpan

/**
 * Codec between the canonical letter HTML dialect and Spannable text.
 *
 * The dialect is produced by normalizeEditableLetterHtml in
 * mobile/lib/features/letters/domain/letter_body_codec.dart: attribute-free
 * p / strong / em / u / ul / ol / li, `<p><br></p>` for blank lines,
 * `<li><br></li>` for empty items, inline nesting strong > em > u, adjacent
 * identical runs merged, and & < > escaped. [toLetterHtml] must serialize
 * back to that exact byte form — the golden tests in LetterHtmlCodecTest
 * mirror the Dart cases in letter_body_codec_test.dart.
 *
 * android.text.Html is deliberately not used: it cannot represent lists and
 * injects dir/style attributes.
 */
class LetterHtmlCodec(private val density: Float) {

    private class PendingSpan(val span: Any, val start: Int, val end: Int, val flags: Int)

    fun fromLetterHtml(html: String): SpannableStringBuilder {
        val out = SpannableStringBuilder()
        // Spans with an INCLUSIVE end grow when text is appended at their end,
        // so they are attached only after the whole text has been built.
        val pendingSpans = mutableListOf<PendingSpan>()
        val openInline = ArrayDeque<Pair<String, Int>>()
        var paragraphStart = -1
        var listType: ListType? = null
        var index = 0

        fun openParagraph() {
            if (paragraphStart < 0) paragraphStart = out.length
        }

        fun closeParagraph(asListItem: Boolean) {
            if (paragraphStart < 0) return
            if (asListItem) {
                listType?.let { type ->
                    pendingSpans.add(
                        PendingSpan(
                            ListItemSpan(type, density),
                            paragraphStart,
                            out.length,
                            listSpanFlags(paragraphStart, out.length),
                        ),
                    )
                }
            }
            out.append('\n')
            paragraphStart = -1
        }

        while (index < html.length) {
            if (html[index] == '<') {
                val close = html.indexOf('>', index)
                if (close == -1) break
                val rawTag = html.substring(index + 1, close).trim()
                index = close + 1
                val closing = rawTag.startsWith("/")
                val name = rawTag.removePrefix("/").removeSuffix("/").trim().lowercase()
                when (name) {
                    "p" -> if (closing) closeParagraph(false) else openParagraph()
                    "li" -> if (closing) closeParagraph(true) else openParagraph()
                    "ul" -> listType = if (closing) null else ListType.BULLET
                    "ol" -> listType = if (closing) null else ListType.NUMBER
                    "br" -> Unit // Only appears in empty blocks; blankness is implicit.
                    "strong", "em", "u" ->
                        if (closing) {
                            val start = openInline.removeLastOrNull()
                                ?.takeIf { it.first == name }?.second ?: continue
                            if (out.length > start) {
                                pendingSpans.add(
                                    PendingSpan(
                                        inlineSpanFor(name),
                                        start,
                                        out.length,
                                        Spannable.SPAN_EXCLUSIVE_INCLUSIVE,
                                    ),
                                )
                            }
                        } else {
                            openInline.addLast(name to out.length)
                        }
                }
            } else {
                val next = html.indexOf('<', index).let { if (it == -1) html.length else it }
                val text = decodeEntities(html.substring(index, next))
                if (paragraphStart < 0 && text.isBlank()) {
                    index = next
                    continue
                }
                openParagraph()
                out.append(text)
                index = next
            }
        }
        closeParagraph(false)
        if (out.isNotEmpty() && out[out.length - 1] == '\n') {
            out.delete(out.length - 1, out.length)
        }
        for (pending in pendingSpans) {
            out.setSpan(
                pending.span,
                pending.start,
                pending.end.coerceAtMost(out.length),
                pending.flags,
            )
        }
        return out
    }

    fun toLetterHtml(text: Spanned): String {
        val str = text.toString()
        if (str.isEmpty() && text.getSpans(0, 0, ListItemSpan::class.java).isEmpty()) {
            return ""
        }
        val sb = StringBuilder()
        var openList: ListType? = null
        var start = 0
        while (true) {
            val end = str.indexOf('\n', start).let { if (it == -1) str.length else it }
            val itemType = text.getSpans(start, end, ListItemSpan::class.java)
                .firstOrNull()?.type
            if (itemType != openList) {
                openList?.let { sb.append(if (it == ListType.BULLET) "</ul>" else "</ol>") }
                itemType?.let { sb.append(if (it == ListType.BULLET) "<ul>" else "<ol>") }
                openList = itemType
            }
            val content = serializeInline(text, start, end)
            if (itemType != null) {
                sb.append("<li>").append(content.ifEmpty { "<br>" }).append("</li>")
            } else {
                sb.append("<p>").append(content.ifEmpty { "<br>" }).append("</p>")
            }
            if (end == str.length) break
            start = end + 1
        }
        openList?.let { sb.append(if (it == ListType.BULLET) "</ul>" else "</ol>") }
        return sb.toString()
    }

    private fun serializeInline(text: Spanned, start: Int, end: Int): String {
        val runs = mutableListOf<Run>()
        var i = start
        while (i < end) {
            val next = text.nextSpanTransition(i, end, CharacterStyle::class.java)
            val spans = text.getSpans(i, next, CharacterStyle::class.java).filter { span ->
                text.getSpanStart(span) <= i &&
                    text.getSpanEnd(span) >= next &&
                    (text.getSpanFlags(span) and Spanned.SPAN_COMPOSING) == 0
            }
            val run = Run(
                text = text.subSequence(i, next).toString().replace('\u00A0', ' '),
                bold = spans.any { it is StyleSpan && (it.style and Typeface.BOLD) != 0 },
                italic = spans.any { it is StyleSpan && (it.style and Typeface.ITALIC) != 0 },
                underline = spans.any { it is UnderlineSpan },
            )
            val last = runs.lastOrNull()
            if (last != null && last.sameStyle(run)) {
                runs[runs.size - 1] = last.copy(text = last.text + run.text)
            } else {
                runs.add(run)
            }
            i = next
        }
        return serializeLevel(runs, 0)
    }

    /** Mirrors _serializeLevel in letter_body_codec.dart: strong > em > u. */
    private fun serializeLevel(runs: List<Run>, level: Int): String {
        if (level == INLINE_TAGS.size) {
            return runs.joinToString("") { escape(it.text) }
        }
        fun flag(run: Run) = when (level) {
            0 -> run.bold
            1 -> run.italic
            else -> run.underline
        }
        val sb = StringBuilder()
        var i = 0
        while (i < runs.size) {
            val f = flag(runs[i])
            var j = i
            while (j < runs.size && flag(runs[j]) == f) j++
            val inner = serializeLevel(runs.subList(i, j), level + 1)
            if (f) sb.append("<${INLINE_TAGS[level]}>").append(inner).append("</${INLINE_TAGS[level]}>")
            else sb.append(inner)
            i = j
        }
        return sb.toString()
    }

    private data class Run(
        val text: String,
        val bold: Boolean,
        val italic: Boolean,
        val underline: Boolean,
    ) {
        fun sameStyle(other: Run) =
            bold == other.bold && italic == other.italic && underline == other.underline
    }

    companion object {
        private val INLINE_TAGS = listOf("strong", "em", "u")

        fun listSpanFlags(start: Int, end: Int): Int =
            if (start == end) Spannable.SPAN_INCLUSIVE_INCLUSIVE
            else Spannable.SPAN_EXCLUSIVE_INCLUSIVE

        fun inlineSpanFor(tag: String): CharacterStyle = when (tag) {
            "strong" -> StyleSpan(Typeface.BOLD)
            "em" -> StyleSpan(Typeface.ITALIC)
            else -> UnderlineSpan()
        }

        /** Matches Dart's HtmlEscape(HtmlEscapeMode.element). */
        fun escape(text: String): String = text
            .replace("&", "&amp;")
            .replace("<", "&lt;")
            .replace(">", "&gt;")

        fun decodeEntities(text: String): String = text
            .replace("&nbsp;", " ")
            .replace("&#160;", " ")
            .replace("&lt;", "<")
            .replace("&gt;", ">")
            .replace("&quot;", "\"")
            .replace("&#39;", "'")
            .replace("&amp;", "&")
    }
}
