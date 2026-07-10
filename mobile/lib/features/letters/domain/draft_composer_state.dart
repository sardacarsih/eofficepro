import 'dart:convert';

import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:eoffice_mobile/features/letters/domain/letter_body_codec.dart';

part 'draft_composer_policy.dart';

const _unset = Object();

enum DraftComposerSaveStatus { idle, saving, saved, failed }

class DraftComposerForm {
  const DraftComposerForm({
    required this.companyId,
    required this.letterTypeId,
    required this.creatorPositionId,
    required this.subject,
    required this.classification,
    required this.priority,
    required this.bodyPlain,
    required this.recipients,
    this.bodyHtml = '',
    this.draftId,
    this.selectedTemplateId = '',
    this.sourceBodyHtml = '',
    this.sourceBodyEditableHtml = '',
    this.onBehalfOfPositionId,
    this.version = 0,
  });

  factory DraftComposerForm.empty({
    required DraftComposerBootstrap bootstrap,
    required String creatorPositionId,
    String? companyId,
  }) {
    final letterType =
        bootstrap.letterTypes.isEmpty ? null : bootstrap.letterTypes.first;
    return DraftComposerForm(
      companyId: bootstrap.companies.any((company) => company.id == companyId)
          ? companyId!
          : (bootstrap.companies.isEmpty ? '' : bootstrap.companies.first.id),
      letterTypeId: letterType?.id ?? '',
      creatorPositionId: creatorPositionId,
      onBehalfOfPositionId: defaultOnBehalfPositionId(
        creatorPositionId,
        bootstrap.positions,
      ),
      subject: '',
      classification:
          letterType?.defaultClassification ?? LetterClassification.biasa,
      priority: LetterPriority.normal,
      bodyPlain: '',
      recipients: const [],
    );
  }

  factory DraftComposerForm.fromDraft(DraftLetter draft) {
    final bodyPlain = letterHtmlToPlainText(draft.bodyHtml);
    final editableHtml = hasUnsupportedLetterFormatting(draft.bodyHtml)
        ? ''
        : normalizeEditableLetterHtml(draft.bodyHtml);
    return DraftComposerForm(
      draftId: draft.id,
      selectedTemplateId: draft.templateId ?? '',
      companyId: draft.companyId,
      letterTypeId: draft.letterTypeId,
      creatorPositionId: draft.creatorPositionId,
      onBehalfOfPositionId: draft.onBehalfOfPositionId,
      subject: draft.subject,
      classification: draft.classification,
      priority: draft.priority,
      bodyPlain: bodyPlain,
      bodyHtml: editableHtml,
      sourceBodyHtml: draft.bodyHtml,
      sourceBodyEditableHtml: editableHtml,
      recipients: List.unmodifiable(draft.recipients),
      version: draft.version,
    );
  }

  final String? draftId;
  final String companyId;
  final String letterTypeId;
  final String selectedTemplateId;
  final String creatorPositionId;
  final String? onBehalfOfPositionId;
  final String subject;
  final LetterClassification classification;
  final LetterPriority priority;
  final String bodyPlain;

  /// Canonical editable HTML maintained by the body editor. Empty when the
  /// body is locked ([bodyReadOnly]) or when only [bodyPlain] is tracked.
  final String bodyHtml;
  final String sourceBodyHtml;

  /// Canonical form of [sourceBodyHtml]; the unedited-body baseline.
  final String sourceBodyEditableHtml;
  final List<DraftRecipient> recipients;
  final int version;

  bool get isPersistable => validationMessage == null;
  bool get bodyReadOnly => hasUnsupportedLetterFormatting(sourceBodyHtml);

  String? get validationMessage {
    if (companyId.isEmpty) return 'Perusahaan wajib dipilih.';
    if (letterTypeId.isEmpty) return 'Jenis surat wajib dipilih.';
    if (creatorPositionId.isEmpty) return 'Jabatan pembuat wajib dipilih.';
    if (subject.trim().isEmpty) return 'Perihal wajib diisi.';
    if (utf8.encode(subject.trim()).length > 255) {
      return 'Perihal maksimal 255 byte.';
    }
    if (!recipients.any((item) => item.type == DraftRecipientType.to)) {
      return 'Minimal satu penerima To wajib dipilih.';
    }
    final keys = <String>{};
    for (final recipient in recipients) {
      final key = [
        recipient.type.wireValue,
        recipient.targetType.wireValue,
        recipient.targetId,
      ].join('|');
      if (!keys.add(key)) return 'Penerima tidak boleh duplikat.';
    }
    if (bodyPlain.trim().isEmpty) return 'Isi surat wajib diisi.';
    return null;
  }

