class ApprovalInboxItem {
  const ApprovalInboxItem({
    required this.stepId,
    required this.letterId,
    required this.subject,
    required this.priority,
    required this.classification,
    required this.letterTypeCode,
    required this.companyCode,
    required this.positionTitle,
    required this.creatorName,
    required this.creatorPosition,
    required this.bodyPlain,
    required this.attachmentCount,
    required this.updatedAt,
    this.isDelegated = false,
    this.delegatedFromTitle,
  });

  final String stepId;
  final String letterId;
  final String subject;
  final String priority;
  final String classification;
  final String letterTypeCode;
  final String companyCode;
  final String positionTitle;
  final String creatorName;
  final String creatorPosition;
  final String bodyPlain;
  final int attachmentCount;
  final String updatedAt;
  final bool isDelegated;
  final String? delegatedFromTitle;

  factory ApprovalInboxItem.fromJson(Map<String, dynamic> json) {
    return ApprovalInboxItem(
      stepId: json['step_id'] as String? ?? '',
      letterId: json['letter_id'] as String? ?? '',
      subject: json['subject'] as String? ?? '',
      priority: json['priority'] as String? ?? 'normal',
      classification: json['classification'] as String? ?? 'biasa',
      letterTypeCode: json['letter_type_code'] as String? ?? '',
      companyCode: json['company_code'] as String? ?? '',
      positionTitle: json['position_title'] as String? ?? '',
      creatorName: json['creator_name'] as String? ?? '',
      creatorPosition: json['creator_position'] as String? ?? '',
      bodyPlain: json['body_plain'] as String? ?? '',
      attachmentCount: json['attachment_count'] as int? ?? 0,
      updatedAt: json['updated_at'] as String? ?? '',
      isDelegated: json['is_delegated'] as bool? ?? false,
      delegatedFromTitle: json['delegated_from_title'] as String?,
    );
  }
}

class IncomingLetter {
  const IncomingLetter({
    required this.id,
    required this.recipientType,
    required this.subject,
    required this.priority,
    required this.classification,
    required this.letterTypeCode,
    required this.companyCode,
    required this.creatorName,
    required this.creatorPositionTitle,
    required this.bodyPlain,
    required this.attachmentCount,
    required this.isRead,
    required this.updatedAt,
    this.letterNumber,
  });

  final String id;
  final String recipientType;
  final String subject;
  final String priority;
  final String classification;
  final String letterTypeCode;
  final String companyCode;
  final String creatorName;
  final String creatorPositionTitle;
  final String bodyPlain;
  final int attachmentCount;
  final bool isRead;
  final String updatedAt;
  final String? letterNumber;

  factory IncomingLetter.fromJson(Map<String, dynamic> json) {
    return IncomingLetter(
      id: json['id'] as String? ?? '',
      recipientType: json['recipient_type'] as String? ?? '',
      subject: json['subject'] as String? ?? '',
      priority: json['priority'] as String? ?? 'normal',
      classification: json['classification'] as String? ?? 'biasa',
      letterTypeCode: json['letter_type_code'] as String? ?? '',
      companyCode: json['company_code'] as String? ?? '',
      creatorName: json['creator_name'] as String? ?? '',
      creatorPositionTitle: json['creator_position_title'] as String? ?? '',
      bodyPlain: json['body_plain'] as String? ?? '',
      attachmentCount: json['attachment_count'] as int? ?? 0,
      isRead: json['is_read'] as bool? ?? false,
      updatedAt: json['updated_at'] as String? ?? '',
      letterNumber: json['letter_number'] as String?,
    );
  }
}

class LetterDetail {
  const LetterDetail({
    required this.id,
    required this.companyCode,
    required this.companyName,
    required this.letterTypeCode,
    required this.letterTypeName,
    required this.subject,
    required this.classification,
    required this.priority,
    required this.status,
    required this.creatorName,
    required this.creatorPositionTitle,
    required this.version,
    required this.bodyPlain,
    required this.recipients,
    required this.attachments,
    required this.approvalSteps,
    required this.approvalActions,
    required this.createdAt,
    required this.updatedAt,
    this.letterNumber,
    this.onBehalfOfTitle,
    this.verifyUrl,
    this.finalPdfUrl,
    this.publishedAt,
    this.cancelledAt,
    this.cancelledByName,
    this.cancelReason,
    this.canCancel = false,
  });

  final String id;
  final String companyCode;
  final String companyName;
  final String letterTypeCode;
  final String letterTypeName;
  final String? letterNumber;
  final String subject;
  final String classification;
  final String priority;
  final String status;
  final String creatorName;
  final String creatorPositionTitle;
  final String? onBehalfOfTitle;
  final int version;
  final String bodyPlain;
  final String? verifyUrl;
  final String? finalPdfUrl;
  final List<LetterRecipient> recipients;
  final List<LetterAttachment> attachments;
  final List<LetterApprovalStep> approvalSteps;
  final List<LetterApprovalAction> approvalActions;
  final String createdAt;
  final String updatedAt;
  final String? publishedAt;
  final String? cancelledAt;
  final String? cancelledByName;
  final String? cancelReason;
  final bool canCancel;

