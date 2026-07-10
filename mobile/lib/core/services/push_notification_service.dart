import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:eoffice_mobile/app/navigation.dart';
import 'package:eoffice_mobile/core/network/api_client.dart';
import 'package:eoffice_mobile/core/services/push_navigation_intent.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

const _firebaseMessagingEnabled =
    bool.fromEnvironment('FIREBASE_MESSAGING_ENABLED');
const _notificationChannel = AndroidNotificationChannel(
  'eoffice_priority',
  'eOffice Priority',
  description: 'Approval, surat masuk, disposisi, dan SLA eOffice Pro.',
  importance: Importance.high,
);

final pushNotificationServiceProvider =
    Provider<PushNotificationService>((ref) {
  return PushNotificationService(ref.watch(dioProvider));
});

@pragma('vm:entry-point')
Future<void> firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  if (!_firebaseMessagingEnabled) return;
  await PushNotificationService.initializeFirebase();
}

class PushNotificationService {
  PushNotificationService(this._dio);

  final Dio _dio;
  static var _initialized = false;
  static var _handlersRegistered = false;
  static var _initialMessageRead = false;
  static PushNavigationIntent? _pendingIntent;
  static final _localNotifications = FlutterLocalNotificationsPlugin();
  var _listeningForRefresh = false;

  static bool get enabled => _firebaseMessagingEnabled;

  static Future<void> initializeFirebase() async {
    if (!enabled || _initialized) return;
    try {
      if (Firebase.apps.isEmpty) {
        await Firebase.initializeApp();
      }
      await _initializeLocalNotifications();
      _registerMessageHandlers();
      _initialized = true;
    } catch (error) {
      debugPrint('Firebase Messaging disabled: $error');
    }
  }

  Future<void> registerCurrentDevice() async {
    if (!enabled) return;
    await initializeFirebase();
    if (!_initialized) return;

    final messaging = FirebaseMessaging.instance;
    await messaging.requestPermission();
    await _localNotifications
        .resolvePlatformSpecificImplementation<
            AndroidFlutterLocalNotificationsPlugin>()
        ?.requestNotificationsPermission();
    final token = await messaging.getToken();
    if (token == null || token.isEmpty) return;
    await registerToken(token);
    await _captureInitialMessage();
    _navigatePendingIntent();

    if (!_listeningForRefresh) {
      _listeningForRefresh = true;
      messaging.onTokenRefresh.listen((token) {
        if (token.isNotEmpty) {
          registerToken(token).catchError((Object error) {
            debugPrint('Register refreshed FCM token failed: $error');
          });
        }
      });
    }
  }

  Future<void> unregisterCurrentDevice() async {
    if (!enabled) return;
    await initializeFirebase();
    if (!_initialized) return;

    final token = await FirebaseMessaging.instance.getToken();
    if (token == null || token.isEmpty) return;
    await unregisterToken(token);
  }

  Future<void> unregisterToken(String token) async {
    try {
      await _dio.delete<void>(
        '/push-tokens',
        data: {'token': token},
      );
    } catch (error) {
      debugPrint('Unregister FCM token failed: $error');
    }
  }

  Future<void> registerToken(String token, {String? deviceId}) async {
    final data = <String, dynamic>{
      'token': token,
      'platform': 'android',
      'device_info': 'android-tablet-fcm',
      'app_version': '0.1.0',
    };
    if (deviceId != null && deviceId.isNotEmpty) {
      data['device_id'] = deviceId;
    }
    await _dio.post<void>(
      '/push-tokens',
      data: data,
    );
  }

  static Future<void> _initializeLocalNotifications() async {
    const initializationSettings = InitializationSettings(
      android: AndroidInitializationSettings('@mipmap/ic_launcher'),
    );
    await _localNotifications.initialize(
      settings: initializationSettings,
      onDidReceiveNotificationResponse: (response) {
        final payload = response.payload;
        if (payload == null || payload.isEmpty) return;
        _navigateToPayload(payload);
      },
    );
    await _localNotifications
        .resolvePlatformSpecificImplementation<
            AndroidFlutterLocalNotificationsPlugin>()
        ?.createNotificationChannel(_notificationChannel);
  }

  static void _registerMessageHandlers() {
    if (_handlersRegistered) return;
    _handlersRegistered = true;
    FirebaseMessaging.onBackgroundMessage(firebaseMessagingBackgroundHandler);
    FirebaseMessaging.onMessage.listen(_showForegroundNotification);
    FirebaseMessaging.onMessageOpenedApp.listen((message) {
      _navigateToMessage(message);
    });
  }

  static Future<void> _captureInitialMessage() async {
    if (_initialMessageRead) return;
    _initialMessageRead = true;
    final message = await FirebaseMessaging.instance.getInitialMessage();
    if (message != null) {
      _pendingIntent = pushNavigationIntentFromData(message.data);
    }
  }

  static Future<void> _showForegroundNotification(RemoteMessage message) async {
    final intent = pushNavigationIntentFromData(message.data);
    final notification = message.notification;
    final title = notification?.title ?? 'eOffice Pro';
    final body = notification?.body ?? 'Notifikasi baru tersedia.';
    await _localNotifications.show(
      id: message.messageId.hashCode,
      title: title,
      body: body,
      notificationDetails: NotificationDetails(
        android: AndroidNotificationDetails(
          _notificationChannel.id,
          _notificationChannel.name,
          channelDescription: _notificationChannel.description,
          importance: Importance.high,
          priority: Priority.high,
        ),
      ),
      payload: intent?.location ?? jsonEncode(message.data),
    );
  }

  static void _navigateToMessage(RemoteMessage message) {
    final intent = pushNavigationIntentFromData(message.data);
    if (intent == null) return;
    _navigateOrStore(intent);
  }

  static void _navigateToPayload(String payload) {
    if (payload.startsWith('/app')) {
      final intent = _intentFromLocation(payload);
      if (intent == null) return;
      _navigateOrStore(intent);
      return;
    }
    try {
      final decoded = jsonDecode(payload);
      if (decoded is Map<String, dynamic>) {
        final intent = pushNavigationIntentFromData(decoded);
        if (intent != null) _navigateOrStore(intent);
      }
    } catch (_) {
      // Ignore malformed notification payloads.
    }
  }

  static void _navigatePendingIntent() {
    final intent = _pendingIntent;
    if (intent == null) return;
    if (_navigateOrStore(intent)) {
      _pendingIntent = null;
    }
  }

  static bool _navigateOrStore(PushNavigationIntent intent) {
    final context = rootNavigatorKey.currentContext;
    if (context == null) {
      _pendingIntent = intent;
      return false;
    }
    context.go(intent.location);
    return true;
  }

  static PushNavigationIntent? _intentFromLocation(String location) {
    final uri = Uri.tryParse(location);
    if (uri == null || uri.path != '/app') return null;
    return PushNavigationIntent(
      targetSection: uri.queryParameters['section'] ?? 'dashboard',
      letterId: uri.queryParameters['letter_id'],
      notificationId: uri.queryParameters['notification_id'],
    );
  }
}
