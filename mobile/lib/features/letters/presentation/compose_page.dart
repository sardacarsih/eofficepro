import 'package:eoffice_mobile/core/exceptions/app_exception.dart';
import 'package:eoffice_mobile/core/network/api_client.dart';
import 'package:eoffice_mobile/core/services/authenticated_file_opener.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_composer_state.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:eoffice_mobile/features/letters/presentation/compose_controller.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/attachment_section.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/compose_action_bar.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/compose_form_fields.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/compose_support.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/draft_list_panel.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

class ComposePage extends ConsumerStatefulWidget {
  const ComposePage({super.key});

  @override
  ConsumerState<ComposePage> createState() => _ComposePageState();
}

class _ComposePageState extends ConsumerState<ComposePage> {
  final _subjectController = TextEditingController();
  var _allowPop = false;

  @override
  void dispose() {
    _subjectController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final asyncState = ref.watch(draftComposerControllerProvider);
    final composer = asyncState.valueOrNull;
    final size = MediaQuery.sizeOf(context);
    final tablet = size.width >= 900 && size.width > size.height;
    final blockPop =
        (composer?.dirty == true || composer?.busy == true) && !_allowPop;

    if (composer != null) _scheduleTextControllerSync(composer.form);

    return PopScope(
      canPop: !blockPop,
      onPopInvokedWithResult: (didPop, result) {
        if (!didPop) _confirmPageExit();
      },
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Tulis Surat'),
          actions: [
            if (composer != null && !tablet)
              IconButton(
                tooltip: 'Draft saya',
                onPressed:
                    composer.busy ? null : () => _showDraftSheet(composer),
                icon: const Icon(Icons.folder_open_outlined),
              ),
            if (composer != null)
              IconButton(
                tooltip: 'Draft baru',
                onPressed:
                    composer.busy ? null : () => _startNewDraft(composer),
                icon: const Icon(Icons.note_add_outlined),
              ),
          ],
        ),
        body: asyncState.when(
          loading: () => const ComposerLoadingView(),
          error: (error, stackTrace) => ComposerErrorView(
            message: error is AppException
                ? error.message
                : 'Gagal memuat fitur Tulis Surat.',
            onRetry: () =>
                ref.read(draftComposerControllerProvider.notifier).retry(),
          ),
          data: (state) => _buildReadyBody(state, tablet: tablet),
        ),
        bottomNavigationBar:
            composer == null || missingComposerMasterData(composer) != null
                ? null
                : ComposeActionBar(
                    state: composer,
                    onSave: _save,
                    onPreview: _preview,
                    onSubmit: () => _submit(composer),
                  ),
      ),
    );
  }

  Widget _buildReadyBody(
    DraftComposerState state, {
    required bool tablet,
  }) {
    final missing = missingComposerMasterData(state);
    if (missing != null) {
      return ComposerUnavailable(
        message: missing,
        onRetry: () =>
            ref.read(draftComposerControllerProvider.notifier).retry(),
      );
    }

    final editor = _buildEditor(state);
    if (!tablet) return editor;
    return Row(
      children: [
        SizedBox(
          width: 340,
          child: DraftListPanel(
            state: state,
            onNewDraft: () => _startNewDraft(state),
            onOpenDraft: (draft) => _openDraft(state, draft),
          ),
        ),
        const VerticalDivider(width: 1),
        Expanded(child: editor),
      ],
    );
  }

  Widget _buildEditor(DraftComposerState state) {
    final theme = Theme.of(context);
    return Scrollbar(
      child: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(20, 20, 20, 32),
        keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
        child: Align(
          alignment: Alignment.topCenter,
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 1040),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  state.form.draftId == null
                      ? 'Draft Baru'
                      : 'Draft v${state.form.version}',
                  style: theme.textTheme.titleLarge,
                ),
                const SizedBox(height: 4),
                Text(
                  state.form.draftId == null
                      ? 'Nomor surat diterbitkan setelah approval final.'
                      : state.dirty
                          ? 'Ada perubahan yang belum tersimpan.'
                          : 'Perubahan terakhir sudah tersimpan.',
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
                if (state.message != null) ...[
                  const SizedBox(height: 16),
                  ComposerFeedbackNotice(
                    message: state.message!,
                    error: false,
                    onDismiss: _clearFeedback,
                  ),
                ],
                if (state.errorMessage != null) ...[
                  const SizedBox(height: 16),
                  ComposerFeedbackNotice(
                    message: state.errorMessage!,
                    error: true,
                    onDismiss: _clearFeedback,
                  ),
                ],
                if (state.attachmentsLoading) ...[
                  const SizedBox(height: 16),
                  const LinearProgressIndicator(),
                  const SizedBox(height: 8),
                  Semantics(
                    liveRegion: true,
                    child: const Text('Membuka draft...'),
                  ),
                ],
                const SizedBox(height: 20),
                ComposeFormFields(
                  state: state,
                  controller:
                      ref.read(draftComposerControllerProvider.notifier),
                  subjectController: _subjectController,
                ),
                const SizedBox(height: 24),
                AttachmentSection(
                  state: state,
                  onPick: _pickAttachment,
                  onOpen: _openAttachment,
                  onDelete: (attachment) =>
                      _deleteAttachment(state, attachment),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _scheduleTextControllerSync(DraftComposerForm form) {
    final subject = form.subject;
    if (_subjectController.text == subject) return;

    // Assigning a TextEditingController during build notifies TextFormField,
    // which makes Form call setState while LayoutBuilder is still building.
    // Resetting the composer after submit therefore needs to happen after the
    // current frame has completed.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final latest = ref.read(draftComposerControllerProvider).valueOrNull;
      if (latest?.form.subject != subject) return;
      _setControllerText(_subjectController, subject);
    });
  }

  void _setControllerText(
    TextEditingController controller,
    String value,
  ) {
    if (controller.text == value) return;
    controller.value = TextEditingValue(
      text: value,
      selection: TextSelection.collapsed(offset: value.length),
    );
  }

  Future<void> _confirmPageExit() async {
    final state = ref.read(draftComposerControllerProvider).valueOrNull;
    if (state?.busy == true) {
      _showMessage('Tunggu proses saat ini selesai.');
      return;
    }
    if (state == null || !state.dirty) {
      if (mounted) context.pop();
      return;
    }
    if (!await _confirmDiscard(state)) return;
    if (!mounted) return;
    setState(() => _allowPop = true);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) context.pop();
    });
  }

  Future<bool> _confirmDiscard(DraftComposerState state) async {
    if (!state.dirty) return true;
    if (state.busy) {
      _showMessage('Tunggu proses saat ini selesai.');
      return false;
    }
    final discard = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Buang perubahan?'),
        content: const Text(
          'Perubahan sejak penyimpanan terakhir akan hilang.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Tetap di sini'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Buang'),
          ),
        ],
      ),
    );
    return discard ?? false;
  }

  Future<void> _startNewDraft(DraftComposerState state) async {
    if (!await _confirmDiscard(state) || !mounted) return;
    ref.read(draftComposerControllerProvider.notifier).newDraft();
  }

  Future<void> _openDraft(
    DraftComposerState state,
    DraftLetter draft,
  ) async {
    if (state.form.draftId == draft.id) return;
    if (!await _confirmDiscard(state) || !mounted) return;
    await ref.read(draftComposerControllerProvider.notifier).openDraft(draft);
    if (mounted &&
        ref.read(draftComposerControllerProvider).valueOrNull?.errorMessage !=
            null) {
      _showCurrentError();
    }
  }

  Future<void> _showDraftSheet(DraftComposerState state) async {
    await showModalBottomSheet<void>(
      context: context,
      useSafeArea: true,
      showDragHandle: true,
      isScrollControlled: true,
      builder: (sheetContext) => SizedBox(
        height: MediaQuery.sizeOf(sheetContext).height * 0.72,
        child: DraftListPanel(
          state: state,
          compact: true,
          onNewDraft: () {
            Navigator.pop(sheetContext);
            _startNewDraft(state);
          },
          onOpenDraft: (draft) {
            Navigator.pop(sheetContext);
            _openDraft(state, draft);
          },
        ),
      ),
    );
  }

  Future<void> _save() async {
    FocusManager.instance.primaryFocus?.unfocus();
    final result =
        await ref.read(draftComposerControllerProvider.notifier).saveDraft();
    if (result == null && mounted) _showCurrentError();
  }

  Future<void> _preview() async {
    FocusManager.instance.primaryFocus?.unfocus();
    final controller = ref.read(draftComposerControllerProvider.notifier);
    final result = await controller.preview();
    if (result == null || !mounted) {
      if (mounted) _showCurrentError();
      return;
    }
    await _openUrl(result.previewUrl, fallback: 'Preview tidak dapat dibuka.');
  }

  Future<void> _submit(DraftComposerState state) async {
    FocusManager.instance.primaryFocus?.unfocus();
    if (state.validationMessage != null) {
      await ref.read(draftComposerControllerProvider.notifier).saveDraft();
      if (mounted) _showCurrentError();
      return;
    }
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Ajukan surat?'),
        content: Text(
          'Surat "${state.form.subject.trim()}" akan dikirim ke alur '
          'approval dan tidak dapat diedit.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Batal'),
          ),
          FilledButton.icon(
            onPressed: () => Navigator.pop(context, true),
            icon: const Icon(Icons.send_outlined),
            label: const Text('Ajukan'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    await ref.read(draftComposerControllerProvider.notifier).submit();
    if (mounted &&
        ref.read(draftComposerControllerProvider).valueOrNull?.errorMessage !=
            null) {
      _showCurrentError();
    }
  }

  Future<void> _pickAttachment() async {
    try {
      final result = await FilePicker.pickFiles(
        type: FileType.custom,
        allowedExtensions: const [
          'pdf',
          'docx',
          'xlsx',
          'xls',
          'csv',
          'png',
          'jpg',
          'jpeg',
        ],
        allowMultiple: false,
        withData: false,
        withReadStream: false,
      );
      if (result == null || result.files.isEmpty) return;
      final file = result.files.single;
      final mimeType = allowedLetterAttachmentMimeType(file.name);
      if (mimeType == null) {
        _showMessage('Tipe file tidak didukung.');
        return;
      }
      if (file.size <= 0 || file.size > 25 * 1024 * 1024) {
        _showMessage('Ukuran lampiran maksimal 25 MB.');
        return;
      }
      final path = file.path;
      if (path == null || path.isEmpty) {
        _showMessage('File belum tersedia secara lokal.');
        return;
      }
      if (!mounted) return;
      await ref.read(draftComposerControllerProvider.notifier).uploadAttachment(
            filePath: path,
            sizeBytes: file.size,
            fileName: file.name,
            mimeType: mimeType,
          );
      if (mounted &&
          ref.read(draftComposerControllerProvider).valueOrNull?.errorMessage !=
              null) {
        _showCurrentError();
      }
    } catch (error) {
      if (mounted) _showMessage('File tidak dapat dibaca.');
    }
  }

  Future<void> _openAttachment(DraftAttachment attachment) async {
    final draftId =
        ref.read(draftComposerControllerProvider).valueOrNull?.form.draftId;
    if (draftId == null || attachment.scanStatus != 'clean') {
      _showMessage(
          'Lampiran belum tersedia karena belum lolos pemindaian keamanan.');
      return;
    }
    try {
      await AuthenticatedFileOpener(ref.read(dioProvider)).open(
        '/letters/drafts/$draftId/attachments/${attachment.id}/download',
        attachment.fileName,
      );
    } on AppException catch (error) {
      if (mounted) _showMessage(error.message);
    }
  }

  Future<void> _deleteAttachment(
    DraftComposerState state,
    DraftAttachment attachment,
  ) async {
    if (state.busy) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Hapus lampiran?'),
        content: Text(attachment.fileName),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Hapus'),
          ),
        ],
      ),
    );
    if (confirmed == true && mounted) {
      await ref
          .read(draftComposerControllerProvider.notifier)
          .deleteAttachment(attachment.id);
      if (mounted &&
          ref.read(draftComposerControllerProvider).valueOrNull?.errorMessage !=
              null) {
        _showCurrentError();
      }
    }
  }

  Future<void> _openUrl(String value, {required String fallback}) async {
    final uri = Uri.tryParse(value);
    if (uri == null ||
        !await launchUrl(uri, mode: LaunchMode.externalApplication)) {
      if (mounted) _showMessage(fallback);
    }
  }

  void _clearFeedback() {
    ref.read(draftComposerControllerProvider.notifier).clearFeedback();
  }

  void _showMessage(String message) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(message)));
  }

  void _showCurrentError() {
    final state = ref.read(draftComposerControllerProvider).valueOrNull;
    _showMessage(
      state?.errorMessage ??
          state?.validationMessage ??
          'Data surat belum lengkap.',
    );
  }
}
