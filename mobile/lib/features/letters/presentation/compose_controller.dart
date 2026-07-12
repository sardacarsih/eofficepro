import 'dart:async';

import 'package:eoffice_mobile/core/exceptions/app_exception.dart';
import 'package:eoffice_mobile/features/auth/presentation/auth_controller.dart';
import 'package:eoffice_mobile/features/letters/data/draft_repository.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_composer_state.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:eoffice_mobile/features/letters/domain/letter_body_codec.dart';
import 'package:eoffice_mobile/features/letters/presentation/compose_remote_actions.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

final draftAutosaveDelayProvider = Provider<Duration>(
  (_) => const Duration(seconds: 30),
);

final draftComposerControllerProvider = AutoDisposeAsyncNotifierProvider<
    DraftComposerController, DraftComposerState>(
  DraftComposerController.new,
);

class DraftComposerController
    extends AutoDisposeAsyncNotifier<DraftComposerState>
    with DraftComposerRemoteActions {
  Timer? _autosaveTimer;
  Future<String?>? _saveInFlight;
  var _selectionGeneration = 0;

  DraftComposerState? get _current => state.valueOrNull;

  @override
  DraftComposerState? get currentComposerState => _current;

  @override
  Future<String?> ensureComposerSaved() => _ensureSaved();

  @override
  void markComposerSelectionChanged() => _selectionGeneration++;

  @override
  Future<DraftComposerState> build() async {
    ref.onDispose(() => _autosaveTimer?.cancel());
    final user = ref.watch(authControllerProvider).valueOrNull?.user;
    if (user == null) throw const AppException('Sesi pengguna tidak tersedia.');
    if (user.positions.isEmpty) {
      throw const AppException(
        'Akun belum ditempatkan ke jabatan aktif.',
      );
    }

    final repository = ref.watch(draftRepositoryProvider);
    final results = await Future.wait<Object>([
      repository.loadBootstrap(),
      repository.listDrafts(),
    ]);
    final bootstrap = results[0] as DraftComposerBootstrap;
    final drafts = results[1] as List<DraftLetter>;
    final creatorPositionIds = user.positions
        .map((position) => position.positionId)
        .where((id) => id.isNotEmpty)
        .toList(growable: false);
    if (creatorPositionIds.isEmpty) {
      throw const AppException(
        'Akun belum ditempatkan ke jabatan aktif.',
      );
    }
    final creatorCompanyByPosition = <String, String>{
      for (final position in user.positions)
        if (position.positionId.isNotEmpty && position.companyId.isNotEmpty)
          position.positionId: position.companyId,
    };
    final initialPositionId = creatorPositionIds.first;

    return DraftComposerState(
      bootstrap: bootstrap,
      drafts: drafts,
      creatorPositionIds: creatorPositionIds,
      creatorCompanyByPosition: creatorCompanyByPosition,
      form: DraftComposerForm.empty(
        bootstrap: bootstrap,
        creatorPositionId: initialPositionId,
        companyId: creatorCompanyByPosition[initialPositionId],
      ),
    );
  }

  void retry() => ref.invalidateSelf();

  void updateForm(DraftComposerForm nextForm) {
    final current = _current;
    if (current == null || current.formLocked) return;
    final next = current.copyWith(
      form: nextForm,
      dirty: true,
      editRevision: current.editRevision + 1,
      saveStatus: current.saveStatus == DraftComposerSaveStatus.saving
          ? DraftComposerSaveStatus.saving
          : DraftComposerSaveStatus.idle,
      message: null,
      errorMessage: null,
      approvalRoute: null,
    );
    state = AsyncData(next);
    _scheduleAutosave(next);
  }

  void editForm(
    DraftComposerForm Function(DraftComposerForm current) update,
  ) {
    final current = _current;
    if (current == null) return;
    updateForm(update(current.form));
  }

  Future<void> previewApprovalRoute() async {
    final current = _current;
    if (current == null) return;
    final validation = current.validationMessage;
    if (validation != null) {
      state = AsyncData(current.copyWith(errorMessage: validation));
      return;
    }
    try {
      final preview = await ref
          .read(draftRepositoryProvider)
          .previewApprovalRoute(payload: current.form.toPayload());
      if (_current == null) return;
      var form = _current!.form;
      if (form.requestedFinalLevel.isEmpty && preview.finalLevel.isNotEmpty) {
        form = form.copyWith(requestedFinalLevel: preview.finalLevel);
      }
      state = AsyncData(_current!
          .copyWith(form: form, approvalRoute: preview, errorMessage: null));
    } catch (error) {
      if (_current == null) return;
      state = AsyncData(_current!
          .copyWith(approvalRoute: null, errorMessage: _messageFor(error)));
    }
  }

  void selectCompany(String companyId) {
    final current = _current;
    if (current == null) return;
    final creatorPositions = current.creatorPositionsForCompany(companyId);
    if (creatorPositions.isEmpty) return;
    final creatorPositionId = creatorPositions.first.id;
    updateForm(
      current.form.copyWith(
        companyId: companyId,
        creatorPositionId: creatorPositionId,
        onBehalfOfPositionId: defaultOnBehalfPositionId(
          creatorPositionId,
          current.bootstrap.positions,
        ),
        selectedTemplateId: '',
        recipients: const [],
      ),
    );
  }

  void selectLetterType(String letterTypeId) {
    final current = _current;
    if (current == null) return;
    DraftLetterType? selected;
    for (final item in current.bootstrap.letterTypes) {
      if (item.id == letterTypeId) {
        selected = item;
        break;
      }
    }
    updateForm(
      current.form.copyWith(
        letterTypeId: letterTypeId,
        selectedTemplateId: '',
        classification:
            selected?.defaultClassification ?? LetterClassification.biasa,
      ),
    );
  }

  void selectCreatorPosition(String positionId) {
    final current = _current;
    if (current == null) return;
    final companyId =
        current.companyForCreatorPosition(positionId) ?? current.form.companyId;
    updateForm(
      current.form.copyWith(
        companyId: companyId,
        creatorPositionId: positionId,
        onBehalfOfPositionId: defaultOnBehalfPositionId(
          positionId,
          current.bootstrap.positions,
        ),
        selectedTemplateId: '',
        recipients: const [],
      ),
    );
  }

  void applyTemplate(String templateId) {
    final current = _current;
    if (current == null) return;
    DraftLetterTemplate? selected;
    for (final item in current.matchingTemplates) {
      if (item.id == templateId) {
        selected = item;
        break;
      }
    }
    if (selected == null) {
      updateForm(current.form.copyWith(selectedTemplateId: templateId));
      return;
    }
    final skeleton = selected.bodySkeleton;
    final editableHtml = hasUnsupportedLetterFormatting(skeleton)
        ? ''
        : normalizeEditableLetterHtml(skeleton);
    updateForm(
      current.form.copyWith(
        selectedTemplateId: templateId,
        bodyPlain: letterHtmlToPlainText(skeleton),
        bodyHtml: editableHtml,
        sourceBodyHtml: skeleton,
        sourceBodyEditableHtml: editableHtml,
      ),
    );
  }

  void simplifyBodyFormatting() {
    final current = _current;
    if (current == null || !current.form.bodyReadOnly) return;
    updateForm(
      current.form.copyWith(
        bodyHtml: plainTextToLetterHtml(current.form.bodyPlain),
        sourceBodyHtml: '',
        sourceBodyEditableHtml: '',
      ),
    );
  }

  void addRecipient(DraftRecipient recipient) {
    final current = _current;
    if (current == null) return;
    final duplicate = current.form.recipients.any(
      (item) =>
          item.type == recipient.type &&
          item.targetType == recipient.targetType &&
          item.targetId == recipient.targetId,
    );
    if (duplicate) {
      state = AsyncData(
        current.copyWith(errorMessage: 'Penerima tersebut sudah ditambahkan.'),
      );
      return;
    }
    updateForm(
      current.form.copyWith(
        recipients: List.unmodifiable([
          ...current.form.recipients,
          recipient,
        ]),
      ),
    );
  }

  void removeRecipient(DraftRecipient recipient) {
    final current = _current;
    if (current == null) return;
    updateForm(
      current.form.copyWith(
        recipients: current.form.recipients
            .where(
              (item) =>
                  item.type != recipient.type ||
                  item.targetType != recipient.targetType ||
                  item.targetId != recipient.targetId,
            )
            .toList(growable: false),
      ),
    );
  }

  void newDraft() {
    final current = _current;
    if (current == null || current.busy) return;
    _selectionGeneration++;
    _autosaveTimer?.cancel();
    _autosaveTimer = null;
    state = AsyncData(
      current.copyWith(
        form: DraftComposerForm.empty(
          bootstrap: current.bootstrap,
          creatorPositionId: current.creatorPositionIds.first,
        ),
        attachments: const [],
        attachmentsLoading: false,
        dirty: false,
        editRevision: current.editRevision + 1,
        saveStatus: DraftComposerSaveStatus.idle,
        lastSavedAt: null,
        message: 'Draft baru siap diisi.',
        errorMessage: null,
      ),
    );
  }

  Future<void> openDraft(DraftLetter selected) async {
    final current = _current;
    if (current == null || current.busy) return;
    final generation = ++_selectionGeneration;
    _autosaveTimer?.cancel();
    _autosaveTimer = null;
    state = AsyncData(
      current.copyWith(
        attachmentsLoading: true,
        message: null,
        errorMessage: null,
      ),
    );

    try {
      final repository = ref.read(draftRepositoryProvider);
      final results = await Future.wait<Object>([
        repository.getDraft(selected.id),
        repository.listAttachments(selected.id),
      ]);
      if (generation != _selectionGeneration || _current == null) return;
      final draft = results[0] as DraftLetter;
      final attachments = results[1] as List<DraftAttachment>;
      state = AsyncData(
        _current!.copyWith(
          form: DraftComposerForm.fromDraft(draft),
          attachments: attachments,
          dirty: false,
          editRevision: _current!.editRevision + 1,
          saveStatus: DraftComposerSaveStatus.idle,
          attachmentsLoading: false,
          lastSavedAt: DateTime.tryParse(draft.updatedAt)?.toLocal(),
          message: null,
          errorMessage: null,
        ),
      );
    } catch (error) {
      if (generation != _selectionGeneration || _current == null) return;
      state = AsyncData(
        _current!.copyWith(
          attachmentsLoading: false,
          errorMessage: _messageFor(error),
        ),
      );
    }
  }

  Future<String?> saveDraft({bool automatic = false}) async {
    final inFlight = _saveInFlight;
    if (inFlight != null) {
      await inFlight;
      return _current?.form.draftId;
    }
    final operation = _performSave(automatic: automatic);
    _saveInFlight = operation;
    try {
      return await operation;
    } finally {
      _saveInFlight = null;
    }
  }

  Future<String?> _performSave({required bool automatic}) async {
    final before = _current;
    if (before == null) return null;
    final validationError = before.validationMessage;
    if (validationError != null) {
      state = AsyncData(
        before.copyWith(
          saveStatus: DraftComposerSaveStatus.failed,
          errorMessage: automatic ? null : validationError,
        ),
      );
      return null;
    }
    final revision = before.editRevision;
    final selectedDraftId = before.form.draftId;
    _autosaveTimer?.cancel();
    _autosaveTimer = null;
    state = AsyncData(
      before.copyWith(
        saveStatus: DraftComposerSaveStatus.saving,
        message: null,
        errorMessage: null,
      ),
    );

    try {
      final repository = ref.read(draftRepositoryProvider);
      final result = await repository.saveDraft(
        draftId: selectedDraftId,
        payload: before.form.toPayload(),
      );
      List<DraftLetter>? refreshedDrafts;
      String? refreshWarning;
      try {
        refreshedDrafts = await repository.listDrafts();
      } catch (_) {
        refreshWarning =
            'Draft tersimpan, tetapi daftar draft belum dapat dimuat ulang.';
      }
      final latest = _current;
      if (latest == null) return result.id;
      final editedDuringSave = latest.editRevision != revision;
      final updated = latest.copyWith(
        drafts: refreshedDrafts ?? latest.drafts,
        form: latest.form.copyWith(
          draftId: result.id,
          version: result.version,
        ),
        dirty: editedDuringSave,
        saveStatus: DraftComposerSaveStatus.saved,
        lastSavedAt: DateTime.now(),
        message: automatic ? null : 'Draft berhasil disimpan.',
        errorMessage: refreshWarning,
      );
      state = AsyncData(updated);
      if (editedDuringSave) _scheduleAutosave(updated);
      return result.id;
    } catch (error) {
      final current = _current;
      if (error is AppException &&
          error.statusCode == 409 &&
          selectedDraftId != null) {
        await _reloadConflictedDraft(selectedDraftId, current);
        return null;
      }
      if (current != null) {
        final failed = current.copyWith(
          dirty: true,
          saveStatus: DraftComposerSaveStatus.failed,
          errorMessage: automatic
              ? 'Autosave gagal. Perubahan tetap ada di layar.'
              : _messageFor(error),
        );
        state = AsyncData(failed);
        if (automatic) _scheduleAutosave(failed);
      }
      return null;
    }
  }

  Future<void> _reloadConflictedDraft(
    String draftId,
    DraftComposerState? current,
  ) async {
    if (current == null) return;
    try {
      final repository = ref.read(draftRepositoryProvider);
      final results = await Future.wait<Object>([
        repository.getDraft(draftId),
        repository.listAttachments(draftId),
      ]);
      final draft = results[0] as DraftLetter;
      final attachments = results[1] as List<DraftAttachment>;
      state = AsyncData(
        current.copyWith(
          form: DraftComposerForm.fromDraft(draft),
          attachments: attachments,
          dirty: false,
          saveStatus: DraftComposerSaveStatus.failed,
          errorMessage:
              'Draft diperbarui dari versi terbaru karena ada perubahan di perangkat lain.',
        ),
      );
    } catch (_) {
      state = AsyncData(
        current.copyWith(
          dirty: true,
          saveStatus: DraftComposerSaveStatus.failed,
          errorMessage:
              'Draft berubah di perangkat lain. Muat ulang sebelum menyimpan kembali.',
        ),
      );
    }
  }

  void clearFeedback() {
    final current = _current;
    if (current == null) return;
    state = AsyncData(
      current.copyWith(message: null, errorMessage: null),
    );
  }

  Future<String?> _ensureSaved() async {
    for (var attempt = 0; attempt < 3; attempt++) {
      final current = _current;
      if (current == null) return null;
      if (current.form.draftId != null && !current.dirty) {
        return current.form.draftId;
      }
      final draftId = await saveDraft();
      if (draftId == null) return null;
    }
    final latest = _current;
    if (latest?.form.draftId != null && latest?.dirty == false) {
      return latest!.form.draftId;
    }
    return null;
  }

  void _scheduleAutosave(DraftComposerState value) {
    if (!value.dirty || value.validationMessage != null) return;
    if (_autosaveTimer?.isActive == true) return;
    _autosaveTimer = Timer(
      ref.read(draftAutosaveDelayProvider),
      () {
        _autosaveTimer = null;
        unawaited(saveDraft(automatic: true));
      },
    );
  }
}

String _messageFor(Object error) {
  return error is AppException ? error.message : error.toString();
}
