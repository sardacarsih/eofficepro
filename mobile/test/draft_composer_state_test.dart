import 'package:eoffice_mobile/features/letters/domain/draft_composer_state.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:eoffice_mobile/features/letters/domain/letter_body_codec.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('DraftComposerForm validation', () {
    test('requires subject, To recipient, and body', () {
      final form = DraftComposerForm.empty(
        bootstrap: _bootstrap(),
        creatorPositionId: 'staff-a',
      );

      expect(form.validationMessage, 'Perihal wajib diisi.');
      expect(
        form.copyWith(subject: 'Pengadaan').validationMessage,
        'Minimal satu penerima To wajib dipilih.',
      );
      expect(
        form.copyWith(
          subject: 'Pengadaan',
          recipients: [_recipient('target-a')],
        ).validationMessage,
        'Isi surat wajib diisi.',
      );
    });

    test('builds a safe API payload from valid plain text', () {
      final form = _validForm('staff-a').copyWith(
        bodyPlain: 'Nilai < 10 & disetujui',
      );

      final json = form.toPayload().toJson();

      expect(json['body_html'], '<p>Nilai &lt; 10 &amp; disetujui</p>');
      expect(json['priority'], 'normal');
      expect(
        json['recipients'],
        [
          {
            'type': 'to',
            'target_type': 'position',
            'target_id': 'target-a',
          },
        ],
      );
    });

    test('preserves rich source HTML when the body is not edited', () {
      const sourceHtml =
          '<h2>Persetujuan</h2><p>Mohon <strong>ditindaklanjuti</strong>.</p>';
      final form = DraftComposerForm.fromDraft(
        _draft(
          bodyHtml: sourceHtml,
          bodyPlain: 'Persetujuan Mohon ditindaklanjuti.',
        ),
      );

      expect(form.bodyReadOnly, isTrue);
      expect(form.toPayload().bodyHtml, sourceHtml);
    });

    test('unsupported body can be explicitly converted to safe plain HTML', () {
      final form = DraftComposerForm.fromDraft(
        _draft(
          bodyHtml: '<h2>Persetujuan</h2><p>Mohon ditindaklanjuti.</p>',
          bodyPlain: 'Persetujuan Mohon ditindaklanjuti.',
        ),
      );

      expect(form.bodyReadOnly, isTrue);

      // Mirrors DraftComposerController.simplifyBodyFormatting.
      final simplified = form.copyWith(
        bodyHtml: plainTextToLetterHtml(form.bodyPlain),
        sourceBodyHtml: '',
        sourceBodyEditableHtml: '',
      );

      expect(simplified.bodyReadOnly, isFalse);
      expect(
        simplified.toPayload().bodyHtml,
        '<p>Persetujuan</p><p>Mohon ditindaklanjuti.</p>',
      );
    });

    test('simple formatting is natively editable and round-trips verbatim', () {
      const sourceHtml = '<p>Mohon <strong>ditindaklanjuti</strong>.</p>'
          '<ul><li>Poin satu</li></ul>';
      final form = DraftComposerForm.fromDraft(
        _draft(
          bodyHtml: sourceHtml,
          bodyPlain: 'Mohon ditindaklanjuti. Poin satu',
        ),
      );

      expect(form.bodyReadOnly, isFalse);
      expect(form.bodyHtml, sourceHtml);
      expect(form.toPayload().bodyHtml, sourceHtml);
    });

    test('edited rich body sends the canonical editor HTML', () {
      final form = DraftComposerForm.fromDraft(
        _draft(
          bodyHtml: '<p>Mohon <strong>ditindaklanjuti</strong>.</p>',
          bodyPlain: 'Mohon ditindaklanjuti.',
        ),
      );

      final edited = form.copyWith(
        bodyHtml: '<p>Mohon <strong>segera ditindaklanjuti</strong>.</p>',
        bodyPlain: 'Mohon segera ditindaklanjuti.',
      );

      expect(
        edited.toPayload().bodyHtml,
        '<p>Mohon <strong>segera ditindaklanjuti</strong>.</p>',
      );
    });

    test('derives multi-paragraph text from HTML instead of flattened body',
        () {
      const sourceHtml = '<p>Paragraf pertama.</p><p>Paragraf kedua.</p>';
      final form = DraftComposerForm.fromDraft(
        _draft(
          bodyHtml: sourceHtml,
          bodyPlain: 'Paragraf pertama. Paragraf kedua.',
        ),
      );

      expect(form.bodyPlain, 'Paragraf pertama.\nParagraf kedua.');
      expect(form.bodyPlain, isNot('Paragraf pertama. Paragraf kedua.'));
      expect(form.bodyReadOnly, isFalse);
      expect(form.toPayload().bodyHtml, sourceHtml);
    });

    test('rejects a subject longer than 255 UTF-8 bytes', () {
      final subject = List.filled(128, 'é').join();
      final form = _validForm('staff-a').copyWith(subject: subject);

      expect(subject.length, lessThanOrEqualTo(255));
      expect(form.validationMessage, 'Perihal maksimal 255 byte.');
      expect(form.toPayload, throwsStateError);
    });
  });

  group('active reference validation', () {
    test('rejects a stale company', () {
      final state = _stateWith(
        _validForm('staff-a').copyWith(companyId: 'company-inactive'),
      );

      expect(
        state.validationMessage,
        'Perusahaan pada draft sudah tidak aktif. Pilih perusahaan lain.',
      );
    });

    test('rejects a stale letter type', () {
      final state = _stateWith(
        _validForm('staff-a').copyWith(letterTypeId: 'type-inactive'),
      );

      expect(
        state.validationMessage,
        'Jenis surat pada draft sudah tidak aktif. Pilih jenis lain.',
      );
    });

    test('rejects a stale creator position', () {
      final state = _stateWith(
        _validForm('staff-a').copyWith(creatorPositionId: 'former-position'),
      );

      expect(
        state.validationMessage,
        'Jabatan pembuat sudah tidak aktif. Pilih jabatan lain.',
      );
    });

    test('rejects a stale recipient', () {
      final state = _stateWith(
        _validForm('staff-a').copyWith(
          recipients: [_recipient('target-inactive')],
        ),
      );

      expect(state.validationMessage, contains('Penerima target-inactive'));
      expect(state.validationMessage, contains('sudah tidak aktif'));
    });
  });

  group('company access from creator assignments', () {
    test('limits companies and creator positions to assigned companies', () {
      const bootstrap = DraftComposerBootstrap(
        companies: [
          DraftCompany(
            id: 'company-a',
            code: 'AAA',
            name: 'Company A',
            isActive: true,
          ),
          DraftCompany(
            id: 'company-b',
            code: 'BBB',
            name: 'Company B',
            isActive: true,
          ),
          DraftCompany(
            id: 'company-c',
            code: 'CCC',
            name: 'Company C',
            isActive: true,
          ),
        ],
        letterTypes: [],
        templates: [],
        positions: [
          DraftPosition(
            id: 'creator-a',
            title: 'Creator A',
            positionType: 'staff',
            isApprover: false,
            isActive: true,
            reportsToTitle: '',
            orgUnitId: 'unit-a',
            orgUnitName: 'Unit A',
            orgUnitLevel: 'department',
            holderName: '',
            holderUserId: '',
            identityLocked: false,
          ),
          DraftPosition(
            id: 'creator-b',
            title: 'Creator B',
            positionType: 'staff',
            isApprover: false,
            isActive: true,
            reportsToTitle: '',
            orgUnitId: 'unit-b',
            orgUnitName: 'Unit B',
            orgUnitLevel: 'department',
            holderName: '',
            holderUserId: '',
            identityLocked: false,
          ),
        ],
        orgUnits: [],
      );
      const state = DraftComposerState(
        bootstrap: bootstrap,
        drafts: [],
        creatorPositionIds: ['creator-a', 'creator-b'],
        creatorCompanyByPosition: {
          'creator-a': 'company-a',
          'creator-b': 'company-b',
        },
        form: DraftComposerForm(
          companyId: 'company-a',
          letterTypeId: '',
          creatorPositionId: 'creator-a',
          subject: '',
          classification: LetterClassification.biasa,
          priority: LetterPriority.normal,
          bodyPlain: '',
          recipients: [],
        ),
      );

      expect(state.availableCompanies.map((company) => company.id), [
        'company-a',
        'company-b',
      ]);
      expect(
          state.creatorPositions.map((position) => position.id), ['creator-a']);
      expect(
        state
            .creatorPositionsForCompany('company-b')
            .map((position) => position.id),
        ['creator-b'],
      );
      expect(state.companyForCreatorPosition('creator-b'), 'company-b');
    });
  });

  group('recipient directorate policy', () {
    test('blocks staff from sending across directorates', () {
      final form = _validForm('staff-a').copyWith(
        recipients: [_recipient('target-b')],
      );

      expect(
        recipientPolicyMessage(form, _bootstrap()),
        contains('Sub Department Head'),
      );
    });

    test('allows sub department head to target cross-directorate position', () {
      final form = _validForm('sub-head-a').copyWith(
        recipients: [_recipient('target-b')],
      );

      expect(recipientPolicyMessage(form, _bootstrap()), isNull);
    });

    test('blocks cross-directorate org unit for managers', () {
      final form = _validForm('sub-head-a').copyWith(
        recipients: [
          const DraftRecipient(
            type: DraftRecipientType.to,
            targetType: DraftRecipientTargetType.orgUnit,
            targetId: 'unit-b',
            label: 'Unit B',
          ),
        ],
      );

      expect(
        recipientPolicyMessage(form, _bootstrap()),
        contains('Penerima unit lintas direktorat'),
      );
    });

    test('filters staff recipient options to its own directorate', () {
      final bootstrap = _bootstrap();
      final state = DraftComposerState(
        bootstrap: bootstrap,
        drafts: const [],
        creatorPositionIds: const ['staff-a'],
        form: _validForm('staff-a'),
      );

      expect(
        state.availableRecipientPositions.map((item) => item.id),
        isNot(contains('target-b')),
      );
      expect(
        state.availableRecipientPositions.map((item) => item.id),
        contains('target-a'),
      );
    });
  });

  group('secretary on-behalf policy', () {
    test('accepts the direct GM manager', () {
      final form = _validForm('secretary-a').copyWith(
        onBehalfOfPositionId: 'gm-a',
      );

      expect(onBehalfPolicyMessage(form, _bootstrap()), isNull);
    });

    test('rejects a position other than the direct manager', () {
      final form = _validForm('secretary-a').copyWith(
        onBehalfOfPositionId: 'target-a',
      );

      expect(
        onBehalfPolicyMessage(form, _bootstrap()),
        contains('atasan langsung'),
      );
    });
  });
}

