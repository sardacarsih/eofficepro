class DashboardSummary {
  const DashboardSummary({
    required this.stats,
    required this.incomingTrend,
    required this.pendingApprovals,
    required this.recentActivities,
  });

  final DashboardStats stats;
  final List<DashboardTrendPoint> incomingTrend;
  final List<DashboardPendingApproval> pendingApprovals;
  final List<DashboardActivity> recentActivities;

  factory DashboardSummary.fromJson(Map<String, dynamic> json) {
    return DashboardSummary(
      stats:
          DashboardStats.fromJson(json['stats'] as Map<String, dynamic>? ?? {}),
      incomingTrend: (json['incoming_trend'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(DashboardTrendPoint.fromJson)
          .toList(),
      pendingApprovals:
          (json['pending_approvals'] as List<dynamic>? ?? const [])
              .whereType<Map<String, dynamic>>()
              .map(DashboardPendingApproval.fromJson)
              .toList(),
      recentActivities:
          (json['recent_activities'] as List<dynamic>? ?? const [])
              .whereType<Map<String, dynamic>>()
              .map(DashboardActivity.fromJson)
              .toList(),
    );
  }
}

class DashboardStats {
  const DashboardStats({
    required this.inboxUnread,
    required this.sentThisMonth,
    required this.pendingApprovals,
    required this.archivedTotal,
  });

  final int inboxUnread;
  final int sentThisMonth;
  final int pendingApprovals;
  final int archivedTotal;

  factory DashboardStats.fromJson(Map<String, dynamic> json) {
    return DashboardStats(
      inboxUnread: json['inbox_unread'] as int? ?? 0,
      sentThisMonth: json['sent_this_month'] as int? ?? 0,
      pendingApprovals: json['pending_approvals'] as int? ?? 0,
      archivedTotal: json['archived_total'] as int? ?? 0,
    );
  }
}

class DashboardTrendPoint {
  const DashboardTrendPoint({
    required this.date,
    required this.total,
  });

  final String date;
  final int total;

  factory DashboardTrendPoint.fromJson(Map<String, dynamic> json) {
    return DashboardTrendPoint(
      date: json['date'] as String? ?? '',
      total: json['total'] as int? ?? 0,
    );
  }
}

class DashboardPendingApproval {
  const DashboardPendingApproval({
    required this.stepId,
    required this.letterId,
    required this.subject,
    required this.creatorName,
    required this.updatedAt,
  });

  final String stepId;
  final String letterId;
  final String subject;
  final String creatorName;
  final String updatedAt;

  factory DashboardPendingApproval.fromJson(Map<String, dynamic> json) {
    return DashboardPendingApproval(
      stepId: json['step_id'] as String? ?? '',
      letterId: json['letter_id'] as String? ?? '',
      subject: json['subject'] as String? ?? '',
      creatorName: json['creator_name'] as String? ?? '',
      updatedAt: json['updated_at'] as String? ?? '',
    );
  }
}

class DashboardActivity {
  const DashboardActivity({
    required this.id,
    required this.eventType,
    required this.title,
    required this.createdAt,
  });

  final String id;
  final String eventType;
  final String title;
  final String createdAt;

  factory DashboardActivity.fromJson(Map<String, dynamic> json) {
    return DashboardActivity(
      id: json['id'] as String? ?? '',
      eventType: json['event_type'] as String? ?? '',
      title: json['title'] as String? ?? '',
      createdAt: json['created_at'] as String? ?? '',
    );
  }
}
