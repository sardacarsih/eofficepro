import 'dart:async';

import 'package:eoffice_mobile/features/letters/domain/draft_composer_state.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:eoffice_mobile/features/letters/presentation/compose_controller.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/attachment_section.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/compose_form_fields.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/compose_support.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/draft_list_panel.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/recipient_picker_sheet.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('form has no overflow at 320dp and 200% text', (tester) async {
    _setViewport(tester);
    final subjectController = TextEditingController(text: _form.subject);
    addTearDown(subjectController.dispose);

    await tester.pumpWidget(
      _app(
        SingleChildScrollView(
          padding: const EdgeInsets.all(20),
          child: ComposeFormFields(
            state: _state,
            controller: DraftComposerController(),
            subjectController: subjectController,
          ),
        ),
      ),
    );

    _expectNoException(tester);
  });

  testWidgets('idle attachment section fits at 320dp and 200% text', (
    tester,
  ) async {
    _setViewport(tester);

    await tester.pumpWidget(
      _app(
        const SingleChildScrollView(
          padding: EdgeInsets.all(20),
          child: AttachmentSection(
            state: _state,
            onPick: _noop,
            onOpen: _noopAttachment,
            onDelete: _noopAttachment,
          ),
        ),
      ),
    );

    expect(find.byTooltip('Tambah lampiran'), findsOneWidget);
    _expectNoException(tester);
  });

  testWidgets('uploading attachment section fits at 320dp and 200% text', (
    tester,
  ) async {
    _setViewport(tester);

    await tester.pumpWidget(
      _app(
        const SingleChildScrollView(
          padding: EdgeInsets.all(20),
          child: AttachmentSection(
            state: _uploadingState,
            onPick: _noop,
            onOpen: _noopAttachment,
            onDelete: _noopAttachment,
          ),
        ),
      ),
    );

    expect(find.byTooltip('Sedang mengunggah lampiran'), findsOneWidget);
    _expectNoException(tester);
  });

  testWidgets('draft list fits at 340dp and 200% text', (tester) async {
    _setViewport(tester, width: 340);

    await tester.pumpWidget(
      _app(
        SizedBox(
          height: 800,
          child: DraftListPanel(
            state: _draftState,
            onNewDraft: _noop,
            onOpenDraft: (_) {},
          ),
        ),
        width: 340,
      ),
    );

    _expectNoException(tester);
  });

  testWidgets('recipient picker fits with keyboard and 200% text', (
    tester,
  ) async {
    _setViewport(tester);
    tester.view.viewInsets = const FakeViewPadding(bottom: 400);
    addTearDown(tester.view.resetViewInsets);
    late BuildContext context;

    await tester.pumpWidget(
      _app(
        Builder(
          builder: (value) {
            context = value;
            return const SizedBox();
          },
        ),
      ),
    );
    unawaited(showRecipientPickerSheet(context, _state));
    await tester.pumpAndSettle();

    expect(find.byType(TextField), findsOneWidget);
    expect(
        MediaQuery.viewInsetsOf(tester.element(find.byType(TextField))).bottom,
        greaterThan(0));
    _expectNoException(tester);
  });

  testWidgets('feedback dismiss exposes a semantic tap action', (tester) async {
    await tester.pumpWidget(
      _app(
        const ComposerFeedbackNotice(
          message: 'Selesai',
          error: false,
          onDismiss: _noop,
        ),
      ),
    );

    final node = tester.getSemantics(find.byTooltip('Tutup pesan'));
    expect(node.getSemanticsData().hasAction(SemanticsAction.tap), isTrue);
  });

  testWidgets('draft item exposes a semantic tap action', (tester) async {
    await tester.pumpWidget(
      _app(
        SizedBox(
          height: 800,
          child: DraftListPanel(
            state: _draftState,
            onNewDraft: _noop,
            onOpenDraft: (_) {},
          ),
        ),
      ),
    );

    final node = tester.getSemantics(
      find.bySemanticsLabel(RegExp('Perihal surat yang sangat panjang')),
    );
    expect(node.getSemanticsData().hasAction(SemanticsAction.tap), isTrue);
  });

  testWidgets('recipient item exposes a semantic tap action', (tester) async {
    late BuildContext context;
    await tester.pumpWidget(
      _app(
        Builder(
          builder: (value) {
            context = value;
            return const SizedBox();
          },
        ),
      ),
    );
    unawaited(showRecipientPickerSheet(context, _state));
    await tester.pumpAndSettle();

    final node = tester.getSemantics(
      find.bySemanticsLabel(RegExp('Manager Tujuan')),
    );
    expect(node.getSemanticsData().hasAction(SemanticsAction.tap), isTrue);
  });
}

