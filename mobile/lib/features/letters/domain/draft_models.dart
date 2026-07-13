part 'draft_submission_models.dart';

enum LetterClassification {
  biasa('biasa'),
  terbatas('terbatas'),
  rahasia('rahasia');

  const LetterClassification(this.wireValue);

  final String wireValue;

  static LetterClassification fromWire(String? value) {
    return values.firstWhere(
      (item) => item.wireValue == value,
      orElse: () => LetterClassification.biasa,
    );
  }
}

enum LetterPriority {
  normal('normal'),
  urgent('urgent');

  const LetterPriority(this.wireValue);

  final String wireValue;

  static LetterPriority fromWire(String? value) {
    return values.firstWhere(
      (item) => item.wireValue == value,
      orElse: () => LetterPriority.normal,
    );
  }
}

enum DraftLetterStatus {
  draft('draft'),
  submitted('submitted'),
  inApproval('in_approval'),
  revision('revision'),
  approved('approved'),
  published('published'),
  cancelled('cancelled'),
  archived('archived');

  const DraftLetterStatus(this.wireValue);

  final String wireValue;

  static DraftLetterStatus fromWire(String? value) {
    return values.firstWhere(
      (item) => item.wireValue == value,
      orElse: () => DraftLetterStatus.draft,
    );
  }
}

enum DraftRecipientType {
  to('to'),
  cc('cc');

  const DraftRecipientType(this.wireValue);

  final String wireValue;

  static DraftRecipientType fromWire(String? value) {
    return values.firstWhere(
      (item) => item.wireValue == value,
      orElse: () => DraftRecipientType.to,
    );
  }
}

enum DraftRecipientTargetType {
  position('position'),
  orgUnit('org_unit');

  const DraftRecipientTargetType(this.wireValue);

  final String wireValue;

  static DraftRecipientTargetType fromWire(String? value) {
    return values.firstWhere(
      (item) => item.wireValue == value,
      orElse: () => DraftRecipientTargetType.position,
    );
  }
}

class DraftComposerBootstrap {
  const DraftComposerBootstrap({
    required this.companies,
    required this.letterTypes,
    required this.templates,
    required this.positions,
    required this.orgUnits,
    this.approvalCategories = const [],
  });

  final List<DraftCompany> companies;
  final List<DraftLetterType> letterTypes;
  final List<DraftLetterTemplate> templates;
  final List<DraftPosition> positions;
  final List<DraftOrgUnit> orgUnits;
  final List<ApprovalCategory> approvalCategories;
}

class ApprovalCategory {
  const ApprovalCategory(
      {required this.id, required this.code, required this.name});
  final String id;
  final String code;
  final String name;
  factory ApprovalCategory.fromJson(Map<String, dynamic> json) =>
      ApprovalCategory(
          id: json['id'] as String? ?? '',
          code: json['code'] as String? ?? '',
          name: json['name'] as String? ?? '');
}

class ApprovalRoutePreview {
  const ApprovalRoutePreview(
      {required this.resolutionMode,
      required this.finalLevel,
      required this.allowedLevels,
      required this.steps,
      this.coordinationScope});
  final String resolutionMode;
  final String finalLevel;
  final String? coordinationScope;
  final List<String> allowedLevels;
  final List<String> steps;
  factory ApprovalRoutePreview.fromJson(Map<String, dynamic> json) =>
      ApprovalRoutePreview(
          resolutionMode: json['resolution_mode'] as String? ?? '',
          finalLevel: json['final_level'] as String? ?? '',
          coordinationScope: json['coordination_scope'] as String?,
          allowedLevels: (json['allowed_levels'] as List<dynamic>? ?? const [])
              .whereType<String>()
              .toList(),
          steps: (json['steps'] as List<dynamic>? ?? const [])
              .whereType<Map<String, dynamic>>()
              .map((item) => item['title'] as String? ?? '')
              .where((item) => item.isNotEmpty)
              .toList());
}

class DraftCompany {
  const DraftCompany({
    required this.id,
    required this.code,
    required this.name,
    required this.isActive,
  });

  final String id;
  final String code;
  final String name;
  final bool isActive;

  factory DraftCompany.fromJson(Map<String, dynamic> json) {
    return DraftCompany(
      id: json['id'] as String? ?? '',
      code: json['code'] as String? ?? '',
      name: json['name'] as String? ?? '',
      isActive: json['is_active'] as bool? ?? true,
    );
  }
}

class DraftLetterType {
  const DraftLetterType({
    required this.id,
    required this.code,
    required this.name,
    required this.defaultClassification,
    required this.defaultSlaHours,
    required this.isActive,
  });

  final String id;
  final String code;
  final String name;
  final LetterClassification defaultClassification;
  final int defaultSlaHours;
  final bool isActive;

  factory DraftLetterType.fromJson(Map<String, dynamic> json) {
    return DraftLetterType(
      id: json['id'] as String? ?? '',
      code: json['code'] as String? ?? '',
      name: json['name'] as String? ?? '',
      defaultClassification: LetterClassification.fromWire(
        json['default_classification'] as String?,
      ),
      defaultSlaHours: _asInt(json['default_sla_hours']),
      isActive: json['is_active'] as bool? ?? true,
    );
  }
}