  DraftLetterPayload toPayload() {
    final validationError = validationMessage;
    if (validationError != null) throw StateError(validationError);
    return DraftLetterPayload(
      companyId: companyId,
      letterTypeId: letterTypeId,
      creatorPositionId: creatorPositionId,
      onBehalfOfPositionId: onBehalfOfPositionId,
      templateId: selectedTemplateId.isEmpty ? null : selectedTemplateId,
      baseVersion: draftId == null ? null : version,
      subject: subject.trim(),
      classification: classification,
      priority: priority,
      bodyHtml: sourceBodyHtml.isNotEmpty && bodyHtml == sourceBodyEditableHtml
          ? sourceBodyHtml
          : (bodyHtml.isNotEmpty ? bodyHtml : plainTextToLetterHtml(bodyPlain)),
      recipients: recipients,
    );
  }

  DraftComposerForm copyWith({
    Object? draftId = _unset,
    String? companyId,
    String? letterTypeId,
    String? selectedTemplateId,
    String? creatorPositionId,
    Object? onBehalfOfPositionId = _unset,
    String? subject,
    LetterClassification? classification,
    LetterPriority? priority,
    String? bodyPlain,
    String? bodyHtml,
    String? sourceBodyHtml,
    String? sourceBodyEditableHtml,
    List<DraftRecipient>? recipients,
    int? version,
  }) {
    return DraftComposerForm(
      draftId: identical(draftId, _unset) ? this.draftId : draftId as String?,
      companyId: companyId ?? this.companyId,
      letterTypeId: letterTypeId ?? this.letterTypeId,
      selectedTemplateId: selectedTemplateId ?? this.selectedTemplateId,
      creatorPositionId: creatorPositionId ?? this.creatorPositionId,
      onBehalfOfPositionId: identical(onBehalfOfPositionId, _unset)
          ? this.onBehalfOfPositionId
          : onBehalfOfPositionId as String?,
      subject: subject ?? this.subject,
      classification: classification ?? this.classification,
      priority: priority ?? this.priority,
      bodyPlain: bodyPlain ?? this.bodyPlain,
      bodyHtml: bodyHtml ?? this.bodyHtml,
      sourceBodyHtml: sourceBodyHtml ?? this.sourceBodyHtml,
      sourceBodyEditableHtml:
          sourceBodyEditableHtml ?? this.sourceBodyEditableHtml,
      recipients: recipients ?? this.recipients,
      version: version ?? this.version,
    );
  }
}

class DraftComposerState {
  const DraftComposerState({
    required this.bootstrap,
    required this.drafts,
    required this.form,
    required this.creatorPositionIds,
    this.creatorCompanyByPosition = const {},
    this.attachments = const [],
    this.dirty = false,
    this.editRevision = 0,
    this.saveStatus = DraftComposerSaveStatus.idle,
    this.attachmentsLoading = false,
    this.uploadingAttachment = false,
    this.previewing = false,
    this.submitting = false,
    this.lastSavedAt,
    this.message,
    this.errorMessage,
  });

  final DraftComposerBootstrap bootstrap;
  final List<DraftLetter> drafts;
  final DraftComposerForm form;
  final List<String> creatorPositionIds;
  final Map<String, String> creatorCompanyByPosition;
  final List<DraftAttachment> attachments;
  final bool dirty;
  final int editRevision;
  final DraftComposerSaveStatus saveStatus;
  final bool attachmentsLoading;
  final bool uploadingAttachment;
  final bool previewing;
  final bool submitting;
  final DateTime? lastSavedAt;
  final String? message;
  final String? errorMessage;

  bool get busy =>
      saveStatus == DraftComposerSaveStatus.saving ||
      attachmentsLoading ||
      uploadingAttachment ||
      previewing ||
      submitting;

  bool get formLocked => attachmentsLoading || previewing || submitting;

  String? get validationMessage =>
      form.validationMessage ??
      referenceValidationMessage ??
      onBehalfPolicyMessage(form, bootstrap) ??
      recipientPolicyMessage(form, bootstrap);

  String? get activeCompanyId => availableCompanies.any(
        (company) => company.id == form.companyId,
      )
          ? form.companyId
          : null;

  String? get activeLetterTypeId => bootstrap.letterTypes.any(
        (type) => type.id == form.letterTypeId,
      )
          ? form.letterTypeId
          : null;

  String? get activeCreatorPositionId => creatorPositions.any(
        (position) => position.id == form.creatorPositionId,
      )
          ? form.creatorPositionId
          : null;

  String? get referenceValidationMessage {
    if (activeCompanyId == null) {
      return 'Perusahaan pada draft sudah tidak aktif. Pilih perusahaan lain.';
    }
    if (activeLetterTypeId == null) {
      return 'Jenis surat pada draft sudah tidak aktif. Pilih jenis lain.';
    }
    if (activeCreatorPositionId == null) {
      return 'Jabatan pembuat sudah tidak aktif. Pilih jabatan lain.';
    }
    for (final recipient in form.recipients) {
      final active = recipient.targetType == DraftRecipientTargetType.position
          ? bootstrap.positions.any(
              (position) => position.id == recipient.targetId,
            )
          : bootstrap.orgUnits.any((unit) => unit.id == recipient.targetId);
      if (!active) {
        return [
          'Penerima',
          recipient.label,
          'sudah tidak aktif. Hapus atau pilih ulang.',
        ].join(' ');
      }
    }
    return null;
  }

