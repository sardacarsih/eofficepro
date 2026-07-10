class PushNavigationIntent {
  const PushNavigationIntent({
    required this.targetSection,
    this.letterId,
    this.notificationId,
  });

  final String targetSection;
  final String? letterId;
  final String? notificationId;

  String get location {
    final params = <String, String>{'section': targetSection};
    if (letterId != null && letterId!.isNotEmpty) {
      params['letter_id'] = letterId!;
    }
    if (notificationId != null && notificationId!.isNotEmpty) {
      params['notification_id'] = notificationId!;
    }
    return Uri(path: '/app', queryParameters: params).toString();
  }
}

PushNavigationIntent? pushNavigationIntentFromData(Map<String, dynamic> data) {
  final eventType = (data['event_type'] as String? ?? '').trim();
  final targetSection =
      _normalizeTargetSection(data['target_section'] as String?, eventType);
  if (targetSection == null) return null;

  return PushNavigationIntent(
    targetSection: targetSection,
    letterId: (data['letter_id'] as String?)?.trim(),
    notificationId: (data['notification_id'] as String?)?.trim(),
  );
}

String? _normalizeTargetSection(String? rawSection, String eventType) {
  final section = rawSection?.trim();
  if (section == 'approvals' ||
      section == 'inbox' ||
      section == 'dispositions' ||
      section == 'dashboard') {
    return section;
  }

  return switch (eventType) {
    'approval_waiting' => 'approvals',
    'letter_incoming' || 'approval_result' => 'inbox',
    'disposition_assigned' || 'disposition_updated' => 'dispositions',
    'sla_reminder' || 'sla_escalation' => 'approvals',
    _ => null,
  };
}
