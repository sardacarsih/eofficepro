import 'package:eoffice_mobile/app/app.dart';
import 'package:eoffice_mobile/core/services/push_notification_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await PushNotificationService.initializeFirebase();
  runApp(const ProviderScope(child: EOfficeApp()));
}
