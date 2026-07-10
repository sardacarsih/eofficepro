import 'package:dio/dio.dart';
import 'package:eoffice_mobile/core/network/api_client.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

final effectivenessRepositoryProvider =
    Provider((ref) => EffectivenessRepository(ref.watch(dioProvider)));
final effectivenessProvider =
    FutureProvider.family<EffectivenessSummary, EffectivenessFilter>(
        (ref, filter) =>
            ref.watch(effectivenessRepositoryProvider).summary(filter));

class EffectivenessFilter {
  const EffectivenessFilter({required this.from, required this.to});
  final DateTime from;
  final DateTime to;
  @override
  bool operator ==(Object other) =>
      other is EffectivenessFilter && other.from == from && other.to == to;
  @override
  int get hashCode => Object.hash(from, to);
}

class EffectivenessSummary {
  const EffectivenessSummary(
      {required this.from,
      required this.to,
      required this.activeUsers,
      required this.registeredUsers,
      required this.lettersCreated,
      required this.lettersPublished,
      required this.pendingApprovals,
      required this.overdueApprovals,
      required this.approvalActions,
      required this.readNotifications,
      required this.totalNotifications});
  final String from, to;
  final int activeUsers,
      registeredUsers,
      lettersCreated,
      lettersPublished,
      pendingApprovals,
      overdueApprovals,
      approvalActions,
      readNotifications,
      totalNotifications;
  factory EffectivenessSummary.fromJson(Map<String, dynamic> j) =>
      EffectivenessSummary(
          from: j['from'] as String? ?? '',
          to: j['to'] as String? ?? '',
          activeUsers: j['active_users'] as int? ?? 0,
          registeredUsers: j['registered_users'] as int? ?? 0,
          lettersCreated: j['letters_created'] as int? ?? 0,
          lettersPublished: j['letters_published'] as int? ?? 0,
          pendingApprovals: j['pending_approvals'] as int? ?? 0,
          overdueApprovals: j['overdue_approvals'] as int? ?? 0,
          approvalActions: j['approval_actions'] as int? ?? 0,
          readNotifications: j['read_notifications'] as int? ?? 0,
          totalNotifications: j['total_notifications'] as int? ?? 0);
}

class EffectivenessRepository {
  const EffectivenessRepository(this._dio);
  final Dio _dio;
  Future<EffectivenessSummary> summary(EffectivenessFilter f) async {
    try {
      String fmt(DateTime d) =>
          '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';
      final r = await _dio.get<Map<String, dynamic>>(
          '/management/effectiveness',
          queryParameters: {'from': fmt(f.from), 'to': fmt(f.to)});
      return EffectivenessSummary.fromJson(r.data ?? {});
    } catch (e) {
      throw mapDioException(e, 'Gagal memuat efektivitas aplikasi');
    }
  }
}