  factory LetterDetail.fromJson(Map<String, dynamic> json) {
    return LetterDetail(
      id: json['id'] as String? ?? '',
      companyCode: json['company_code'] as String? ?? '',
      companyName: json['company_name'] as String? ?? '',
      letterTypeCode: json['letter_type_code'] as String? ?? '',
      letterTypeName: json['letter_type_name'] as String? ?? '',
      letterNumber: json['letter_number'] as String?,
      subject: json['subject'] as String? ?? '',
      classification: json['classification'] as String? ?? '',
      priority: json['priority'] as String? ?? '',
      status: json['status'] as String? ?? '',
      creatorName: json['creator_name'] as String? ?? '',
      creatorPositionTitle: json['creator_position_title'] as String? ?? '',
      onBehalfOfTitle: json['on_behalf_of_title'] as String?,
      version: json['version'] as int? ?? 0,
      bodyPlain: json['body_plain'] as String? ?? '',
      verifyUrl: json['verify_url'] as String?,
      finalPdfUrl: json['final_pdf_url'] as String?,
      recipients: (json['recipients'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(LetterRecipient.fromJson)
          .toList(),
      attachments: (json['attachments'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(LetterAttachment.fromJson)
          .toList(),
      approvalSteps: (json['approval_steps'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(LetterApprovalStep.fromJson)
          .toList(),
      approvalActions: (json['approval_actions'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(LetterApprovalAction.fromJson)
          .toList(),
      createdAt: json['created_at'] as String? ?? '',
      updatedAt: json['updated_at'] as String? ?? '',
      publishedAt: json['published_at'] as String?,
      cancelledAt: json['cancelled_at'] as String?,
      cancelledByName: json['cancelled_by_name'] as String?,
      cancelReason: json['cancel_reason'] as String?,
      canCancel: json['can_cancel'] as bool? ?? false,
    );
  }
}

class LetterRecipient {
  const LetterRecipient({
    required this.type,
    required this.targetType,
    required this.targetId,
    required this.label,
  });

  final String type;
  final String targetType;
  final String targetId;
  final String label;

  factory LetterRecipient.fromJson(Map<String, dynamic> json) {
    return LetterRecipient(
      type: json['type'] as String? ?? '',
      targetType: json['target_type'] as String? ?? '',
      targetId: json['target_id'] as String? ?? '',
      label: json['label'] as String? ?? '',
    );
  }
}

class LetterAttachment {
  const LetterAttachment({
    required this.id,
    required this.fileName,
    required this.mimeType,
    required this.sizeBytes,
    required this.scanStatus,
    this.downloadUrl,
  });

  final String id;
  final String fileName;
  final String mimeType;
  final int sizeBytes;
  final String scanStatus;
  final String? downloadUrl;

  factory LetterAttachment.fromJson(Map<String, dynamic> json) {
    return LetterAttachment(
      id: json['id'] as String? ?? '',
      fileName: json['file_name'] as String? ?? '',
      mimeType: json['mime_type'] as String? ?? '',
      sizeBytes: json['size_bytes'] as int? ?? 0,
      scanStatus: json['scan_status'] as String? ?? '',
      downloadUrl: json['download_url'] as String?,
    );
  }
}

class LetterApprovalStep {
  const LetterApprovalStep({
    required this.id,
    required this.stepOrder,
    required this.status,
    required this.positionTitle,
    this.slaDeadline,
    this.decidedAt,
  });

  final String id;
  final int stepOrder;
  final String status;
  final String positionTitle;
  final String? slaDeadline;
  final String? decidedAt;

  factory LetterApprovalStep.fromJson(Map<String, dynamic> json) {
    return LetterApprovalStep(
      id: json['id'] as String? ?? '',
      stepOrder: json['step_order'] as int? ?? 0,
      status: json['status'] as String? ?? '',
      positionTitle: json['position_title'] as String? ?? '',
      slaDeadline: json['sla_deadline'] as String?,
      decidedAt: json['decided_at'] as String?,
    );
  }
}

class LetterApprovalAction {
  const LetterApprovalAction({
    required this.id,
    required this.stepId,
    required this.action,
    required this.actorName,
    required this.createdAt,
    required this.positionTitle,
    required this.signaturePresent,
    this.note,
    this.onBehalfOf = false,
    this.onBehalfOfPositionTitle,
  });

  final String id;
  final String stepId;
  final String action;
  final String actorName;
  final String createdAt;
  final String positionTitle;
  final bool signaturePresent;
  final String? note;
  final bool onBehalfOf;
  final String? onBehalfOfPositionTitle;

  factory LetterApprovalAction.fromJson(Map<String, dynamic> json) {
    return LetterApprovalAction(
      id: json['id'] as String? ?? '',
      stepId: json['step_id'] as String? ?? '',
      action: json['action'] as String? ?? '',
      actorName: json['actor_name'] as String? ?? '',
      createdAt: json['created_at'] as String? ?? '',
      positionTitle: json['position_title'] as String? ?? '',
      signaturePresent: json['signature_present'] as bool? ?? false,
      note: json['note'] as String?,
      onBehalfOf: json['on_behalf_of'] as bool? ?? false,
      onBehalfOfPositionTitle: json['on_behalf_of_position_title'] as String?,
    );
  }
}
