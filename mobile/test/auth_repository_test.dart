import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:eoffice_mobile/core/exceptions/app_exception.dart';
import 'package:eoffice_mobile/core/services/token_storage.dart';
import 'package:eoffice_mobile/features/auth/data/auth_repository.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('login refreshes the profile so active positions are available',
      () async {
    final adapter = _AuthAdapter();
    final dio = Dio(BaseOptions(baseUrl: 'http://example.test'))
      ..httpClientAdapter = adapter;
    final storage = _MemoryTokenStorage();
    final repository = AuthRepository(dio: dio, storage: storage);

    final user = await repository.login(
      identifier: 'creator@example.test',
      password: 'password',
    );

    expect(adapter.paths, ['/auth/login', '/auth/me']);
    expect(storage.tokens?.accessToken, 'access-token');
    expect(user.roles, ['creator']);
    expect(user.positions.single.positionId, 'position-1');
  });

  test('login clears partial tokens when loading the profile fails', () async {
    final adapter = _AuthAdapter(failProfile: true);
    final dio = Dio(BaseOptions(baseUrl: 'http://example.test'))
      ..httpClientAdapter = adapter;
    final storage = _MemoryTokenStorage();
    final repository = AuthRepository(dio: dio, storage: storage);

    await expectLater(
      repository.login(
        identifier: 'creator@example.test',
        password: 'password',
      ),
      throwsA(
        isA<AppException>().having(
          (error) => error.message,
          'message',
          'profile gagal',
        ),
      ),
    );

    expect(storage.tokens, isNull);
    expect(storage.clearCalls, 1);
  });
}

class _MemoryTokenStorage extends TokenStorage {
  _MemoryTokenStorage() : super(const FlutterSecureStorage());

  TokenPair? tokens;
  var clearCalls = 0;

  @override
  Future<TokenPair?> read() async => tokens;

  @override
  Future<void> save(TokenPair pair) async => tokens = pair;

  @override
  Future<void> clear() async {
    clearCalls++;
    tokens = null;
  }
}

class _AuthAdapter implements HttpClientAdapter {
  _AuthAdapter({this.failProfile = false});

  final bool failProfile;
  final paths = <String>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    paths.add(options.path);
    if (options.path == '/auth/login') {
      return _jsonResponse({
        'access_token': 'access-token',
        'refresh_token': 'refresh-token',
        'user': {
          'id': 'user-1',
          'email': 'creator@example.test',
          'full_name': 'Creator',
          'roles': ['creator'],
          'positions': [],
        },
      });
    }
    if (options.path == '/auth/me' && failProfile) {
      return _jsonResponse({'error': 'profile gagal'}, statusCode: 500);
    }
    if (options.path == '/auth/me') {
      return _jsonResponse({
        'id': 'user-1',
        'email': 'creator@example.test',
        'full_name': 'Creator',
        'roles': ['creator'],
        'positions': [
          {
            'position_id': 'position-1',
            'title': 'Staff',
            'position_type': 'staff',
            'org_unit': 'Operasional',
            'assignment_type': 'definitive',
          },
        ],
      });
    }
    return _jsonResponse({'error': 'not found'}, statusCode: 404);
  }

  ResponseBody _jsonResponse(
    Map<String, dynamic> body, {
    int statusCode = 200,
  }) {
    return ResponseBody.fromString(
      jsonEncode(body),
      statusCode,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