  bool get canSave => validationMessage == null && !busy;

  List<DraftLetterTemplate> get matchingTemplates {
    return bootstrap.templates
        .where(
          (template) =>
              template.companyId == form.companyId &&
              template.letterTypeId == form.letterTypeId,
        )
        .toList(growable: false);
  }

  List<DraftPosition> get creatorPositions {
    return creatorPositionsForCompany(form.companyId);
  }

  List<DraftPosition> creatorPositionsForCompany(String companyId) {
    final allowed = creatorPositionIds.toSet();
    return bootstrap.positions
        .where(
          (position) =>
              allowed.contains(position.id) &&
              (creatorCompanyByPosition.isEmpty ||
                  creatorCompanyByPosition[position.id] == companyId),
        )
        .toList(growable: false);
  }

  List<DraftCompany> get availableCompanies {
    if (creatorCompanyByPosition.isEmpty) return bootstrap.companies;
    final accessibleCompanyIds = creatorCompanyByPosition.values.toSet();
    return bootstrap.companies
        .where((company) => accessibleCompanyIds.contains(company.id))
        .toList(growable: false);
  }

  String? companyForCreatorPosition(String positionId) =>
      creatorCompanyByPosition[positionId];

  List<DraftPosition> get availableRecipientPositions {
    final creator = selectedCreatorPosition;
    final directorateId = creator == null
        ? null
        : directorateIdForOrgUnit(
            creator.orgUnitId,
            bootstrap.orgUnits,
          );
    if (creator == null ||
        directorateId == null ||
        isManagerOrAbovePositionType(creator.positionType)) {
      return bootstrap.positions;
    }
    return bootstrap.positions
        .where(
          (position) =>
              directorateIdForOrgUnit(
                position.orgUnitId,
                bootstrap.orgUnits,
              ) ==
              directorateId,
        )
        .toList(growable: false);
  }

  List<DraftOrgUnit> get availableRecipientOrgUnits {
    final creator = selectedCreatorPosition;
    final directorateId = creator == null
        ? null
        : directorateIdForOrgUnit(
            creator.orgUnitId,
            bootstrap.orgUnits,
          );
    if (directorateId == null) return bootstrap.orgUnits;
    return bootstrap.orgUnits
        .where(
          (unit) =>
              directorateIdForOrgUnit(unit.id, bootstrap.orgUnits) ==
              directorateId,
        )
        .toList(growable: false);
  }

  DraftPosition? get selectedCreatorPosition {
    for (final position in bootstrap.positions) {
      if (position.id == form.creatorPositionId) return position;
    }
    return null;
  }

  DraftPosition? get onBehalfPosition {
    for (final position in bootstrap.positions) {
      if (position.id == form.onBehalfOfPositionId) return position;
    }
    return null;
  }

  DraftComposerState copyWith({
    DraftComposerBootstrap? bootstrap,
    List<DraftLetter>? drafts,
    DraftComposerForm? form,
    List<String>? creatorPositionIds,
    Map<String, String>? creatorCompanyByPosition,
    List<DraftAttachment>? attachments,
    bool? dirty,
    int? editRevision,
    DraftComposerSaveStatus? saveStatus,
    bool? attachmentsLoading,
    bool? uploadingAttachment,
    bool? previewing,
    bool? submitting,
    Object? lastSavedAt = _unset,
    Object? message = _unset,
    Object? errorMessage = _unset,
  }) {
    return DraftComposerState(
      bootstrap: bootstrap ?? this.bootstrap,
      drafts: drafts ?? this.drafts,
      form: form ?? this.form,
      creatorPositionIds: creatorPositionIds ?? this.creatorPositionIds,
      creatorCompanyByPosition:
          creatorCompanyByPosition ?? this.creatorCompanyByPosition,
      attachments: attachments ?? this.attachments,
      dirty: dirty ?? this.dirty,
      editRevision: editRevision ?? this.editRevision,
      saveStatus: saveStatus ?? this.saveStatus,
      attachmentsLoading: attachmentsLoading ?? this.attachmentsLoading,
      uploadingAttachment: uploadingAttachment ?? this.uploadingAttachment,
      previewing: previewing ?? this.previewing,
      submitting: submitting ?? this.submitting,
      lastSavedAt: identical(lastSavedAt, _unset)
          ? this.lastSavedAt
          : lastSavedAt as DateTime?,
      message: identical(message, _unset) ? this.message : message as String?,
      errorMessage: identical(errorMessage, _unset)
          ? this.errorMessage
          : errorMessage as String?,
    );
  }
}
