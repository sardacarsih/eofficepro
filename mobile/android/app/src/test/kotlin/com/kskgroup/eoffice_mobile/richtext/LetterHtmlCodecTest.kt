package com.kskgroup.eoffice_mobile.richtext

import android.graphics.Typeface
import android.text.Spanned
import android.text.style.StyleSpan
import android.text.style.UnderlineSpan
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * Round-trip goldens for the canonical letter HTML dialect.
 *
 * These fixtures must stay byte-identical to the normalizeEditableLetterHtml
 * goldens in mobile/test/letter_body_codec_test.dart — Dart normalizes
 * stored HTML into this form before seeding the editor, and the unedited-body
 * check relies on string equality with what this codec serializes.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class LetterHtmlCodecTest {

    private val codec = LetterHtmlCodec(density = 1f)

    private val canonicalGoldens = listOf(
        "<p>Halo <strong>Tim</strong>,</p><p>Mohon diproses untuk hari ini.</p>",
        "<p>a</p><p><br></p><p>b</p>",
        "<p>a <strong>b</strong> <em>c</em></p>",
        "<p><strong><em><u>a</u></em></strong></p>",
        "<p><strong>ab</strong></p>",
        "<ul><li>Satu</li><li><strong>Dua</strong></li></ul><ol><li>Pertama</li></ol>",
        "<ol><li><br></li><li>Isi</li></ol>",
        "<p>1 &lt; 2 &amp; 3</p>",
        "<p>Halo <strong>Tim</strong>,</p><p><br></p><p>Salam</p>",
        "<ul><li>Satu</li><li><em>Dua</em></li></ul><p>Penutup</p>",
    )

    @Test
    fun `serialize inverts parse for every canonical golden`() {
        for (golden in canonicalGoldens) {
            assertEquals(golden, codec.toLetterHtml(codec.fromLetterHtml(golden)))
        }
    }

    @Test
    fun `parse produces one line per block`() {
        val text = codec.fromLetterHtml("<p>a</p><p><br></p><p>b</p>")
        assertEquals("a\n\nb", text.toString())
    }

    @Test
    fun `parse maps inline tags onto style spans`() {
        val text = codec.fromLetterHtml("<p>Halo <strong>Tim</strong> <em>ya</em> <u>ok</u></p>")
        assertEquals("Halo Tim ya ok", text.toString())

        val bold = text.getSpans(0, text.length, StyleSpan::class.java)
            .single { it.style == Typeface.BOLD }
        assertEquals(5, text.getSpanStart(bold))
        assertEquals(8, text.getSpanEnd(bold))

        val italic = text.getSpans(0, text.length, StyleSpan::class.java)
            .single { it.style == Typeface.ITALIC }
        assertEquals("ya", text.substring(text.getSpanStart(italic), text.getSpanEnd(italic)))

        val underline = text.getSpans(0, text.length, UnderlineSpan::class.java).single()
        assertEquals("ok", text.substring(text.getSpanStart(underline), text.getSpanEnd(underline)))
    }

    @Test
    fun `parse attaches list item spans per paragraph`() {
        val text = codec.fromLetterHtml("<ul><li>Satu</li><li>Dua</li></ul><ol><li>Tiga</li></ol>")
        assertEquals("Satu\nDua\nTiga", text.toString())

        val items = text.getSpans(0, text.length, ListItemSpan::class.java)
        assertEquals(3, items.size)
        assertEquals(2, items.count { it.type == ListType.BULLET })
        assertEquals(1, items.count { it.type == ListType.NUMBER })
    }

    @Test
    fun `parse decodes entities and serialize re-escapes them`() {
        val text = codec.fromLetterHtml("<p>1 &lt; 2 &amp; 3</p>")
        assertEquals("1 < 2 & 3", text.toString())
        assertEquals("<p>1 &lt; 2 &amp; 3</p>", codec.toLetterHtml(text))
    }

    @Test
    fun `nested inline styles serialize in canonical order`() {
        val text = codec.fromLetterHtml("<p><strong><em><u>a</u></em></strong></p>")
        assertEquals("<p><strong><em><u>a</u></em></strong></p>", codec.toLetterHtml(text))
    }

    @Test
    fun `adjacent identical runs merge on serialization`() {
        val text = codec.fromLetterHtml("<p><strong>a</strong></p>")
        text.insert(1, "b")
        SpanFormatter.applyInline(text, 1, 2, SpanFormatter.InlineKind.BOLD)
        assertEquals("<p><strong>ab</strong></p>", codec.toLetterHtml(text))
    }

    @Test
    fun `empty input round-trips to empty output`() {
        assertEquals("", codec.toLetterHtml(codec.fromLetterHtml("")))
        assertTrue(codec.fromLetterHtml("").isEmpty())
    }

    @Test
    fun `toggling a list on a plain paragraph produces list markup`() {
        val text = codec.fromLetterHtml("<p>Satu</p><p>Dua</p>")
        SpanFormatter.toggleList(text, 0, text.length, ListType.BULLET, density = 1f)
        assertEquals("<ul><li>Satu</li><li>Dua</li></ul>", codec.toLetterHtml(text))

        SpanFormatter.toggleList(text, 0, text.length, ListType.BULLET, density = 1f)
        assertEquals("<p>Satu</p><p>Dua</p>", codec.toLetterHtml(text))
    }

    @Test
    fun `toggling inline styles over a selection round-trips`() {
        val text = codec.fromLetterHtml("<p>Mohon diproses</p>")
        SpanFormatter.toggleInline(text, 0, 5, SpanFormatter.InlineKind.BOLD)
        assertEquals("<p><strong>Mohon</strong> diproses</p>", codec.toLetterHtml(text))

        SpanFormatter.toggleInline(text, 0, 5, SpanFormatter.InlineKind.BOLD)
        assertEquals("<p>Mohon diproses</p>", codec.toLetterHtml(text))
    }
}