DraftComposerForm _validForm(String creatorPositionId) {
  return DraftComposerForm(
    companyId: 'company-1',
    letterTypeId: 'type-1',
    creatorPositionId: creatorPositionId,
    onBehalfOfPositionId: defaultOnBehalfPositionId(
      creatorPositionId,
      _bootstrap().positions,
    ),
    subject: 'Pengadaan',
    classification: LetterClassification.biasa,
    priority: LetterPriority.normal,
    bodyPlain: 'Mohon diproses.',
    recipients: [_recipient('target-a')],
  );
}

DraftComposerState _stateWith(DraftComposerForm form) {
  return DraftComposerState(
    bootstrap: _bootstrap(),
    drafts: const [],
    form: form,
    creatorPositionIds: const ['staff-a'],
  );
}

DraftLetter _draft({
  required String bodyHtml,
  required String bodyPlain,
}) {
  return DraftLetter(
    id: 'draft-1',
    companyId: 'company-1',
    companyCode: 'KSK',
    companyName: 'KSK Group',
    letterTypeId: 'type-1',
    letterTypeCode: 'ND',
    letterTypeName: 'Nota Dinas',
    subject: 'Pengadaan',
    classification: LetterClassification.biasa,
    priority: LetterPriority.normal,
    status: DraftLetterStatus.draft,
    creatorPositionId: 'staff-a',
    creatorPositionTitle: 'Staff A',
    version: 1,
    bodyHtml: bodyHtml,
    bodyPlain: bodyPlain,
    recipients: [_recipient('target-a')],
    createdAt: '2026-07-10T01:00:00Z',
    updatedAt: '2026-07-10T02:00:00Z',
  );
}

