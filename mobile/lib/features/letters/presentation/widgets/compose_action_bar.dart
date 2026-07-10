import 'package:eoffice_mobile/features/letters/domain/draft_composer_state.dart';
import 'package:flutter/material.dart';

class ComposeActionBar extends StatelessWidget {
  const ComposeActionBar({
    required this.state,
    required this.onSave,
    required this.onPreview,
    required this.onSubmit,
    super.key,
  });

  final DraftComposerState state;
  final VoidCallback onSave;
  final VoidCallback onPreview;
  final VoidCallback onSubmit;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Material(
      color: scheme.surface,
      child: DecoratedBox(
        decoration: BoxDecoration(
          border: Border(top: BorderSide(color: scheme.outlineVariant)),
        ),
        child: SafeArea(
          top: false,
          child: Padding(
            padding: const EdgeInsets.fromLTRB(16, 10, 16, 12),
            child: LayoutBuilder(
              builder: (context, constraints) {
                final textScale =
                    (MediaQuery.textScalerOf(context).scale(14) / 14)
                        .clamp(1.0, 1.6);
                final sideStatus =
                    constraints.maxWidth >= 720 * textScale.clamp(1.0, 1.3);
                final labels = constraints.maxWidth >= 520 * textScale;
                final buttons = _ActionButtons(
                  state: state,
                  showLabels: labels,
                  onSave: onSave,
                  onPreview: onPreview,
                  onSubmit: onSubmit,
                );
                if (sideStatus) {
                  return Row(
                    children: [
                      Expanded(child: _SaveStatus(state: state)),
                      const SizedBox(width: 16),
                      buttons,
                    ],
                  );
                }
                return Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    _SaveStatus(state: state),
                    const SizedBox(height: 8),
                    buttons,
                  ],
                );
              },
            ),
          ),
        ),
      ),
    );
  }
}

class _ActionButtons extends StatelessWidget {
  const _ActionButtons({
    required this.state,
    required this.showLabels,
    required this.onSave,
    required this.onPreview,
    required this.onSubmit,
  });

  final DraftComposerState state;
  final bool showLabels;
  final VoidCallback onSave;
  final VoidCallback onPreview;
  final VoidCallback onSubmit;

  @override
  Widget build(BuildContext context) {
    if (!showLabels) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          _CompactAction(
            tooltip: 'Simpan draft',
            onPressed: state.busy ? null : onSave,
            icon: state.saveStatus == DraftComposerSaveStatus.saving
                ? const _ButtonProgress()
                : const Icon(Icons.save_outlined),
          ),
          const SizedBox(width: 12),
          _CompactAction(
            tooltip: 'Preview PDF',
            onPressed: state.busy ? null : onPreview,
            tonal: true,
            icon: state.previewing
                ? const _ButtonProgress()
                : const Icon(Icons.picture_as_pdf_outlined),
          ),
          const SizedBox(width: 12),
          _CompactAction(
            tooltip: 'Ajukan surat',
            onPressed: state.busy ? null : onSubmit,
            primary: true,
            icon: state.submitting
                ? const _ButtonProgress()
                : const Icon(Icons.send_outlined),
          ),
        ],
      );
    }
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        OutlinedButton.icon(
          onPressed: state.busy ? null : onSave,
          icon: state.saveStatus == DraftComposerSaveStatus.saving
              ? const _ButtonProgress()
              : const Icon(Icons.save_outlined),
          label: const Text('Simpan Draft'),
        ),
        const SizedBox(width: 8),
        FilledButton.tonalIcon(
          onPressed: state.busy ? null : onPreview,
          icon: state.previewing
              ? const _ButtonProgress()
              : const Icon(Icons.picture_as_pdf_outlined),
          label: const Text('Preview'),
        ),
        const SizedBox(width: 8),
        FilledButton.icon(
          onPressed: state.busy ? null : onSubmit,
          icon: state.submitting
              ? const _ButtonProgress()
              : const Icon(Icons.send_outlined),
          label: const Text('Ajukan'),
        ),
      ],
    );
  }
}

class _CompactAction extends StatelessWidget {
  const _CompactAction({
    required this.tooltip,
    required this.onPressed,
    required this.icon,
    this.tonal = false,
    this.primary = false,
  });

  final String tooltip;
  final VoidCallback? onPressed;
  final Widget icon;
  final bool tonal;
  final bool primary;

  @override
  Widget build(BuildContext context) {
    final button = primary
        ? IconButton.filled(
            onPressed: onPressed,
            tooltip: tooltip,
            icon: icon,
          )
        : tonal
            ? IconButton.filledTonal(
                onPressed: onPressed,
                tooltip: tooltip,
                icon: icon,
              )
            : IconButton.outlined(
                onPressed: onPressed,
                tooltip: tooltip,
                icon: icon,
              );
    return Semantics(button: true, label: tooltip, child: button);
  }
}

class _SaveStatus extends StatelessWidget {
  const _SaveStatus({required this.state});

  final DraftComposerState state;

  @override
  Widget build(BuildContext context) {
    final (icon, label) = switch (state.saveStatus) {
      DraftComposerSaveStatus.saving => (Icons.sync, 'Menyimpan perubahan...'),
      DraftComposerSaveStatus.saved => (
          Icons.cloud_done_outlined,
          state.dirty ? 'Ada perubahan baru' : _savedLabel(state.lastSavedAt),
        ),
      DraftComposerSaveStatus.failed => (
          Icons.cloud_off_outlined,
          'Perubahan belum tersimpan'
        ),
      DraftComposerSaveStatus.idle => (
          state.dirty ? Icons.edit_outlined : Icons.cloud_outlined,
          state.dirty
              ? 'Perubahan belum tersimpan'
              : state.form.draftId == null
                  ? 'Draft baru'
                  : 'Draft tersimpan',
        ),
    };
    return Semantics(
      liveRegion: true,
      label: label,
      child: ExcludeSemantics(
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, size: 20),
            const SizedBox(width: 8),
            Flexible(
              child: Text(
                label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ButtonProgress extends StatelessWidget {
  const _ButtonProgress();

  @override
  Widget build(BuildContext context) {
    return const SizedBox.square(
      dimension: 18,
      child: CircularProgressIndicator(strokeWidth: 2),
    );
  }
}

String _savedLabel(DateTime? value) {
  if (value == null) return 'Draft tersimpan';
  final hour = value.hour.toString().padLeft(2, '0');
  final minute = value.minute.toString().padLeft(2, '0');
  return 'Tersimpan $hour:$minute';
}
