import 'package:eoffice_mobile/features/letters/presentation/widgets/native_rich_text_view.dart';
import 'package:flutter/material.dart';

/// Formatting toolbar for the native rich-text body editor.
class RichTextToolbar extends StatelessWidget {
  const RichTextToolbar({
    required this.enabled,
    required this.formatState,
    required this.onFormat,
    super.key,
  });

  final bool enabled;
  final RichTextFormatState formatState;
  final ValueChanged<String> onFormat;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return ExcludeFocus(
      child: DecoratedBox(
        decoration: BoxDecoration(
          border: Border.all(color: scheme.outlineVariant),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
          child: Wrap(
            spacing: 4,
            children: [
              _button(
                format: 'bold',
                icon: Icons.format_bold,
                tooltip: 'Tebal',
                selected: formatState.bold,
              ),
              _button(
                format: 'italic',
                icon: Icons.format_italic,
                tooltip: 'Miring',
                selected: formatState.italic,
              ),
              _button(
                format: 'underline',
                icon: Icons.format_underlined,
                tooltip: 'Garis bawah',
                selected: formatState.underline,
              ),
              _button(
                format: 'bulletList',
                icon: Icons.format_list_bulleted,
                tooltip: 'Daftar poin',
                selected: formatState.bulletList,
              ),
              _button(
                format: 'numberList',
                icon: Icons.format_list_numbered,
                tooltip: 'Daftar angka',
                selected: formatState.numberList,
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _button({
    required String format,
    required IconData icon,
    required String tooltip,
    required bool selected,
  }) {
    return IconButton(
      tooltip: tooltip,
      isSelected: selected,
      onPressed: enabled ? () => onFormat(format) : null,
      icon: Icon(icon),
      selectedIcon: Icon(icon),
    );
  }
}
