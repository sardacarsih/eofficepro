import 'package:dio/dio.dart';
import 'package:eoffice_mobile/core/constants/api_config.dart';
import 'package:eoffice_mobile/core/exceptions/app_exception.dart';
import 'package:eoffice_mobile/core/services/auth_session_events.dart';
import 'package:eoffice_mobile/core/services/token_storage.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

final dioProvider = Provider<Dio>((ref) {
  final storage = ref.watch(tokenStorageProvider);
  final dio = Dio(
    BaseOptions(
      baseUrl: ApiConfig.baseUrl,
      connectTimeout: ApiConfig.connectTimeout,
      receiveTimeout: ApiConfig.receiveTimeout,
      headers: const {'Content-Type': 'application/json'},
    ),
  );

  final sessionEvents = ref.read(authSessionEventsProvider.notifier);
  dio.interceptors.add(
    AuthInterceptor(
      dio: dio,
      storage: storage,
      onSessionRefreshed: sessionEvents.refreshed,
      onSessionExpired: sessionEvents.expired,
    ),
  );
  return dio;
});

class AuthInterceptor extends Interceptor {
  AuthInterceptor({
    required this.dio,
    required this.storage,
    this.onSessionRefreshed,
    this.onSessionExpired,
    Dio? refreshDio,
  }) : _refreshDio = refreshDio ?? _createRefreshDio();

  final Dio dio;
  final TokenStorage storage;
  final void Function()? onSessionRefreshed;
  final void Function()? onSessionExpired;
  final Dio _refreshDio;
  Future<TokenPair?>? _refreshInFlight;
  final _expiredAuthorizations = <String?>{};

  @override
  void onRequest(
    RequestOptions options,
    RequestInterceptorHandler handler,
  ) async {
    final tokens = await storage.read();
    if (tokens != null && options.headers['Authorization'] == null) {
      options.headers['Authorization'] = 'Bearer ${tokens.accessToken}';
    }
    handler.next(options);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) async {
    final status = err.response?.statusCode;
    final alreadyRetried = err.requestOptions.extra['retried'] == true;
    if (status != 401 || alreadyRetried) {
      handler.next(err);
      return;
    }

    var refreshed = await _refreshToken();
    if (refreshed == null) {
      final requestAuthorization =
          err.requestOptions.headers['Authorization'] as String?;
      if (!_expiredAuthorizations.add(requestAuthorization)) {
        handler.next(err);
        return;
      }
      final current = await storage.read();
      final currentAuthorization =
          current == null ? null : ['Bearer', current.accessToken].join(' ');
      if (current != null && requestAuthorization != currentAuthorization) {
        _expiredAuthorizations.remove(requestAuthorization);
        refreshed = current;
      } else {
        await storage.clear();
        onSessionExpired?.call();
      }
    }
    if (refreshed == null) {
      handler.next(err);
      return;
    }

    final request = err.requestOptions;
    request.extra['retried'] = true;
    request.headers['Authorization'] = 'Bearer ${refreshed.accessToken}';
    try {
      final response = await dio.fetch<dynamic>(request);
      handler.resolve(response);
    } on DioException catch (refreshErr) {
      handler.next(refreshErr);
    }
  }

  Future<TokenPair?> _refreshToken() {
    final inFlight = _refreshInFlight;
    if (inFlight != null) return inFlight;

    final operation = _performRefresh();
    _refreshInFlight = operation;
    operation.whenComplete(() {
      if (identical(_refreshInFlight, operation)) _refreshInFlight = null;
    });
    return operation;
  }

  Future<TokenPair?> _performRefresh() async {
    final tokens = await storage.read();
    if (tokens == null) return null;

    try {
      final response = await _refreshDio.post<Map<String, dynamic>>(
        '/auth/refresh',
        data: {'refresh_token': tokens.refreshToken},
      );
      final data = response.data ?? {};
      final pair = TokenPair(
        accessToken: data['access_token'] as String,
        refreshToken: data['refresh_token'] as String,
      );
      await storage.save(pair);
      _expiredAuthorizations.clear();
      onSessionRefreshed?.call();
      return pair;
    } on Object {
      return null;
    }
  }

  static Dio _createRefreshDio() {
    return Dio(
      BaseOptions(
        baseUrl: ApiConfig.baseUrl,
        connectTimeout: ApiConfig.connectTimeout,
        receiveTimeout: ApiConfig.receiveTimeout,
        headers: const {'Content-Type': 'application/json'},
      ),
    );
  }
}

AppException mapDioException(Object error, String fallback) {
  if (error is! DioException) return AppException(fallback);

  final data = error.response?.data;
  if (data is Map<String, dynamic>) {
    final message = data['error'];
    if (message is String && message.isNotEmpty) {
      return AppException(message, statusCode: error.response?.statusCode);
    }
  }

  if (error.type == DioExceptionType.connectionTimeout ||
      error.type == DioExceptionType.receiveTimeout ||
      error.type == DioExceptionType.connectionError) {
    return const AppException(
      'Tidak dapat terhubung ke server. Periksa koneksi lalu coba lagi.',
    );
  }

  return AppException(fallback, statusCode: error.response?.statusCode);
}
