import 'package:dio/dio.dart';
import 'package:eoffice_mobile/core/exceptions/app_exception.dart';
import 'package:eoffice_mobile/core/network/api_client.dart';
import 'package:eoffice_mobile/core/services/token_storage.dart';
import 'package:eoffice_mobile/features/auth/domain/user.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

final authRepositoryProvider = Provider<AuthRepository>((ref) {
  return AuthRepository(
    dio: ref.watch(dioProvider),
    storage: ref.watch(tokenStorageProvider),
  );
});

class AuthRepository {
  const AuthRepository({required Dio dio, required TokenStorage storage})
      : _dio = dio,
        _storage = storage;

  final Dio _dio;
  final TokenStorage _storage;

  Future<User> login({
    required String identifier,
    required String password,
    bool rememberMe = false,
  }) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/auth/login',
        data: {
          'identifier': identifier,
          'password': password,
          'remember_me': rememberMe,
        },
      );
      final data = response.data ?? {};
      await _storage.save(
        TokenPair(
          accessToken: data['access_token'] as String,
          refreshToken: data['refresh_token'] as String,
        ),
      );
      final profile = await _dio.get<Map<String, dynamic>>('/auth/me');
      return User.fromJson(profile.data ?? {});
    } catch (error) {
      await _storage.clear();
      throw mapDioException(error, 'Login gagal');
    }
  }

  Future<User?> restoreSession() async {
    final tokens = await _storage.read();
    if (tokens == null) return null;
    try {
      return await me();
    } on AppException catch (error) {
      if (error.statusCode == 401) await _storage.clear();
      rethrow;
    }
  }

  Future<User> me() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>('/auth/me');
      return User.fromJson(response.data ?? {});
    } catch (error) {
      throw mapDioException(error, 'Gagal memuat sesi pengguna');
    }
  }

  Future<void> forgotPassword(String email) async {
    try {
      await _dio.post<void>('/auth/forgot-password', data: {'email': email});
    } catch (error) {
      throw mapDioException(error, 'Gagal mengirim kode reset');
    }
  }

  Future<void> resetPassword({
    required String email,
    required String code,
    required String newPassword,
  }) async {
    try {
      await _dio.post<void>(
        '/auth/reset-password',
        data: {'email': email, 'code': code, 'new_password': newPassword},
      );
    } catch (error) {
      throw mapDioException(error, 'Gagal mengatur ulang password');
    }
  }

  Future<String> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/auth/change-password',
        data: {
          'current_password': currentPassword,
          'new_password': newPassword,
        },
      );
      final message = response.data?['message'];
      return message is String && message.isNotEmpty
          ? message
          : 'Password berhasil diubah, silakan login ulang';
    } catch (error) {
      throw mapDioException(error, 'Gagal mengubah password');
    }
  }

  Future<void> logout() async {
    final tokens = await _storage.read();
    if (tokens != null) {
      try {
        await _dio.post<void>(
          '/auth/logout',
          data: {'refresh_token': tokens.refreshToken},
        );
      } catch (_) {
        // Logout lokal tetap dilakukan walaupun server sedang tidak terjangkau.
      }
    }
    await _storage.clear();
  }
}
