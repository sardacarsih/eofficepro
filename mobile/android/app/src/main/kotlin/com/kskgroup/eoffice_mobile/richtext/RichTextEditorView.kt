package com.kskgroup.eoffice_mobile.richtext

import android.content.Context
import android.graphics.Color
import android.text.Editable
import android.text.InputType
import android.text.Spannable
import android.text.TextWatcher
import android.view.Gravity
import android.view.View
import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.platform.PlatformView
import kotlin.math.abs

/**
 * Native rich-text editor for the letter body, driven by the Dart side of
 * the channel in native_rich_text_view.dart.
 */
class RichTextEditorView(
    context: Context,
    messenger: BinaryMessenger,
    viewId: Int,
    params: Map<String, Any?>,
) : PlatformView, MethodChannel.MethodCallHandler {

    private val density = context.resources.displayMetrics.density
    private val codec = LetterHtmlCodec(density)
    private val channel =
        MethodChannel(messenger, "com.kskgroup.eoffice/rich_text_editor/$viewId")
    private val editText = LetterEditText(context)

    /** Guards programmatic mutations so they do not echo back as user edits. */
    private var suppressEvents = false
    private var textChangeInFlight = false
    private var insertedNewlineAt = -1
    private var pendingApplyRange: Pair<Int, Int>? = null
    private val pendingInline = mutableSetOf<SpanFormatter.InlineKind>()
    private var pendingPosition = -1
    private var lastReportedHeightDp = -1.0

    private val watcher = object : TextWatcher {
        override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {
            if (suppressEvents) return
            textChangeInFlight = true
        }

        override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {
            if (suppressEvents || s == null) return
            insertedNewlineAt =
                if (before == 0 && count == 1 && s[start] == '\n') start else -1
            pendingApplyRange =
                if (count > 0 && start == pendingPosition && pendingInline.isNotEmpty()) {
                    start to (start + count)
                } else {
                    null
                }
        }

        override fun afterTextChanged(editable: Editable) {
            if (suppressEvents) {
                textChangeInFlight = false
                return
            }
            withSuppressedEvents {
                applyPendingInline(editable)
                handleListContinuation(editable)
                normalizeListSpans(editable)
            }
            textChangeInFlight = false
            emitChanged(editable)
            emitSelectionState(editText.selectionStart, editText.selectionEnd)
            reportContentHeight()
        }
    }

    init {
        channel.setMethodCallHandler(this)
        editText.apply {
            setBackgroundColor(Color.TRANSPARENT)
            gravity = Gravity.TOP or Gravity.START
            inputType = InputType.TYPE_CLASS_TEXT or
                InputType.TYPE_TEXT_FLAG_MULTI_LINE or
                InputType.TYPE_TEXT_FLAG_CAP_SENTENCES
            hint = params["hint"] as? String ?: ""
            textSize = (params["textSizeSp"] as? Number)?.toFloat() ?: 16f
            minHeight = (((params["minHeightDp"] as? Number)?.toFloat() ?: 0f) * density).toInt()
            (params["textColor"] as? Number)?.let { setTextColor(it.toInt()) }
            (params["hintColor"] as? Number)?.let { setHintTextColor(it.toInt()) }
            isEnabled = params["enabled"] as? Boolean ?: true
            val pad = (12 * density).toInt()
            setPadding(pad, pad, pad, pad)
        }
        (params["initialHtml"] as? String)?.takeIf { it.isNotEmpty() }?.let { html ->
            withSuppressedEvents { editText.setText(codec.fromLetterHtml(html)) }
        }
        editText.addTextChangedListener(watcher)
        editText.onSelectionChangedListener = { selStart, selEnd ->
            if (!suppressEvents && !textChangeInFlight) {
                if (selStart != pendingPosition) {
                    pendingInline.clear()
                    pendingPosition = -1
                }
                emitSelectionState(selStart, selEnd)
            }
        }
        editText.setOnFocusChangeListener { _, focused ->
            channel.invokeMethod("onFocusChanged", mapOf("focused" to focused))
        }
        editText.addOnLayoutChangeListener { _, _, _, _, _, _, _, _, _ -> reportContentHeight() }
    }

    override fun getView(): View = editText

    override fun dispose() {
        channel.setMethodCallHandler(null)
    }

    override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
        when (call.method) {
            "setHtml" -> {
                val html = call.argument<String>("html") ?: ""
                withSuppressedEvents {
                    editText.setText(codec.fromLetterHtml(html))
                    editText.setSelection(editText.length())
                }
                pendingInline.clear()
                pendingPosition = -1
                reportContentHeight()
                result.success(null)
            }
            "setEnabled" -> {
                editText.isEnabled = call.argument<Boolean>("enabled") ?: true
                result.success(null)
            }
            "applyFormat" -> {
                applyFormat(call.argument<String>("format") ?: "")
                result.success(null)
            }
            "getHtml" -> {
                result.success(
                    mapOf(
                        "html" to codec.toLetterHtml(editText.text),
                        "plain" to editText.text.toString(),
                    ),
                )
            }
            "clearFocus" -> {
                editText.clearFocus()
                result.success(null)
            }
            else -> result.notImplemented()
        }
    }

    private fun applyPendingInline(editable: Editable) {
        val range = pendingApplyRange ?: return
        pendingApplyRange = null
        for (kind in pendingInline) {
            SpanFormatter.applyInline(editable, range.first, range.second, kind)
        }
        pendingInline.clear()
        pendingPosition = -1
    }

    /** Enter inside a list continues it; Enter on an empty item exits it. */
    private fun handleListContinuation(editable: Editable) {
        val pos = insertedNewlineAt
        insertedNewlineAt = -1
        if (pos < 0) return
        val str = editable.toString()
        val prevStart = str.lastIndexOf('\n', pos - 1) + 1
        val prevSpan = editable.getSpans(prevStart, pos, ListItemSpan::class.java)
            .firstOrNull() ?: return
        if (pos == prevStart) {
            editable.removeSpan(prevSpan)
            editable.delete(pos, pos + 1)
        } else {
            val nextStart = pos + 1
            val nextEnd = str.indexOf('\n', nextStart).let { if (it == -1) str.length else it }
            editable.setSpan(
                ListItemSpan(prevSpan.type, density),
                nextStart,
                nextEnd,
                LetterHtmlCodec.listSpanFlags(nextStart, nextEnd),
            )
        }
    }

    /**
     * Re-snaps every list span to exactly one paragraph after arbitrary
     * edits (deletes across paragraphs, pastes, IME rewrites).
     */
    private fun normalizeListSpans(editable: Editable) {
        val str = editable.toString()
        val claimed = mutableSetOf<Int>()
        for (span in editable.getSpans(0, editable.length, ListItemSpan::class.java)) {
            val spanStart = editable.getSpanStart(span)
            if (spanStart < 0) continue
            val paraStart = str.lastIndexOf('\n', spanStart - 1) + 1
            val paraEnd = str.indexOf('\n', paraStart).let { if (it == -1) str.length else it }
            if (!claimed.add(paraStart)) {
                editable.removeSpan(span)
                continue
            }
            if (spanStart != paraStart || editable.getSpanEnd(span) != paraEnd) {
                editable.removeSpan(span)
                editable.setSpan(
                    span,
                    paraStart,
                    paraEnd,
                    LetterHtmlCodec.listSpanFlags(paraStart, paraEnd),
                )
            }
        }
    }

    private fun applyFormat(format: String) {
        val editable = editText.text ?: return
        val start = editText.selectionStart.coerceAtLeast(0)
        val end = editText.selectionEnd.coerceAtLeast(start)
        withSuppressedEvents {
            when (format) {
                "bold" -> toggleInlineOrPending(editable, start, end, SpanFormatter.InlineKind.BOLD)
                "italic" ->
                    toggleInlineOrPending(editable, start, end, SpanFormatter.InlineKind.ITALIC)
                "underline" ->
                    toggleInlineOrPending(editable, start, end, SpanFormatter.InlineKind.UNDERLINE)
                "bulletList" ->
                    SpanFormatter.toggleList(editable, start, end, ListType.BULLET, density)
                "numberList" ->
                    SpanFormatter.toggleList(editable, start, end, ListType.NUMBER, density)
                else -> return@withSuppressedEvents
            }
        }
        editText.invalidate()
        emitChanged(editable)
        emitSelectionState(start, end)
        reportContentHeight()
    }

    private fun toggleInlineOrPending(
        editable: Spannable,
        start: Int,
        end: Int,
        kind: SpanFormatter.InlineKind,
    ) {
        if (start != end) {
            SpanFormatter.toggleInline(editable, start, end, kind)
            return
        }
        if (SpanFormatter.isInlineActive(editable, start, end, kind)) {
            SpanFormatter.splitInlineAt(editable, start, kind)
            pendingInline.remove(kind)
        } else if (!pendingInline.remove(kind)) {
            pendingInline.add(kind)
            pendingPosition = start
        }
    }

    private fun emitChanged(editable: Editable) {
        channel.invokeMethod(
            "onChanged",
            mapOf(
                "html" to codec.toLetterHtml(editable),
                "plain" to editable.toString(),
            ),
        )
    }

    private fun emitSelectionState(selStart: Int, selEnd: Int) {
        val editable = editText.text ?: return
        val start = selStart.coerceAtLeast(0)
        val end = selEnd.coerceAtLeast(start)
        fun inline(kind: SpanFormatter.InlineKind): Boolean =
            SpanFormatter.isInlineActive(editable, start, end, kind) ||
                (start == end && start == pendingPosition && pendingInline.contains(kind))
        channel.invokeMethod(
            "onSelectionChanged",
            mapOf(
                "bold" to inline(SpanFormatter.InlineKind.BOLD),
                "italic" to inline(SpanFormatter.InlineKind.ITALIC),
                "underline" to inline(SpanFormatter.InlineKind.UNDERLINE),
                "bulletList" to SpanFormatter.listStateAt(editable, start, end, ListType.BULLET),
                "numberList" to SpanFormatter.listStateAt(editable, start, end, ListType.NUMBER),
            ),
        )
    }

    private fun reportContentHeight() {
        editText.post {
            val layout = editText.layout ?: return@post
            val desiredPx =
                layout.height + editText.compoundPaddingTop + editText.compoundPaddingBottom
            val heightDp = desiredPx / density.toDouble()
            if (abs(heightDp - lastReportedHeightDp) >= 1.0) {
                lastReportedHeightDp = heightDp
                channel.invokeMethod("onContentHeight", mapOf("heightDp" to heightDp))
            }
        }
    }

    private inline fun withSuppressedEvents(block: () -> Unit) {
        val previous = suppressEvents
        suppressEvents = true
        try {
            block()
        } finally {
            suppressEvents = previous
        }
    }
}
