import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:eoffice_mobile/core/services/push_navigation_intent.dart';
import 'package:eoffice_mobile/core/services/push_notification_service.dart';
import 'package:eoffice_mobile/features/auth/domain/user.dart';
import 'package:eoffice_mobile/features/home/domain/dashboard_summary.dart';
import 'package:eoffice_mobile/features/home/presentation/home_page.dart';
import 'package:eoffice_mobile/features/letters/data/letter_repository.dart';
import 'package:eoffice_mobile/features/letters/domain/letter_models.dart';
import 'package:eoffice_mobile/features/letters/presentation/signature_pad.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('parses user positions from auth payload', () {
    final user = User.fromJson({
      'id': 'u-1',
      'email': 'auditor@example.com',
      'full_name': 'Approver',
      'roles': ['auditor'],
      'positions': [
        {
          'position_id': 'p-1',
          'title': 'GM Biro',
          'position_type': 'gm',
          'org_unit': 'Head Office',
          'assignment_type': 'definitive',
        },
      ],
    });

    expect(user.fullName, 'Approver');
    expect(user.positions.single.title, 'GM Biro');
  });

  test('allows compose entry only for authorized users with a position', () {
    User userWith({required List<String> roles, required bool hasPosition}) {
      return User.fromJson({
        'id': 'u-1',
        'email': 'user@example.com',
        'full_name': 'User',
        'roles': roles,
        'positions': hasPosition
            ? [
                {
                  'position_id': 'p-1',
                  'title': 'Staff',
                  'position_type': 'staff',
                  'org_unit': 'Head Office',
                  'assignment_type': 'definitive',
                },
              ]
            : <Map<String, dynamic>>[],
      });
    }

    expect(
      canComposeLetters(userWith(roles: const ['creator'], hasPosition: true)),
      isTrue,
    );
    expect(
      canComposeLetters(userWith(roles: const ['auditor'], hasPosition: true)),
      isFalse,
    );
    expect(
      canComposeLetters(userWith(roles: const ['admin'], hasPosition: false)),
      isFalse,
    );
    expect(canComposeLetters(null), isFalse);
  });

  test('parses letter detail lists safely', () {
    final letter = LetterDetail.fromJson({
      'id': 'l-1',
      'company_code': 'KSK',
      'company_name': 'KSK Group',
      'letter_type_code': 'ND',
      'letter_type_name': 'Nota Dinas',
      'subject': 'Uji',
      'classification': 'biasa',
      'priority': 'normal',
      'status': 'published',
      'creator_name': 'Creator',
      'creator_position_title': 'Staff',
      'version': 1,
      'body_plain': 'Isi surat',
      'recipients': [
        {
          'type': 'to',
          'target_type': 'position',
          'target_id': 'p-2',
          'label': 'Direktur',
        },
      ],
      'attachments': [],
      'approval_steps': [],
      'approval_actions': [],
      'created_at': '2026-07-09T00:00:00Z',
      'updated_at': '2026-07-09T00:00:00Z',
    });

    expect(letter.subject, 'Uji');
    expect(letter.recipients.single.label, 'Direktur');
  });

  test('parses dashboard summary with trend and letter id', () {
    final summary = DashboardSummary.fromJson({
      'stats': {
        'inbox_unread': 2,
        'sent_this_month': 4,
        'pending_approvals': 1,
        'archived_total': 7,
      },
      'incoming_trend': [
        {'date': '2026-07-09', 'total': 0},
        {'date': '2026-07-10', 'total': 3},
      ],
      'pending_approvals': [
        {
          'step_id': 'step-1',
          'letter_id': 'letter-1',
          'subject': 'Kebutuhan Server',
          'creator_name': 'Creator',
          'updated_at': '2026-07-10T09:00:00Z',
        },
      ],
      'recent_activities': [
        {
          'id': 'n-1',
          'event_type': 'letter_incoming',
          'title': 'Surat masuk: test',
          'created_at': '2026-07-10T09:30:00Z',
        },
      ],
    });

    expect(summary.stats.archivedTotal, 7);
    expect(summary.incomingTrend, hasLength(2));
    expect(summary.incomingTrend.last.date, '2026-07-10');
    expect(summary.incomingTrend.last.total, 3);
    expect(summary.pendingApprovals.single.letterId, 'letter-1');
    expect(summary.pendingApprovals.single.stepId, 'step-1');
    expect(summary.recentActivities.single.eventType, 'letter_incoming');
  });

  test('parses dashboard summary from old backend without new fields', () {
    final summary = DashboardSummary.fromJson({
      'stats': {'inbox_unread': 1},
      'pending_approvals': [
        {'step_id': 'step-1', 'subject': 'Tanpa letter id'},
      ],
    });

    expect(summary.incomingTrend, isEmpty);
    expect(summary.pendingApprovals.single.letterId, '');
    expect(summary.recentActivities, isEmpty);
  });

  test('uses two-pane layout only for tablet landscape', () {
    const landscape = TabletLayoutSpec(Size(1280, 800));
    const portrait = TabletLayoutSpec(Size(800, 1280));

    expect(landscape.useTwoPane, isTrue);
    expect(landscape.listPaneWidth, 420);
    expect(portrait.useTwoPane, isFalse);
    expect(portrait.detailPadding, const EdgeInsets.all(16));
  });

  test('maps push notification data to app route', () {
    final approval = pushNavigationIntentFromData({
      'event_type': 'approval_waiting',
      'letter_id': 'letter-1',
    });
    expect(
      approval?.location,
      '/app?section=approvals&letter_id=letter-1',
    );

    final disposition = pushNavigationIntentFromData({
      'event_type': 'disposition_assigned',
      'letter_id': 'letter-2',
      'notification_id': 'notification-1',
    });
    expect(
      disposition?.location,
      '/app?section=dispositions&letter_id=letter-2&notification_id=notification-1',
    );

    final explicit = pushNavigationIntentFromData({
      'event_type': 'approval_result',
      'target_section': 'inbox',
      'letter_id': 'letter-3',
    });
    expect(explicit?.location, '/app?section=inbox&letter_id=letter-3');
  });

  test('maps route section values to work sections', () {
    expect(workSectionFromRouteValue('dashboard'), WorkSection.dashboard);
    expect(workSectionFromRouteValue('inbox'), WorkSection.inbox);
    expect(workSectionFromRouteValue('dispositions'), WorkSection.dispositions);
    expect(workSectionFromRouteValue('unknown'), WorkSection.approvals);
  });

  testWidgets('signature dialog enables submit after stroke and clears it',
      (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: SignatureCaptureDialog(),
        ),
      ),
    );

    FilledButton submitButton() {
      return tester.widget<FilledButton>(
        find.widgetWithText(FilledButton, 'Tanda tangani'),
      );
    }

    TextButton clearButton() {
      return tester.widget<TextButton>(
        find.widgetWithText(TextButton, 'Bersihkan'),
      );
    }

    expect(submitButton().onPressed, isNull);
    expect(clearButton().onPressed, isNull);

    await tester.drag(find.byType(SignaturePad), const Offset(160, 40));
    await tester.pump();

    expect(submitButton().onPressed, isNotNull);
    expect(clearButton().onPressed, isNotNull);

    await tester.tap(find.widgetWithText(TextButton, 'Bersihkan'));
    await tester.pump();

    expect(submitButton().onPressed, isNull);
    expect(clearButton().onPressed, isNull);
  });

  test('approval repository sends signature payload for approve', () async {
    final adapter = _CaptureAdapter();
    final dio = Dio(BaseOptions(baseUrl: 'http://example.test'))
      ..httpClientAdapter = adapter;
    final repository = LetterRepository(dio);

    await repository.actApproval(
      stepId: 'step-1',
      action: 'approve',
      signatureImageBase64: 'png-base64',
    );

    expect(adapter.path, '/approvals/steps/step-1/actions');
    expect(adapter.body?['action'], 'approve');
    expect(adapter.body?['signature_image_base64'], 'png-base64');
    expect(adapter.body?['signature_mime_type'], 'image/png');
    expect(adapter.body?['client_action_id'], isA<String>());
    expect(adapter.body?['device_info'], 'android-tablet-online');
  });

  test('push service sends register and unregister token payloads', () async {
    final adapter = _CaptureAdapter();
    final dio = Dio(BaseOptions(baseUrl: 'http://example.test'))
      ..httpClientAdapter = adapter;
    final service = PushNotificationService(dio);

    await service.registerToken('fcm-token', deviceId: 'device-1');

    expect(adapter.method, 'POST');
    expect(adapter.path, '/push-tokens');
    expect(adapter.body?['token'], 'fcm-token');
    expect(adapter.body?['platform'], 'android');
    expect(adapter.body?['app_version'], '0.1.0');
    expect(adapter.body?['device_id'], 'device-1');

    await service.unregisterToken('fcm-token');

    expect(adapter.method, 'DELETE');
    expect(adapter.path, '/push-tokens');
    expect(adapter.body?['token'], 'fcm-token');
  });
}

class _CaptureAdapter implements HttpClientAdapter {
  String? method;
  String? path;
  Map<String, dynamic>? body;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    method = options.method;
    path = options.uri.path;
    body = Map<String, dynamic>.from(options.data as Map);
    return ResponseBody.fromString(
      '{}',
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
