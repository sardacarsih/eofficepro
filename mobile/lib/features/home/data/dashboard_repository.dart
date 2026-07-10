import 'package:dio/dio.dart';
import 'package:eoffice_mobile/core/network/api_client.dart';
import 'package:eoffice_mobile/features/home/domain/dashboard_summary.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

final dashboardRepositoryProvider = Provider<DashboardRepository>((ref) {
  return DashboardRepository(ref.watch(dioProvider));
});

final dashboardSummaryProvider = FutureProvider<DashboardSummary>((ref) {
  return ref.watch(dashboardRepositoryProvider).summary();
});

class DashboardRepository {
  const DashboardRepository(this._dio);

  final Dio _dio;

  Future<DashboardSummary> summary() async {
    try {
      final response =
          await _dio.get<Map<String, dynamic>>('/dashboard/summary');
      return DashboardSummary.fromJson(response.data ?? {});
    } catch (error) {
      throw mapDioException(error, 'Gagal memuat dashboard');
    }
  }
}