void _setViewport(WidgetTester tester, {double width = 320}) {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, 800);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
}

Widget _app(Widget child, {double width = 320}) {
  return MaterialApp(
    home: MediaQuery(
      data: MediaQueryData(
        size: Size(width, 800),
        textScaler: const TextScaler.linear(2),
      ),
      child: Scaffold(body: child),
    ),
  );
}

void _expectNoException(WidgetTester tester) {
  expect(tester.takeException(), isNull);
}

void _noop() {}

void _noopAttachment(DraftAttachment _) {}

const _company = DraftCompany(
  id: 'company-1',
  code: 'CMP',
  name: 'Perusahaan Panjang',
  isActive: true,
);

const _letterType = DraftLetterType(
  id: 'type-1',
  code: 'MEMO',
  name: 'Memorandum Panjang',
  defaultClassification: LetterClassification.biasa,
  defaultSlaHours: 24,
  isActive: true,
);

const _creatorPosition = DraftPosition(
  id: 'position-1',
  title: 'Manager Operasional Panjang',
  positionType: 'manager',
  isApprover: false,
  isActive: true,
  reportsToTitle: '',
  orgUnitId: 'unit-1',
  orgUnitName: 'Departemen Operasional Panjang',
  orgUnitLevel: 'department',
  holderName: 'Pengguna',
  holderUserId: 'user-1',
  identityLocked: false,
);

const _targetPosition = DraftPosition(
  id: 'position-2',
  title: 'Manager Tujuan',
  positionType: 'manager',
  isApprover: false,
  isActive: true,
  reportsToTitle: '',
  orgUnitId: 'unit-1',
  orgUnitName: 'Departemen Tujuan',
  orgUnitLevel: 'department',
  holderName: 'Penerima',
  holderUserId: 'user-2',
  identityLocked: false,
);

const _orgUnit = DraftOrgUnit(
  id: 'unit-1',
  code: 'UNIT',
  name: 'Departemen Operasional Panjang',
  unitLevel: 'directorate',
  isActive: true,
);

const _bootstrap = DraftComposerBootstrap(
  companies: [_company],
  letterTypes: [_letterType],
  templates: [],
  positions: [_creatorPosition, _targetPosition],
  orgUnits: [_orgUnit],
);

const _form = DraftComposerForm(
  companyId: 'company-1',
  letterTypeId: 'type-1',
  creatorPositionId: 'position-1',
  subject: 'Perihal surat yang cukup panjang',
  classification: LetterClassification.biasa,
  priority: LetterPriority.normal,
  bodyPlain: 'Isi surat',
  recipients: [
    DraftRecipient(
      type: DraftRecipientType.to,
      targetType: DraftRecipientTargetType.position,
      targetId: 'position-1',
      label: 'Manager Operasional Panjang',
    ),
  ],
);

const _state = DraftComposerState(
  bootstrap: _bootstrap,
  drafts: [],
  form: _form,
  creatorPositionIds: ['position-1'],
);

const _uploadingState = DraftComposerState(
  bootstrap: _bootstrap,
  drafts: [],
  form: _form,
  creatorPositionIds: ['position-1'],
  uploadingAttachment: true,
);

const _draftState = DraftComposerState(
  bootstrap: _bootstrap,
  drafts: [
    DraftLetter(
      id: 'draft-1',
      companyId: 'company-1',
      companyCode: 'CMP',
      companyName: 'Perusahaan Panjang',
      letterTypeId: 'type-1',
      letterTypeCode: 'MEMO',
      letterTypeName: 'Memorandum Panjang',
      subject: 'Perihal surat yang sangat panjang untuk pengujian',
      classification: LetterClassification.biasa,
      priority: LetterPriority.normal,
      status: DraftLetterStatus.draft,
      creatorPositionId: 'position-1',
      creatorPositionTitle: 'Manager Operasional Panjang',
      version: 12,
      bodyHtml: '<p>Isi</p>',
      bodyPlain: 'Isi',
      recipients: [],
      createdAt: '2026-07-10T10:00:00Z',
      updatedAt: '2026-07-10T10:00:00Z',
    ),
  ],
  form: _form,
  creatorPositionIds: ['position-1'],
);
