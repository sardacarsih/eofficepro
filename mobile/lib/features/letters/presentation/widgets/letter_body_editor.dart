import 'dart:io';
import 'dart:math' as math;

import 'package:eoffice_mobile/features/letters/domain/letter_body_codec.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/native_rich_text_view.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/rich_text_toolbar.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

const _minEditorHeightDp = 288.0; // ~12 text lines, like the old minLines: 12.
const _bodyHint = 'Tulis isi surat di sini';
const _requiredBodyMessage = 'Isi surat wajib diisi';

/// Letter body editor.
///
/// On Android this hosts the native rich-text EditText plus a formatting
/// toolbar. Bodies whose stored HTML exceeds the editable subset stay locked
/// read-only exactly like before, and on other host platforms (widget tests)
/// it degrades to the previous plain-text `TextFormField`.
class LetterBodyEditor extends StatefulWidget {
  const LetterBodyEditor({
    required this.value,
    required this.plainValue,
    required this.enabled,
    required this.readOnly,
    required this.onChanged,
    required this.onSimplifyRequested,
    this.useNativeEditor,
    super.key,
  });

  /// Canonical editable HTML (DraftComposerForm.bodyHtml).
  final String value;

  /// Plain-text projection (DraftComposerForm.bodyPlain).
  final String plainValue;

  /// False while the form is busy (saving/previewing/submitting).
  final bool enabled;

  /// True when the stored HTML is outside the editable subset.
  final bool readOnly;

  final void Function(String html, String plain) onChanged;
  final VoidCallback onSimplifyRequested;

  /// Overrides platform detection; used by widget tests.
  final bool? useNativeEditor;

  @override
  State<LetterBodyEditor> createState() => _LetterBodyEditorState();
}

class _LetterBodyEditorState extends State<LetterBodyEditor> {
  final _plainController = TextEditingController();
  final _fieldKey = GlobalKey<FormFieldState<String>>();
  RichTextEditorChannel? _channel;
  var _editorHtml = '';
  var _creationHtml = '';
  var _formatState = const RichTextFormatState();
  var _focused = false;
  var _contentHeight = 0.0;

  bool get _useNative =>
      !widget.readOnly &&
      (widget.useNativeEditor ?? (!kIsWeb && Platform.isAndroid));

  @override
  void initState() {
    super.initState();
    _plainController.text = widget.plainValue;
    _editorHtml = widget.value;
  }

  @override
  void didUpdateWidget(covariant LetterBodyEditor oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (_useNative) {
      if (widget.value != _editorHtml) {
        _editorHtml = widget.value;
        _scheduleNativeContentSync(
          html: widget.value,
          plain: widget.plainValue,
        );
      }
      if (widget.enabled != oldWidget.enabled) {
        _channel?.setEnabled(widget.enabled);
      }
    }
    if (_plainController.text != widget.plainValue) {
      _schedulePlainTextSync(widget.plainValue);
    }
  }

  void _scheduleNativeContentSync({
    required String html,
    required String plain,
  }) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || !_useNative || widget.value != html) return;
      _channel?.setHtml(html);
      _fieldKey.currentState?.didChange(plain);
    });
  }

  void _schedulePlainTextSync(String value) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || widget.plainValue != value) return;
      _plainController.value = TextEditingValue(
        text: value,
        selection: TextSelection.collapsed(offset: value.length),
      );
    });
  }

  @override
  void dispose() {
    _channel?.dispose();
    _plainController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (widget.readOnly) return _buildLocked();
    if (_useNative) return _buildNative(context);
    return _buildFallback();
  }

  Widget _buildLocked() {
    return TextFormField(
      controller: _plainController,
      enabled: widget.enabled,
      readOnly: true,
      minLines: 12,
      maxLines: null,
      decoration: InputDecoration(
        alignLabelWithHint: true,
        labelText: 'Isi Surat',
        hintText: _bodyHint,
        helperText: 'Isi berformat lanjutan dipertahankan tanpa perubahan.',
        suffixIcon: IconButton(
          tooltip: 'Edit sebagai teks biasa',
          onPressed: widget.enabled ? widget.onSimplifyRequested : null,
          icon: const Icon(Icons.lock_outline),
        ),
      ),
      validator: _validateBody,
    );
  }

  Widget _buildFallback() {
    return TextFormField(
      controller: _plainController,
      enabled: widget.enabled,
      minLines: 12,
      maxLines: null,
      keyboardType: TextInputType.multiline,
      textCapitalization: TextCapitalization.sentences,
      decoration: const InputDecoration(
        alignLabelWithHint: true,
        labelText: 'Isi Surat',
        hintText: _bodyHint,
      ),
      validator: _validateBody,
      onChanged: (value) =>
          widget.onChanged(plainTextToLetterHtml(value), value),
    );
  }

  Widget _buildNative(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return FormField<String>(
      key: _fieldKey,
      initialValue: widget.plainValue,
      validator: _validateBody,
      autovalidateMode: AutovalidateMode.onUserInteraction,
      builder: (field) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            RichTextToolbar(
              enabled: widget.enabled,
              formatState: _formatState,
              onFormat: (format) => _channel?.applyFormat(format),
            ),
            const SizedBox(height: 8),
            InputDecorator(
              decoration: InputDecoration(
                labelText: 'Isi Surat',
                alignLabelWithHint: true,
                errorText: field.errorText,
                enabled: widget.enabled,
              ),
              isFocused: _focused,
              isEmpty: false,
              child: SizedBox(
                height: math.max(_minEditorHeightDp, _contentHeight),
                child: NativeRichTextView(
                  creationParams: _creationParams(scheme),
                  onViewCreated: _onViewCreated,
                ),
              ),
            ),
          ],
        );
      },
    );
  }

  Map<String, Object?> _creationParams(ColorScheme scheme) {
    _creationHtml = widget.value;
    return {
      'initialHtml': widget.value,
      'hint': _bodyHint,
      'enabled': widget.enabled,
      'textSizeSp': 16.0,
      'minHeightDp': _minEditorHeightDp,
      'textColor': scheme.onSurface.toARGB32(),
      'hintColor': scheme.onSurfaceVariant.toARGB32(),
    };
  }

  void _onViewCreated(int viewId) {
    _channel?.dispose();
    _channel = RichTextEditorChannel(
      viewId: viewId,
      onChanged: (html, plain) {
        _editorHtml = html;
        _fieldKey.currentState?.didChange(plain);
        widget.onChanged(html, plain);
      },
      onSelectionChanged: (state) {
        if (mounted) setState(() => _formatState = state);
      },
      onContentHeight: (heightDp) {
        if (mounted && (heightDp - _contentHeight).abs() > 0.5) {
          setState(() => _contentHeight = heightDp);
        }
      },
      onFocusChanged: (focused) {
        if (mounted) setState(() => _focused = focused);
      },
    );
    // The view may have been created after the form state moved on (e.g. a
    // draft finished opening in the meantime); reseed if so.
    if (_editorHtml != _creationHtml) {
      _channel!.setHtml(_editorHtml);
    }
    if (!widget.enabled) {
      _channel!.setEnabled(false);
    }
  }

  String? _validateBody(String? value) {
    if (value == null || value.trim().isEmpty) return _requiredBodyMessage;
    return null;
  }
}