DraftRecipient _recipient(String targetId) {
  return DraftRecipient(
    type: DraftRecipientType.to,
    targetType: DraftRecipientTargetType.position,
    targetId: targetId,
    label: targetId,
  );
}

DraftComposerBootstrap _bootstrap() {
  return const DraftComposerBootstrap(
    companies: [
      DraftCompany(
        id: 'company-1',
        code: 'KSK',
        name: 'KSK Group',
        isActive: true,
      ),
    ],
    letterTypes: [
      DraftLetterType(
        id: 'type-1',
        code: 'ND',
        name: 'Nota Dinas',
        defaultClassification: LetterClassification.biasa,
        defaultSlaHours: 24,
        isActive: true,
      ),
    ],
    templates: [],
    positions: [
      DraftPosition(
        id: 'staff-a',
        title: 'Staff A',
        positionType: 'staff',
        isApprover: false,
        isActive: true,
        reportsToTitle: '',
        orgUnitId: 'unit-a',
        orgUnitName: 'Unit A',
        orgUnitLevel: 'division',
        holderName: 'Staff',
        holderUserId: 'user-staff',
        identityLocked: false,
      ),
      DraftPosition(
        id: 'sub-head-a',
        title: 'Sub Department Head A',
        positionType: 'sub_dept_head',
        isApprover: true,
        isActive: true,
        reportsToTitle: '',
        orgUnitId: 'unit-a',
        orgUnitName: 'Unit A',
        orgUnitLevel: 'division',
        holderName: 'Manager',
        holderUserId: 'user-manager',
        identityLocked: false,
      ),
      DraftPosition(
        id: 'secretary-a',
        title: 'Secretary A',
        positionType: 'secretary',
        isApprover: false,
        isActive: true,
        reportsTo: 'gm-a',
        reportsToTitle: 'GM A',
        orgUnitId: 'unit-a',
        orgUnitName: 'Unit A',
        orgUnitLevel: 'division',
        holderName: 'Secretary',
        holderUserId: 'user-secretary',
        identityLocked: false,
      ),
      DraftPosition(
        id: 'gm-a',
        title: 'GM A',
        positionType: 'gm',
        isApprover: true,
        isActive: true,
        reportsToTitle: '',
        orgUnitId: 'unit-a',
        orgUnitName: 'Unit A',
        orgUnitLevel: 'division',
        holderName: 'GM',
        holderUserId: 'user-gm',
        identityLocked: false,
      ),
      DraftPosition(
        id: 'target-a',
        title: 'Target A',
        positionType: 'staff',
        isApprover: false,
        isActive: true,
        reportsToTitle: '',
        orgUnitId: 'unit-a',
        orgUnitName: 'Unit A',
        orgUnitLevel: 'division',
        holderName: 'Target A',
        holderUserId: 'user-target-a',
        identityLocked: false,
      ),
      DraftPosition(
        id: 'target-b',
        title: 'Target B',
        positionType: 'staff',
        isApprover: false,
        isActive: true,
        reportsToTitle: '',
        orgUnitId: 'unit-b',
        orgUnitName: 'Unit B',
        orgUnitLevel: 'division',
        holderName: 'Target B',
        holderUserId: 'user-target-b',
        identityLocked: false,
      ),
    ],
    orgUnits: [
      DraftOrgUnit(
        id: 'directorate-a',
        code: 'DIR-A',
        name: 'Directorate A',
        unitLevel: 'directorate',
        isActive: true,
      ),
      DraftOrgUnit(
        id: 'unit-a',
        parentId: 'directorate-a',
        code: 'UNIT-A',
        name: 'Unit A',
        unitLevel: 'division',
        isActive: true,
      ),
      DraftOrgUnit(
        id: 'directorate-b',
        code: 'DIR-B',
        name: 'Directorate B',
        unitLevel: 'directorate',
        isActive: true,
      ),
      DraftOrgUnit(
        id: 'unit-b',
        parentId: 'directorate-b',
        code: 'UNIT-B',
        name: 'Unit B',
        unitLevel: 'division',
        isActive: true,
      ),
    ],
  );
}
