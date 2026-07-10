import 'package:eoffice_mobile/features/letters/domain/draft_composer_state.dart';
import 'package:flutter/material.dart';

class ComposerLoadingView extends StatelessWidget {
  const ComposerLoadingView({super.key});

  @override
  Widget build(BuildContext context) {
    return const Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          CircularProgressIndicator(),
          SizedBox(height: 12),
          Text('Menyiapkan editor surat...'),
        ],
      ),
    );
  }
}

class ComposerErrorView extends StatelessWidget {
  const ComposerErrorView({
    required this.message,
    required this.onRetry,
    super.key,
  });

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.cloud_off_outlined, size: 44),
            const SizedBox(height: 12),
            Text(message, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            FilledButton.icon(
              onPressed: onRetry,
              icon: const Icon(Icons.refresh),
              label: const Text('Coba lagi'),
            ),
          ],
        ),
      ),
    );
  }
}

class ComposerUnavailable extends StatelessWidget {
  const ComposerUnavailable({
    required this.message,
    required this.onRetry,
    super.key,
  });

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.edit_off_outlined, size: 44),
            const SizedBox(height: 12),
            Text(message, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            OutlinedButton.icon(
              onPressed: onRetry,
              icon: const Icon(Icons.refresh),
              label: const Text('Muat ulang'),
            ),
          ],
        ),
      ),
    );
  }
}

class ComposerFeedbackNotice extends StatelessWidget {
  const ComposerFeedbackNotice({
    required this.message,
    required this.error,
    required this.onDismiss,
    super.key,
  });

  final String message;
  final bool error;
  final VoidCallback onDismiss;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final background =
        error ? scheme.errorContainer : scheme.secondaryContainer;
    final foreground =
        error ? scheme.onErrorContainer : scheme.onSecondaryContainer;
    return Semantics(
      liveRegion: true,
      container: true,
      label: message,
      child: Material(
        color: background,
        borderRadius: BorderRadius.circular(8),
        child: Padding(
          padding: const EdgeInsets.only(left: 16, top: 8, bottom: 8),
          child: Row(
            children: [
              ExcludeSemantics(
                child: Icon(
                  error ? Icons.error_outline : Icons.check_circle_outline,
                  color: foreground,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: ExcludeSemantics(
                  child: Text(message, style: TextStyle(color: foreground)),
                ),
              ),
              IconButton(
                tooltip: 'Tutup pesan',
                onPressed: onDismiss,
                icon: Icon(Icons.close, color: foreground),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

String? missingComposerMasterData(DraftComposerState state) {
  if (state.bootstrap.companies.isEmpty) {
    return 'Master perusahaan belum tersedia.';
  }
  if (state.bootstrap.letterTypes.isEmpty) {
    return 'Master jenis surat belum tersedia.';
  }
  if (state.creatorPositions.isEmpty) {
    return 'Jabatan pembuat tidak ditemukan atau sudah tidak aktif.';
  }
  if (state.bootstrap.positions.isEmpty) {
    return 'Daftar penerima jabatan belum tersedia.';
  }
  return null;
}

String? allowedLetterAttachmentMimeType(String fileName) {
  final dot = fileName.lastIndexOf('.');
  if (dot < 0) return null;
  return switch (fileName.substring(dot + 1).toLowerCase()) {
    'pdf' => 'application/pdf',
    'docx' =>
      'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    'xlsx' =>
      'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    'xls' => 'application/vnd.ms-excel',
    'csv' => 'text/csv',
    'png' => 'image/png',
    'jpg' || 'jpeg' => 'image/jpeg',
    _ => null,
  };
}