class DraftLetterTemplate {
  const DraftLetterTemplate({
    required this.id,
    required this.letterTypeId,
    required this.letterTypeCode,
    required this.letterTypeName,
    required this.companyId,
    required this.companyCode,
    required this.companyName,
    required this.version,
    required this.layoutConfig,
    required this.bodySkeleton,
    required this.isActive,
    required this.createdAt,
  });

  final String id;
  final String letterTypeId;
  final String letterTypeCode;
  final String letterTypeName;
  final String companyId;
  final String companyCode;
  final String companyName;
  final int version;
  final Map<String, dynamic> layoutConfig;
  final String bodySkeleton;
  final bool isActive;
  final String createdAt;

  factory DraftLetterTemplate.fromJson(Map<String, dynamic> json) {
    final rawLayout = json['layout_config'];
    return DraftLetterTemplate(
      id: json['id'] as String? ?? '',
      letterTypeId: json['letter_type_id'] as String? ?? '',
      letterTypeCode: json['letter_type_code'] as String? ?? '',
      letterTypeName: json['letter_type_name'] as String? ?? '',
      companyId: json['company_id'] as String? ?? '',
      companyCode: json['company_code'] as String? ?? '',
      companyName: json['company_name'] as String? ?? '',
      version: _asInt(json['version']),
      layoutConfig: rawLayout is Map
          ? Map<String, dynamic>.from(rawLayout)
          : const <String, dynamic>{},
      bodySkeleton: json['body_skeleton'] as String? ?? '',
      isActive: json['is_active'] as bool? ?? true,
      createdAt: json['created_at'] as String? ?? '',
    );
  }
}

class DraftPosition {
  const DraftPosition({
    required this.id,
    required this.title,
    required this.positionType,
    required this.isApprover,
    required this.isActive,
    required this.reportsToTitle,
    required this.orgUnitId,
    required this.orgUnitName,
    required this.orgUnitLevel,
    required this.holderName,
    required this.holderUserId,
    required this.identityLocked,
    this.reportsTo,
  });

  final String id;
  final String title;
  final String positionType;
  final bool isApprover;
  final bool isActive;
  final String? reportsTo;
  final String reportsToTitle;
  final String orgUnitId;
  final String orgUnitName;
  final String orgUnitLevel;
  final String holderName;
  final String holderUserId;
  final bool identityLocked;

  String get label => '$title - $orgUnitName';

  factory DraftPosition.fromJson(Map<String, dynamic> json) {
    return DraftPosition(
      id: json['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      positionType: json['position_type'] as String? ?? '',
      isApprover: json['is_approver'] as bool? ?? false,
      isActive: json['is_active'] as bool? ?? true,
      reportsTo: json['reports_to'] as String?,
      reportsToTitle: json['reports_to_title'] as String? ?? '',
      orgUnitId: json['org_unit_id'] as String? ?? '',
      orgUnitName: json['org_unit_name'] as String? ?? '',
      orgUnitLevel: json['org_unit_level'] as String? ?? '',
      holderName: json['holder_name'] as String? ?? '',
      holderUserId: json['holder_user_id'] as String? ?? '',
      identityLocked: json['identity_locked'] as bool? ?? false,
    );
  }
}

class DraftOrgUnit {
  const DraftOrgUnit({
    required this.id,
    required this.code,
    required this.name,
    required this.unitLevel,
    required this.isActive,
    this.parentId,
    this.region,
  });

  final String id;
  final String? parentId;
  final String code;
  final String name;
  final String unitLevel;
  final String? region;
  final bool isActive;

  String get label => '$name - $code';

  factory DraftOrgUnit.fromJson(Map<String, dynamic> json) {
    return DraftOrgUnit(
      id: json['id'] as String? ?? '',
      parentId: json['parent_id'] as String?,
      code: json['code'] as String? ?? '',
      name: json['name'] as String? ?? '',
      unitLevel: json['unit_level'] as String? ?? '',
      region: json['region'] as String?,
      isActive: json['is_active'] as bool? ?? true,
    );
  }
}

class DraftRecipient {
  const DraftRecipient({
    required this.type,
    required this.targetType,
    required this.targetId,
    this.label = '',
  });

  final DraftRecipientType type;
  final DraftRecipientTargetType targetType;
  final String targetId;
  final String label;

  factory DraftRecipient.fromJson(Map<String, dynamic> json) {
    return DraftRecipient(
      type: DraftRecipientType.fromWire(json['type'] as String?),
      targetType: DraftRecipientTargetType.fromWire(
        json['target_type'] as String?,
      ),
      targetId: json['target_id'] as String? ?? '',
      label: json['label'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'type': type.wireValue,
      'target_type': targetType.wireValue,
      'target_id': targetId,
    };
  }
}

class DraftLetter {
  const DraftLetter({
    required this.id,
    required this.companyId,
    required this.companyCode,
    required this.companyName,
    required this.letterTypeId,
    required this.letterTypeCode,
    required this.letterTypeName,
    required this.subject,
    required this.classification,
    required this.priority,
    required this.status,
    required this.creatorPositionId,
    required this.creatorPositionTitle,
    required this.version,
    required this.bodyHtml,
    required this.bodyPlain,
    required this.recipients,
    required this.createdAt,
    required this.updatedAt,
    this.letterNumber,
    this.onBehalfOfPositionId,
    this.onBehalfOfTitle,
    this.templateId,
    this.approvalCategoryId,
    this.requestedFinalLevel,
    this.resolvedFinalLevel,
    this.coordinationScope,
  });

