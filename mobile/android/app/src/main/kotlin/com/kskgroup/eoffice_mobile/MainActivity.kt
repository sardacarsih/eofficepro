package com.kskgroup.eoffice_mobile

import com.kskgroup.eoffice_mobile.richtext.RichTextEditorViewFactory
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine

class MainActivity : FlutterActivity() {
    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        flutterEngine.platformViewsController.registry.registerViewFactory(
            "com.kskgroup.eoffice/rich_text_editor",
            RichTextEditorViewFactory(flutterEngine.dartExecutor.binaryMessenger),
        )
    }
}
