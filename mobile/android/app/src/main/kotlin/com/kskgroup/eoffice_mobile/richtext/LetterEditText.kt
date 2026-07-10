package com.kskgroup.eoffice_mobile.richtext

import android.content.Context
import android.widget.EditText

/** EditText that surfaces selection changes to RichTextEditorView. */
class LetterEditText(context: Context) : EditText(context) {

    var onSelectionChangedListener: ((Int, Int) -> Unit)? = null

    override fun onSelectionChanged(selStart: Int, selEnd: Int) {
        super.onSelectionChanged(selStart, selEnd)
        onSelectionChangedListener?.invoke(selStart, selEnd)
    }
}