  final String id;
  final String companyId;
  final String companyCode;
  final String companyName;
  final String letterTypeId;
  final String letterTypeCode;
  final String letterTypeName;
  final String? letterNumber;
  final String subject;
  final LetterClassification classification;
  final LetterPriority priority;
  final DraftLetterStatus status;
  final String creatorPositionId;
  final String creatorPositionTitle;
  final String? onBehalfOfPositionId;
  final String? onBehalfOfTitle;
  final String? templateId;
  final String? approvalCategoryId;
  final String? requestedFinalLevel;
  final String? resolvedFinalLevel;
  final String? coordinationScope;
  final int version;
  final String bodyHtml;
  final String bodyPlain;
  final List<DraftRecipient> recipients;
  final String createdAt;
  final String updatedAt;

  factory DraftLetter.fromJson(Map<String, dynamic> json) {
    return DraftLetter(
      id: json['id'] as String? ?? '',
      companyId: json['company_id'] as String? ?? '',
      companyCode: json['company_code'] as String? ?? '',
      companyName: json['company_name'] as String? ?? '',
      letterTypeId: json['letter_type_id'] as String? ?? '',
      letterTypeCode: json['letter_type_code'] as String? ?? '',
      letterTypeName: json['letter_type_name'] as String? ?? '',
      letterNumber: json['letter_number'] as String?,
      subject: json['subject'] as String? ?? '',
      classification: LetterClassification.fromWire(
        json['classification'] as String?,
      ),
      priority: LetterPriority.fromWire(json['priority'] as String?),
      status: DraftLetterStatus.fromWire(json['status'] as String?),
      creatorPositionId: json['creator_position_id'] as String? ?? '',
      creatorPositionTitle: json['creator_position_title'] as String? ?? '',
      onBehalfOfPositionId: json['on_behalf_of_position_id'] as String?,
      onBehalfOfTitle: json['on_behalf_of_title'] as String?,
      templateId: json['template_id'] as String?,
      approvalCategoryId: json['approval_category_id'] as String?,
      requestedFinalLevel: json['requested_final_level'] as String?,
      resolvedFinalLevel: json['resolved_final_level'] as String?,
      coordinationScope: json['coordination_scope'] as String?,
      version: _asInt(json['version']),
      bodyHtml: json['body_html'] as String? ?? '',
      bodyPlain: json['body_plain'] as String? ?? '',
      recipients: (json['recipients'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(DraftRecipient.fromJson)
          .toList(),
      createdAt: json['created_at'] as String? ?? '',
      updatedAt: json['updated_at'] as String? ?? '',
    );
  }
}

class DraftLetterPayload {
  const DraftLetterPayload({
    required this.companyId,
    required this.letterTypeId,
    required this.creatorPositionId,
    required this.subject,
    required this.priority,
    required this.bodyHtml,
    required this.recipients,
    this.onBehalfOfPositionId,
    this.classification,
    this.templateId,
    this.baseVersion,
    this.approvalCategoryId,
    this.requestedFinalLevel,
  });

  final String companyId;
  final String letterTypeId;
  final String creatorPositionId;
  final String? onBehalfOfPositionId;
  final String subject;
  final LetterClassification? classification;
  final String? templateId;
  final int? baseVersion;
  final String? approvalCategoryId;
  final String? requestedFinalLevel;
  final LetterPriority priority;
  final String bodyHtml;
  final List<DraftRecipient> recipients;

  Map<String, dynamic> toJson() {
    return {
      'company_id': companyId,
      'letter_type_id': letterTypeId,
      'creator_position_id': creatorPositionId,
      'on_behalf_of_position_id': onBehalfOfPositionId,
      if (templateId case final value?) 'template_id': value,
      if (baseVersion != null) 'base_version': baseVersion,
      if (approvalCategoryId != null)
        'approval_category_id': approvalCategoryId,
      if (requestedFinalLevel != null)
        'requested_final_level': requestedFinalLevel,
      'subject': subject,
      if (classification case final value?) 'classification': value.wireValue,
      'priority': priority.wireValue,
      'body_html': bodyHtml,
      'recipients': recipients.map((recipient) => recipient.toJson()).toList(),
    };
  }
}

class DraftSaveResult {
  const DraftSaveResult({required this.id, required this.version});

  final String id;
  final int version;

  factory DraftSaveResult.fromJson(Map<String, dynamic> json) {
    return DraftSaveResult(
      id: json['id'] as String? ?? '',
      version: _asInt(json['version']),
    );
  }
}

int _asInt(Object? value) => value is num ? value.toInt() : 0;
