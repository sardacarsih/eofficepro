import 'package:eoffice_mobile/core/exceptions/app_exception.dart';
import 'package:eoffice_mobile/features/letters/data/draft_repository.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_composer_state.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

mixin DraftComposerRemoteActions
    on AutoDisposeAsyncNotifier<DraftComposerState> {
  DraftComposerState? get currentComposerState;

  Future<String?> ensureComposerSaved();

  void markComposerSelectionChanged();

  Future<void> uploadAttachment({
    required String filePath,
    required int sizeBytes,
    required String fileName,
    required String mimeType,
  }) async {
    if (sizeBytes <= 0 || sizeBytes > 25 * 1024 * 1024) {
      _setActionError('Ukuran lampiran harus antara 1 byte dan 25 MB.');
      return;
    }
    final draftId = await ensureComposerSaved();
    final current = currentComposerState;
    if (draftId == null || current == null) return;
    state = AsyncData(
      current.copyWith(
        uploadingAttachment: true,
        message: null,
        errorMessage: null,
      ),
    );
    try {
      final repository = ref.read(draftRepositoryProvider);
      await repository.uploadAttachment(
        draftId: draftId,
        filePath: filePath,
        fileName: fileName,
        mimeType: mimeType,
      );
      var attachments = current.attachments;
      String? refreshWarning;
      try {
        attachments = await repository.listAttachments(draftId);
      } catch (_) {
        refreshWarning =
            'Lampiran terunggah, tetapi daftar belum dapat dimuat ulang.';
      }
      final latest = currentComposerState;
      if (latest == null) return;
      state = AsyncData(
        latest.copyWith(
          attachments: attachments,
          uploadingAttachment: false,
          message: 'Lampiran berhasil ditambahkan.',
          errorMessage: refreshWarning,
        ),
      );
    } catch (error) {
      _finishActionWithError(error, uploadingAttachment: false);
    }
  }

  Future<void> deleteAttachment(String attachmentId) async {
    final current = currentComposerState;
    final draftId = current?.form.draftId;
    if (current == null || draftId == null || current.busy) return;
    state = AsyncData(
      current.copyWith(
        uploadingAttachment: true,
        message: null,
        errorMessage: null,
      ),
    );
    try {
      final repository = ref.read(draftRepositoryProvider);
      await repository.deleteAttachment(
        draftId: draftId,
        attachmentId: attachmentId,
      );
      var attachments = current.attachments
          .where((item) => item.id != attachmentId)
          .toList(growable: false);
      String? refreshWarning;
      try {
        attachments = await repository.listAttachments(draftId);
      } catch (_) {
        refreshWarning =
            'Lampiran terhapus, tetapi daftar belum dapat dimuat ulang.';
      }
      final latest = currentComposerState;
      if (latest == null) return;
      state = AsyncData(
        latest.copyWith(
          attachments: attachments,
          uploadingAttachment: false,
          message: 'Lampiran dihapus.',
          errorMessage: refreshWarning,
        ),
      );
    } catch (error) {
      _finishActionWithError(error, uploadingAttachment: false);
    }
  }

  Future<DraftPreviewResult?> preview() async {
    final draftId = await ensureComposerSaved();
    final current = currentComposerState;
    if (draftId == null || current == null) return null;
    state = AsyncData(
      current.copyWith(
        previewing: true,
        message: null,
        errorMessage: null,
      ),
    );
    try {
      final result =
          await ref.read(draftRepositoryProvider).previewDraft(draftId);
      final latest = currentComposerState;
      if (latest != null) {
        state = AsyncData(latest.copyWith(previewing: false));
      }
      return result;
    } catch (error) {
      _finishActionWithError(error, previewing: false);
      return null;
    }
  }

  Future<DraftSubmitResult?> submit() async {
    final draftId = await ensureComposerSaved();
    final current = currentComposerState;
    if (draftId == null || current == null) return null;
    state = AsyncData(
      current.copyWith(
        submitting: true,
        message: null,
        errorMessage: null,
      ),
    );
    final repository = ref.read(draftRepositoryProvider);
    late final DraftSubmitResult result;
    try {
      result = await repository.submitDraft(draftId);
    } catch (error) {
      _finishActionWithError(error, submitting: false);
      return null;
    }

    var drafts = current.drafts
        .where((draft) => draft.id != draftId)
        .toList(growable: false);
    String? refreshWarning;
    try {
      drafts = await repository.listDrafts();
    } catch (_) {
      refreshWarning =
          'Surat sudah diajukan, tetapi daftar draft belum termuat ulang.';
    }
    final latest = currentComposerState;
    if (latest == null) return result;
    markComposerSelectionChanged();
    state = AsyncData(
      latest.copyWith(
        drafts: drafts,
        form: DraftComposerForm.empty(
          bootstrap: latest.bootstrap,
          creatorPositionId: latest.creatorPositionIds.first,
          companyId: latest.companyForCreatorPosition(
            latest.creatorPositionIds.first,
          ),
        ),
        attachments: const [],
        dirty: false,
        editRevision: latest.editRevision + 1,
        saveStatus: DraftComposerSaveStatus.idle,
        submitting: false,
        lastSavedAt: null,
        message: 'Surat berhasil diajukan ke '
            '${result.approvalSteps.length} tahap approval.',
        errorMessage: refreshWarning,
      ),
    );
    return result;
  }

  void _setActionError(String message) {
    final current = currentComposerState;
    if (current == null) return;
    state = AsyncData(current.copyWith(errorMessage: message));
  }

  void _finishActionWithError(
    Object error, {
    bool? uploadingAttachment,
    bool? previewing,
    bool? submitting,
  }) {
    final current = currentComposerState;
    if (current == null) return;
    state = AsyncData(
      current.copyWith(
        uploadingAttachment: uploadingAttachment,
        previewing: previewing,
        submitting: submitting,
        errorMessage: error is AppException ? error.message : error.toString(),
      ),
    );
  }
}
