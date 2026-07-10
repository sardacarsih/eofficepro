import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';

const richTextEditorViewType = 'com.kskgroup.eoffice/rich_text_editor';

/// Formatting active at the native editor's cursor/selection.
class RichTextFormatState {
  const RichTextFormatState({
    this.bold = false,
    this.italic = false,
    this.underline = false,
    this.bulletList = false,
    this.numberList = false,
  });

  factory RichTextFormatState.fromMap(Map<Object?, Object?> map) {
    return RichTextFormatState(
      bold: map['bold'] == true,
      italic: map['italic'] == true,
      underline: map['underline'] == true,
      bulletList: map['bulletList'] == true,
      numberList: map['numberList'] == true,
    );
  }

  final bool bold;
  final bool italic;
  final bool underline;
  final bool bulletList;
  final bool numberList;
}

/// Dart side of the per-view channel to RichTextEditorView.kt.
class RichTextEditorChannel {
  RichTextEditorChannel({
    required int viewId,
    this.onChanged,
    this.onSelectionChanged,
    this.onContentHeight,
    this.onFocusChanged,
  }) : _channel = MethodChannel('$richTextEditorViewType/$viewId') {
    _channel.setMethodCallHandler(_handleCall);
  }

  final MethodChannel _channel;
  void Function(String html, String plain)? onChanged;
  void Function(RichTextFormatState state)? onSelectionChanged;
  void Function(double heightDp)? onContentHeight;
  void Function(bool focused)? onFocusChanged;

  Future<Object?> _handleCall(MethodCall call) async {
    final arguments = call.arguments;
    final map = arguments is Map ? arguments : const <Object?, Object?>{};
    switch (call.method) {
      case 'onChanged':
        onChanged?.call(
          map['html'] as String? ?? '',
          map['plain'] as String? ?? '',
        );
      case 'onSelectionChanged':
        onSelectionChanged?.call(RichTextFormatState.fromMap(map));
      case 'onContentHeight':
        onContentHeight?.call((map['heightDp'] as num?)?.toDouble() ?? 0);
      case 'onFocusChanged':
        onFocusChanged?.call(map['focused'] == true);
    }
    return null;
  }

  Future<void> setHtml(String html) =>
      _channel.invokeMethod('setHtml', {'html': html});

  Future<void> setEnabled(bool enabled) =>
      _channel.invokeMethod('setEnabled', {'enabled': enabled});

  Future<void> applyFormat(String format) =>
      _channel.invokeMethod('applyFormat', {'format': format});

  Future<({String html, String plain})> getHtml() async {
    final result = await _channel.invokeMapMethod<String, Object?>('getHtml');
    return (
      html: result?['html'] as String? ?? '',
      plain: result?['plain'] as String? ?? '',
    );
  }

  Future<void> clearFocus() => _channel.invokeMethod('clearFocus');

  void dispose() => _channel.setMethodCallHandler(null);
}

/// Hosts the native Android rich-text EditText through texture-layer hybrid
/// composition (never virtual display, so the IME positions correctly).
class NativeRichTextView extends StatelessWidget {
  const NativeRichTextView({
    required this.creationParams,
    required this.onViewCreated,
    super.key,
  });

  final Map<String, Object?> creationParams;
  final ValueChanged<int> onViewCreated;

  @override
  Widget build(BuildContext context) {
    return PlatformViewLink(
      viewType: richTextEditorViewType,
      surfaceFactory: (context, controller) {
        return AndroidViewSurface(
          controller: controller as AndroidViewController,
          // Taps and long-presses reach the EditText (caret placement and
          // selection); vertical drags keep scrolling the surrounding form.
          gestureRecognizers: const <Factory<OneSequenceGestureRecognizer>>{
            Factory<TapGestureRecognizer>(TapGestureRecognizer.new),
            Factory<LongPressGestureRecognizer>(LongPressGestureRecognizer.new),
          },
          hitTestBehavior: PlatformViewHitTestBehavior.opaque,
        );
      },
      onCreatePlatformView: (params) {
        return PlatformViewsService.initSurfaceAndroidView(
          id: params.id,
          viewType: richTextEditorViewType,
          layoutDirection: TextDirection.ltr,
          creationParams: creationParams,
          creationParamsCodec: const StandardMessageCodec(),
          onFocus: () => params.onFocusChanged(true),
        )
          ..addOnPlatformViewCreatedListener(params.onPlatformViewCreated)
          ..addOnPlatformViewCreatedListener(onViewCreated)
          ..create();
      },
    );
  }
}
