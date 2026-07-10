import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:eoffice_mobile/core/network/api_client.dart';
import 'package:eoffice_mobile/core/services/token_storage.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const requestCount = 4;

  test('parallel 401 responses share one refresh and retry with new token',
      () async {
    final storage = _MemoryTokenStorage(_oldTokens);
    final protectedAdapter = _ProtectedAdapter();
    final refreshAdapter = _RefreshAdapter();
    var refreshedEvents = 0;
    var expiredEvents = 0;
    final dio = _createDio(
      storage: storage,
      protectedAdapter: protectedAdapter,
      refreshAdapter: refreshAdapter,
      onSessionRefreshed: () => refreshedEvents++,
      onSessionExpired: () => expiredEvents++,
    );

    final requests = List.generate(
      requestCount,
      (index) => dio.get<Map<String, dynamic>>('/protected/$index'),
    );
    await _waitUntil(
      () =>
          protectedAdapter.oldTokenRequests == requestCount &&
          refreshAdapter.calls == 1,
    );

    refreshAdapter.release();
    final responses = await Future.wait(requests);

    expect(responses.map((response) => response.statusCode), everyElement(200));
    expect(refreshAdapter.calls, 1);
    expect(refreshAdapter.refreshTokens, ['old-refresh']);
    expect(protectedAdapter.oldTokenRequests, requestCount);
    expect(protectedAdapter.newTokenRequests, requestCount);
    expect(
      protectedAdapter.retryAuthorizations,
      everyElement('Bearer new-access'),
    );
    expect(storage.tokens?.accessToken, 'new-access');
    expect(storage.tokens?.refreshToken, 'new-refresh');
    expect(storage.saveCalls, 1);
    expect(storage.clearCalls, 0);
    expect(refreshedEvents, 1);
    expect(expiredEvents, 0);
  });

  test('parallel terminal refresh failure clears and expires session once',
      () async {
    final storage = _MemoryTokenStorage(_oldTokens);
    final protectedAdapter = _ProtectedAdapter();
    final refreshAdapter = _RefreshAdapter(fail: true);
    var refreshedEvents = 0;
    var expiredEvents = 0;
    final dio = _createDio(
      storage: storage,
      protectedAdapter: protectedAdapter,
      refreshAdapter: refreshAdapter,
      onSessionRefreshed: () => refreshedEvents++,
      onSessionExpired: () => expiredEvents++,
    );

    final requests = List.generate(
      requestCount,
      (index) async {
        try {
          await dio.get<Map<String, dynamic>>('/protected/$index');
          return null;
        } on DioException catch (error) {
          return error;
        }
      },
    );
    await _waitUntil(
      () =>
          protectedAdapter.oldTokenRequests == requestCount &&
          refreshAdapter.calls == 1,
    );

    refreshAdapter.release();
    final errors = await Future.wait(requests);

    expect(errors, everyElement(isA<DioException>()));
    expect(refreshAdapter.calls, 1);
    expect(refreshAdapter.refreshTokens, ['old-refresh']);
    expect(storage.tokens, isNull);
    expect(storage.saveCalls, 0);
    expect(storage.clearCalls, 1);
    expect(refreshedEvents, 0);
    expect(expiredEvents, 1);
  });
}

Dio _createDio({
  required _MemoryTokenStorage storage,
  required _ProtectedAdapter protectedAdapter,
  required _RefreshAdapter refreshAdapter,
  required void Function() onSessionRefreshed,
  required void Function() onSessionExpired,
}) {
  final dio = Dio(BaseOptions(baseUrl: 'http://example.test'))
    ..httpClientAdapter = protectedAdapter;
  dio.interceptors.add(
    AuthInterceptor(
      dio: dio,
      storage: storage,
      refreshDio: refreshAdapter.dio,
      onSessionRefreshed: onSessionRefreshed,
      onSessionExpired: onSessionExpired,
    ),
  );
  return dio;
}

Future<void> _waitUntil(
  bool Function() condition, {
  Duration timeout = const Duration(seconds: 2),
}) async {
  final stopwatch = Stopwatch()..start();
  while (!condition()) {
    if (stopwatch.elapsed >= timeout) {
      fail('Timed out while waiting for concurrent interceptor requests.');
    }
    await Future<void>.delayed(const Duration(milliseconds: 5));
  }
}

const _oldTokens = TokenPair(
  accessToken: 'old-access',
  refreshToken: 'old-refresh',
);

class _MemoryTokenStorage extends TokenStorage {
  _MemoryTokenStorage(this.tokens) : super(const FlutterSecureStorage());

  TokenPair? tokens;
  var saveCalls = 0;
  var clearCalls = 0;

  @override
  Future<TokenPair?> read() async => tokens;

  @override
  Future<void> save(TokenPair pair) async {
    saveCalls++;
    tokens = pair;
  }

  @override
  Future<void> clear() async {
    clearCalls++;
    tokens = null;
  }
}

class _ProtectedAdapter implements HttpClientAdapter {
  var oldTokenRequests = 0;
  var newTokenRequests = 0;
  final retryAuthorizations = <String?>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final authorization = options.headers['Authorization'] as String?;
    if (authorization == 'Bearer old-access') {
      oldTokenRequests++;
      return _jsonResponse({'error': 'access token expired'}, statusCode: 401);
    }
    if (authorization == 'Bearer new-access') {
      newTokenRequests++;
      retryAuthorizations.add(authorization);
      return _jsonResponse({'path': options.path});
    }
    return _jsonResponse({'error': 'missing token'}, statusCode: 401);
  }

  @override
  void close({bool force = false}) {}
}

class _RefreshAdapter implements HttpClientAdapter {
  _RefreshAdapter({this.fail = false}) {
    dio = Dio(BaseOptions(baseUrl: 'http://example.test'))
      ..httpClientAdapter = this;
  }

  final bool fail;
  final _gate = Completer<void>();
  late final Dio dio;
  var calls = 0;
  final refreshTokens = <String>[];

  void release() {
    if (!_gate.isCompleted) _gate.complete();
  }

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    calls++;
    final body = await _readJsonBody(requestStream);
    refreshTokens.add(body['refresh_token'] as String);
    await _gate.future;
    if (fail) {
      return _jsonResponse({'error': 'refresh rejected'}, statusCode: 401);
    }
    return _jsonResponse({
      'access_token': 'new-access',
      'refresh_token': 'new-refresh',
    });
  }

  @override
  void close({bool force = false}) {}
}

Future<Map<String, dynamic>> _readJsonBody(
  Stream<Uint8List>? requestStream,
) async {
  if (requestStream == null) return const {};
  final bytes = await requestStream.expand((chunk) => chunk).toList();
  return jsonDecode(utf8.decode(bytes)) as Map<String, dynamic>;
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
