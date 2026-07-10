import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:eoffice_mobile/core/exceptions/app_exception.dart';
import 'package:eoffice_mobile/features/letters/data/draft_repository.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const recipient = DraftRecipient(
    type: DraftRecipientType.to,
    targetType: DraftRecipientTargetType.position,
    targetId: 'position-2',
    label: 'Direktur - Kantor Pusat',
  );
  const payload = DraftLetterPayload(
    companyId: 'company-1',
    letterTypeId: 'type-1',
    creatorPositionId: 'position-1',
    onBehalfOfPositionId: 'position-director',
    subject: 'Permohonan pengadaan',
    classification: LetterClassification.terbatas,
    priority: LetterPriority.urgent,
    bodyHtml: '<p>Isi surat</p>',
    recipients: [recipient],
  );

  test('parses a draft and serializes the exact save payload', () {
    final draft = DraftLetter.fromJson({
      'id': 'draft-1',
      'company_id': 'company-1',
      'company_code': 'KSK',
      'company_name': 'KSK Group',
      'letter_type_id': 'type-1',
      'letter_type_code': 'ND',
      'letter_type_name': 'Nota Dinas',
      'subject': 'Permohonan pengadaan',
      'classification': 'terbatas',
      'priority': 'urgent',
      'status': 'in_approval',
      'creator_position_id': 'position-1',
      'creator_position_title': 'Secretary',
      'on_behalf_of_position_id': 'position-director',
      'on_behalf_of_title': 'Director',
      'version': 3,
      'body_html': '<p>Isi surat</p>',
      'body_plain': 'Isi surat',
      'recipients': [
        {
          'type': 'to',
          'target_type': 'position',
          'target_id': 'position-2',
          'label': 'Direktur - Kantor Pusat',
        },
      ],
      'created_at': '2026-07-10T01:00:00Z',
      'updated_at': '2026-07-10T02:00:00Z',
    });

    expect(draft.status, DraftLetterStatus.inApproval);
    expect(draft.classification, LetterClassification.terbatas);
    expect(
        draft.recipients.single.targetType, DraftRecipientTargetType.position);
    expect(payload.toJson(), {
      'company_id': 'company-1',
      'letter_type_id': 'type-1',
      'creator_position_id': 'position-1',
      'on_behalf_of_position_id': 'position-director',
      'subject': 'Permohonan pengadaan',
      'classification': 'terbatas',
      'priority': 'urgent',
      'body_html': '<p>Isi surat</p>',
      'recipients': [
        {
          'type': 'to',
          'target_type': 'position',
          'target_id': 'position-2',
        },
      ],
    });
  });

  test('bootstrap loads every lookup page and flattens the org tree', () async {
    final adapter = _StubAdapter((options) {
      final page = options.queryParameters['page'] as int? ?? 1;
      return switch (options.path) {
        '/companies' => _jsonResponse({
            'data': [
              {
                'id': 'company-$page',
                'code': 'C$page',
                'name': 'Company $page',
                'is_active': true,
              },
            ],
            'meta': {'page': page, 'page_size': 100, 'total_pages': 2},
          }),
        '/letter-types' => _pagedResponse([
            {
              'id': 'type-1',
              'code': 'ND',
              'name': 'Nota Dinas',
              'default_classification': 'rahasia',
              'default_sla_hours': 24,
              'is_active': true,
            },
          ]),
        '/letter-templates' => _pagedResponse([
            {
              'id': 'template-1',
              'letter_type_id': 'type-1',
              'letter_type_code': 'ND',
              'letter_type_name': 'Nota Dinas',
              'company_id': 'company-1',
              'company_code': 'C1',
              'company_name': 'Company 1',
              'version': 1,
              'layout_config': {'paper': 'A4'},
              'body_skeleton': '<p>Template</p>',
              'is_active': true,
              'created_at': '2026-07-10T01:00:00Z',
            },
          ]),
        '/positions' => _pagedResponse([
            {
              'id': 'position-1',
              'title': 'Secretary',
              'position_type': 'secretary',
              'is_approver': false,
              'is_active': true,
              'reports_to': 'position-director',
              'reports_to_title': 'Director',
              'org_unit_id': 'unit-child',
              'org_unit_name': 'Sekretariat',
              'org_unit_level': 'division',
              'holder_name': 'User',
              'holder_user_id': 'user-1',
              'identity_locked': true,
            },
          ]),
        '/org-units' => _jsonResponse({
            'tree': [
              {
                'id': 'unit-root',
                'parent_id': null,
                'code': 'DIR',
                'name': 'Direktorat',
                'unit_level': 'directorate',
                'is_active': true,
                'children': [
                  {
                    'id': 'unit-child',
                    'parent_id': 'unit-root',
                    'code': 'SEC',
                    'name': 'Sekretariat',
                    'unit_level': 'division',
                    'is_active': true,
                  },
                ],
              },
            ],
          }),
        _ => _jsonResponse({}, statusCode: 404),
      };
    });
    final repository = DraftRepository(_dioWith(adapter));

    final bootstrap = await repository.loadBootstrap();

    expect(
        bootstrap.companies.map((item) => item.id), ['company-1', 'company-2']);
    expect(bootstrap.letterTypes.single.defaultClassification,
        LetterClassification.rahasia);
    expect(bootstrap.templates.single.layoutConfig['paper'], 'A4');
    expect(bootstrap.positions.single.reportsTo, 'position-director');
    expect(
        bootstrap.orgUnits.map((unit) => unit.id), ['unit-root', 'unit-child']);
    final companyRequests = adapter.requests
        .where((request) => request.path == '/companies')
        .toList();
    expect(companyRequests, hasLength(2));
    expect(companyRequests.last.queryParameters['page'], 2);
  });

  test('save chooses POST or PUT and preserves the typed result', () async {
    final adapter = _StubAdapter((options) {
      if (options.method == 'POST') {
        return _jsonResponse({'id': 'draft-new', 'version': 1},
            statusCode: 201);
      }
      return _jsonResponse({'id': 'draft-1', 'version': 4});
    });
    final repository = DraftRepository(_dioWith(adapter));

    final created = await repository.saveDraft(payload: payload);
    final updated = await repository.saveDraft(
      draftId: 'draft-1',
      payload: payload,
    );

    expect(created.id, 'draft-new');
    expect(created.version, 1);
    expect(updated.version, 4);
    expect(adapter.requests[0].method, 'POST');
    expect(adapter.requests[0].path, '/letters/drafts');
    expect(adapter.requests[1].method, 'PUT');
    expect(adapter.requests[1].path, '/letters/drafts/draft-1');
    expect(
      (adapter.requests[1].data as Map<String, dynamic>)['priority'],
      'urgent',
    );
  });

  test('lists drafts and gets a single draft with typed parsing', () async {
    final draftJson = {
      'id': 'draft-1',
      'classification': 'biasa',
      'priority': 'normal',
      'status': 'revision',
      'version': 2,
      'recipients': <Map<String, dynamic>>[],
    };
    final adapter = _StubAdapter((options) {
      if (options.path.endsWith('/draft-1')) {
        return _jsonResponse({'letter': draftJson});
      }
      return _jsonResponse({
        'letters': [draftJson],
      });
    });
    final repository = DraftRepository(_dioWith(adapter));

    final drafts = await repository.listDrafts();
    final draft = await repository.getDraft('draft-1');

    expect(drafts.single.status, DraftLetterStatus.revision);
    expect(draft.version, 2);
  });

  test('handles attachments, preview, and submit contracts', () async {
    final adapter = _StubAdapter((options) {
      if (options.path.endsWith('/attachments') && options.method == 'GET') {
        return _jsonResponse({
          'attachments': [
            {
              'id': 'attachment-1',
              'file_name': 'memo.pdf',
              'mime_type': 'application/pdf',
              'size_bytes': 3,
              'storage_key': 'letters/draft-1/memo.pdf',
              'checksum_sha256': 'abc',
              'scan_status': 'clean',
              'download_url': 'https://files.test/memo.pdf',
              'created_at': '2026-07-10T01:00:00Z',
            },
          ],
        });
      }
      if (options.path.endsWith('/attachments') && options.method == 'POST') {
        return _jsonResponse({'id': 'attachment-new'}, statusCode: 201);
      }
      if (options.path.endsWith('/preview')) {
        return _jsonResponse({
          'storage_key': 'letters/draft-1/preview.pdf',
          'preview_url': 'https://files.test/preview.pdf',
          'expires_in': 900,
        });
      }
      if (options.path.endsWith('/submit')) {
        return _jsonResponse({
          'id': 'draft-1',
          'status': 'in_approval',
          'approval_cycle': 2,
          'qr_token': 'qr-token',
          'verify_url': 'https://web.test/verify/qr-token',
          'approval_steps': [
            {
              'step_order': 1,
              'flow_group': 1,
              'position_id': 'approver-1',
              'position_type': 'gm',
              'title': 'General Manager',
            },
          ],
        });
      }
      return _jsonResponse({'id': 'attachment-1'});
    });
    final repository = DraftRepository(_dioWith(adapter));

    final attachments = await repository.listAttachments('draft-1');
    final attachmentId = await repository.uploadAttachment(
      draftId: 'draft-1',
      bytes: Uint8List.fromList([1, 2, 3]),
      fileName: 'memo.pdf',
      mimeType: 'application/pdf',
    );
    await repository.deleteAttachment(
      draftId: 'draft-1',
      attachmentId: 'attachment-1',
    );
    final preview = await repository.previewDraft('draft-1');
    final submit = await repository.submitDraft('draft-1');

    expect(attachments.single.scanStatus, 'clean');
    expect(attachmentId, 'attachment-new');
    expect(preview.expiresIn, 900);
    expect(submit.status, DraftLetterStatus.inApproval);
    expect(submit.approvalSteps.single.positionType, 'gm');

    final uploadRequest = adapter.requests.firstWhere(
      (request) => request.data is FormData,
    );
    final form = uploadRequest.data as FormData;
    expect(form.files.single.key, 'file');
    expect(form.files.single.value.filename, 'memo.pdf');
    expect(form.files.single.value.contentType.toString(), 'application/pdf');
    expect(form.files.single.value.length, 3);
  });

  test('maps backend error messages to AppException', () async {
    final adapter = _StubAdapter(
      (_) => _jsonResponse(
        {'error': 'minimal satu penerima tujuan wajib dipilih'},
        statusCode: 400,
      ),
    );
    final repository = DraftRepository(_dioWith(adapter));

    expect(
      () => repository.createDraft(payload),
      throwsA(
        isA<AppException>()
            .having((error) => error.statusCode, 'statusCode', 400)
            .having(
              (error) => error.message,
              'message',
              'minimal satu penerima tujuan wajib dipilih',
            ),
      ),
    );
  });
}

Dio _dioWith(HttpClientAdapter adapter) {
  return Dio(BaseOptions(baseUrl: 'http://example.test'))
    ..httpClientAdapter = adapter;
}

_StubResponse _pagedResponse(List<Map<String, dynamic>> data) {
  return _jsonResponse({
    'data': data,
    'meta': {'page': 1, 'page_size': 100, 'total_pages': 1},
  });
}

_StubResponse _jsonResponse(
  Map<String, dynamic> data, {
  int statusCode = 200,
}) {
  return _StubResponse(data, statusCode);
}

typedef _RequestHandler = FutureOr<_StubResponse> Function(
  RequestOptions options,
);

class _StubAdapter implements HttpClientAdapter {
  _StubAdapter(this._handler);

  final _RequestHandler _handler;
  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    final result = await _handler(options);
    return ResponseBody.fromString(
      jsonEncode(result.data),
      result.statusCode,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

class _StubResponse {
  const _StubResponse(this.data, this.statusCode);

  final Map<String, dynamic> data;
  final int statusCode;
}
