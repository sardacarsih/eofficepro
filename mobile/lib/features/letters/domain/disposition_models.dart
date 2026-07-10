class DispositionInboxItem {
  const DispositionInboxItem({
    required this.recipientId,
    required this.dispositionId,
    required this.letterId,
    required this.letterSubject,
    required this.fromPositionTitle,
    required this.creatorName,
    required this.myPositionTitle,
    required this.instruction,
    required this.status,
    required this.createdAt,
    this.letterNumber,
    this.dueDate,
    this.followupNote,
    this.completedAt,
  });

  final String recipientId;
  final String dispositionId;
  final String letterId;
  final String letterSubject;
  final String? letterNumber;
  final String fromPositionTitle;
  final String creatorName;
  final String myPositionTitle;
  final String instruction;
  final String? dueDate;
  final String status;
  final String? followupNote;
  final String? completedAt;
  final String createdAt;

  factory DispositionInboxItem.fromJson(Map<String, dynamic> json) {
    return DispositionInboxItem(
      recipientId: json['recipient_id'] as String? ?? '',
      dispositionId: json['disposition_id'] as String? ?? '',
      letterId: json['letter_id'] as String? ?? '',
      letterSubject: json['letter_subject'] as String? ?? '',
      letterNumber: json['letter_number'] as String?,
      fromPositionTitle: json['from_position_title'] as String? ?? '',
      creatorName: json['creator_name'] as String? ?? '',
      myPositionTitle: json['my_position_title'] as String? ?? '',
      instruction: json['instruction'] as String? ?? '',
      dueDate: json['due_date'] as String?,
      status: json['status'] as String? ?? 'open',
      followupNote: json['followup_note'] as String?,
      completedAt: json['completed_at'] as String?,
      createdAt: json['created_at'] as String? ?? '',
    );
  }
}

class DispositionItem {
  const DispositionItem({
    required this.id,
    required this.letterId,
    required this.fromPositionId,
    required this.fromPositionTitle,
    required this.creatorName,
    required this.instruction,
    required this.createdAt,
    required this.recipients,
    this.parentDispositionId,
    this.dueDate,
  });

  final String id;
  final String letterId;
  final String? parentDispositionId;
  final String fromPositionId;
  final String fromPositionTitle;
  final String creatorName;
  final String instruction;
  final String? dueDate;
  final String createdAt;
  final List<DispositionRecipient> recipients;

  factory DispositionItem.fromJson(Map<String, dynamic> json) {
    return DispositionItem(
      id: json['id'] as String? ?? '',
      letterId: json['letter_id'] as String? ?? '',
      parentDispositionId: json['parent_disposition_id'] as String?,
      fromPositionId: json['from_position_id'] as String? ?? '',
      fromPositionTitle: json['from_position_title'] as String? ?? '',
      creatorName: json['creator_name'] as String? ?? '',
      instruction: json['instruction'] as String? ?? '',
      dueDate: json['due_date'] as String?,
      createdAt: json['created_at'] as String? ?? '',
      recipients: (json['recipients'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(DispositionRecipient.fromJson)
          .toList(),
    );
  }
}

class DispositionRecipient {
  const DispositionRecipient({
    required this.id,
    required this.positionId,
    required this.positionTitle,
    required this.holderName,
    required this.status,
    this.followupNote,
    this.completedAt,
  });

  final String id;
  final String positionId;
  final String positionTitle;
  final String holderName;
  final String status;
  final String? followupNote;
  final String? completedAt;

  factory DispositionRecipient.fromJson(Map<String, dynamic> json) {
    return DispositionRecipient(
      id: json['id'] as String? ?? '',
      positionId: json['position_id'] as String? ?? '',
      positionTitle: json['position_title'] as String? ?? '',
      holderName: json['holder_name'] as String? ?? '',
      status: json['status'] as String? ?? '',
      followupNote: json['followup_note'] as String?,
      completedAt: json['completed_at'] as String?,
    );
  }
}

class PositionOption {
  const PositionOption({
    required this.id,
    required this.title,
    required this.orgUnitName,
    required this.holderName,
  });

  final String id;
  final String title;
  final String orgUnitName;
  final String holderName;

  String get label {
    if (holderName.isEmpty) return '$title - $orgUnitName';
    return '$title - $orgUnitName ($holderName)';
  }

  factory PositionOption.fromJson(Map<String, dynamic> json) {
    return PositionOption(
      id: json['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      orgUnitName: json['org_unit_name'] as String? ?? '',
      holderName: json['holder_name'] as String? ?? '',
    );
  }
}
