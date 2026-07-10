import 'package:eoffice_mobile/features/letters/domain/draft_composer_state.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:flutter/material.dart';

class AttachmentSection extends StatelessWidget {
  const AttachmentSection({
    required this.state,
    required this.onPick,
    required this.onOpen,
    required this.onDelete,
    super.key,
  });

  final DraftComposerState state;
  final VoidCallback onPick;
  final ValueChanged<DraftAttachment> onOpen;
  final ValueChanged<DraftAttachment> onDelete;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return DecoratedBox(
      decoration: BoxDecoration(
        border: Border.all(color: scheme.outlineVariant),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            LayoutBuilder(
              builder: (context, constraints) {
                final textScale = MediaQuery.textScalerOf(context).scale(1);
                final stacked = constraints.maxWidth < 520 || textScale > 1.3;
                final title = Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Icon(Icons.attach_file),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            'Lampiran',
                            style: Theme.of(context).textTheme.titleSmall,
                          ),
                          Text(
                            'PDF, Word, Excel, CSV, PNG, atau JPG. Maks. 25 MB.',
                            style: Theme.of(context).textTheme.bodySmall,
                          ),
                        ],
                      ),
                    ),
                  ],
                );
                final onPressed = state.busy || state.validationMessage != null
                    ? null
                    : onPick;

                if (stacked) {
                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      title,
                      const SizedBox(height: 12),
                      Align(
                        alignment: Alignment.centerRight,
                        child: IconButton.filledTonal(
                          tooltip: state.uploadingAttachment
                              ? 'Sedang mengunggah lampiran'
                              : 'Tambah lampiran',
                          onPressed: onPressed,
                          icon: state.uploadingAttachment
                              ? const SizedBox.square(
                                  dimension: 18,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 2,
                                  ),
                                )
                              : const Icon(Icons.upload_file_outlined),
                        ),
                      ),
                    ],
                  );
                }

                final action = FilledButton.tonalIcon(
                  onPressed: onPressed,
                  icon: state.uploadingAttachment
                      ? const SizedBox.square(
                          dimension: 18,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.upload_file_outlined),
                  label: Text(
                    state.uploadingAttachment ? 'Mengunggah' : 'Tambah',
                  ),
                );
                return Row(
                  children: [
                    Expanded(child: title),
                    const SizedBox(width: 12),
                    action,
                  ],
                );
              },
            ),
            if (state.validationMessage != null) ...[
              const SizedBox(height: 8),
              Text(
                'Lengkapi data wajib sebelum menambah lampiran.',
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: scheme.onSurfaceVariant,
                    ),
              ),
            ],
            const SizedBox(height: 16),
            if (state.attachmentsLoading)
              const Center(
                child: Padding(
                  padding: EdgeInsets.all(16),
                  child: CircularProgressIndicator(),
                ),
              )
            else if (state.attachments.isEmpty)
              _EmptyAttachments(color: scheme.outlineVariant)
            else
              ListView.separated(
                shrinkWrap: true,
                physics: const NeverScrollableScrollPhysics(),
                itemCount: state.attachments.length,
                separatorBuilder: (_, __) => const Divider(height: 1),
                itemBuilder: (context, index) {
                  final attachment = state.attachments[index];
                  return _AttachmentRow(
                    attachment: attachment,
                    enabled: !state.busy,
                    onOpen: onOpen,
                    onDelete: onDelete,
                  );
                },
              ),
          ],
        ),
      ),
    );
  }
}

class _EmptyAttachments extends StatelessWidget {
  const _EmptyAttachments({required this.color});

  final Color color;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        border: Border.all(color: color),
        borderRadius: BorderRadius.circular(8),
      ),
      child: const Padding(
        padding: EdgeInsets.symmetric(horizontal: 16, vertical: 20),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.insert_drive_file_outlined),
            SizedBox(width: 8),
            Flexible(child: Text('Belum ada lampiran')),
          ],
        ),
      ),
    );
  }
}

class _AttachmentRow extends StatelessWidget {
  const _AttachmentRow({
    required this.attachment,
    required this.enabled,
    required this.onOpen,
    required this.onDelete,
  });

  final DraftAttachment attachment;
  final bool enabled;
  final ValueChanged<DraftAttachment> onOpen;
  final ValueChanged<DraftAttachment> onDelete;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      contentPadding: EdgeInsets.zero,
      leading: const Icon(Icons.insert_drive_file_outlined),
      title: Text(
        attachment.fileName,
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
      ),
      subtitle: Text(
        '${formatAttachmentBytes(attachment.sizeBytes)} - '
        'scan ${attachment.scanStatus}',
      ),
      trailing: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (attachment.scanStatus == 'clean')
            IconButton(
              tooltip: 'Buka lampiran',
              onPressed: enabled ? () => onOpen(attachment) : null,
              icon: const Icon(Icons.open_in_new),
            ),
          IconButton(
            tooltip: 'Hapus lampiran',
            onPressed: enabled ? () => onDelete(attachment) : null,
            icon: const Icon(Icons.delete_outline),
          ),
        ],
      ),
    );
  }
}

String formatAttachmentBytes(int bytes) {
  if (bytes < 1024) return '$bytes B';
  if (bytes < 1024 * 1024) {
    return '${(bytes / 1024).round()} KB';
  }
  return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
}
