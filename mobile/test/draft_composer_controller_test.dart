import 'dart:async';

import 'package:dio/dio.dart';
import 'package:eoffice_mobile/core/exceptions/app_exception.dart';
import 'package:eoffice_mobile/features/auth/domain/user.dart';
import 'package:eoffice_mobile/features/auth/presentation/auth_controller.dart';
import 'package:eoffice_mobile/features/letters/data/draft_repository.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_composer_state.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:eoffice_mobile/features/letters/domain/letter_body_codec.dart';
import 'package:eoffice_mobile/features/letters/presentation/compose_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'bootstrap becomes ready and a valid form autosaves exactly once',
    () async {
      final repository = _FakeDraftRepository();
      final container = _createContainer(
        repository: repository,
        autosaveDelay: const Duration(milliseconds: 10),
      );
      final subscription = container.listen(
        draftComposerControllerProvider,
        (_, __) {},
        fireImmediately: true,
      );
      addTearDown(() {
        subscription.close();
        container.dispose();
      });

      final initial =
          await container.read(draftComposerControllerProvider.future);

      expect(container.read(draftComposerControllerProvider).hasValue, isTrue);
      expect(repository.loadBootstrapCalls, 1);
      expect(repository.listDraftsCalls, 1);
      expect(initial.bootstrap.companies.single.id, 'company-1');
      expect(initial.form.creatorPositionId, 'creator-position');

      final controller = container.read(
        draftComposerControllerProvider.notifier,
      );
      controller.updateForm(_validForm(initial.form));

      await _waitUntil(() {
        final value =
            container.read(draftComposerControllerProvider).valueOrNull;
        return repository.saveDraftCalls == 1 &&
            value?.saveStatus == DraftComposerSaveStatus.saved;
      });

      final saved =
          container.read(draftComposerControllerProvider).requireValue;
      expect(repository.saveDraftCalls, 1);
      expect(repository.lastDraftId, isNull);
      expect(repository.lastPayload?.subject, 'Permohonan pengadaan');
      expect(saved.form.draftId, 'draft-created');
      expect(saved.form.version, 1);
      expect(saved.dirty, isFalse);
      expect(saved.errorMessage, isNull);

      await Future<void>.delayed(const Duration(milliseconds: 30));
      expect(repository.saveDraftCalls, 1);
    },
  );

  test('manual save error keeps the form dirty and exposes its message',
      () async {
    final repository = _FakeDraftRepository(
      saveError: const AppException('Server menolak draft.'),
    );
    final container = _createContainer(
      repository: repository,
      autosaveDelay: const Duration(hours: 1),
    );
    final subscription = container.listen(
      draftComposerControllerProvider,
      (_, __) {},
      fireImmediately: true,
    );
    addTearDown(() {
      subscription.close();
      container.dispose();
    });

    final initial =
        await container.read(draftComposerControllerProvider.future);
    final controller = container.read(
      draftComposerControllerProvider.notifier,
    );
    controller.updateForm(_validForm(initial.form));

    final draftId = await controller.saveDraft();

    final failed = container.read(draftComposerControllerProvider).requireValue;
    expect(draftId, isNull);
    expect(repository.saveDraftCalls, 1);
    expect(failed.dirty, isTrue);
    expect(failed.saveStatus, DraftComposerSaveStatus.failed);
    expect(failed.errorMessage, 'Server menolak draft.');
    expect(failed.form.draftId, isNull);
    expect(failed.form.version, 0);
  });

  test('edit made while draft list refreshes is preserved as dirty', () async {
    final repository = _FakeDraftRepository();
    final container = _createContainer(
      repository: repository,
      autosaveDelay: const Duration(hours: 1),
    );
    final subscription = container.listen(
      draftComposerControllerProvider,
      (_, __) {},
      fireImmediately: true,
    );
    addTearDown(() {
      subscription.close();
      container.dispose();
    });

    final initial =
        await container.read(draftComposerControllerProvider.future);
    final controller = container.read(draftComposerControllerProvider.notifier);
    controller.updateForm(_validForm(initial.form));

    final refresh = Completer<List<DraftLetter>>();
    repository.nextListDraftsCompleter = refresh;
    final save = controller.saveDraft();
    await _waitUntil(
      () => repository.saveDraftCalls == 1 && repository.listDraftsCalls == 2,
    );

    controller.editForm(
      (form) => form.copyWith(subject: 'Perihal terbaru saat refresh'),
    );
    refresh.complete(const []);
    await save;

    final state = container.read(draftComposerControllerProvider).requireValue;
    expect(state.form.subject, 'Perihal terbaru saat refresh');
    expect(state.form.draftId, 'draft-created');
    expect(state.form.version, 1);
    expect(state.dirty, isTrue);
    expect(state.saveStatus, DraftComposerSaveStatus.saved);
  });

  test('committed submit stays successful when draft refresh fails', () async {
    final repository = _FakeDraftRepository();
    final container = _createContainer(
      repository: repository,
      autosaveDelay: const Duration(hours: 1),
    );
    final subscription = container.listen(
      draftComposerControllerProvider,
      (_, __) {},
      fireImmediately: true,
    );
    addTearDown(() {
      subscription.close();
      container.dispose();
    });

    final initial =
        await container.read(draftComposerControllerProvider.future);
    final controller = container.read(draftComposerControllerProvider.notifier);
    controller.updateForm(_validForm(initial.form));
    expect(await controller.saveDraft(), 'draft-created');

    repository.failNextListDrafts = true;
    final result = await controller.submit();

    final state = container.read(draftComposerControllerProvider).requireValue;
    expect(result?.id, 'draft-created');
    expect(repository.submitDraftCalls, 1);
    expect(state.form.draftId, isNull);
    expect(state.dirty, isFalse);
    expect(state.submitting, isFalse);
    expect(state.message, contains('berhasil diajukan'));
    expect(state.errorMessage, contains('sudah diajukan'));
  });

  test('applying a template seeds normalized editable HTML', () async {
    final repository = _FakeDraftRepository();
    final container = _createContainer(
      repository: repository,
      autosaveDelay: const Duration(hours: 1),
    );
    final subscription = container.listen(
      draftComposerControllerProvider,
      (_, __) {},
      fireImmediately: true,
    );
    addTearDown(() {
      subscription.close();
      container.dispose();
    });

    await container.read(draftComposerControllerProvider.future);
    final controller = container.read(draftComposerControllerProvider.notifier);

    controller.applyTemplate('template-simple');
    var form =
        container.read(draftComposerControllerProvider).requireValue.form;
    expect(form.bodyReadOnly, isFalse);
    expect(
      form.bodyHtml,
      '<p>Dengan hormat,</p><p><strong>Mohon persetujuan.</strong></p>',
    );
    expect(form.sourceBodyEditableHtml, form.bodyHtml);
    expect(form.bodyPlain, 'Dengan hormat,\nMohon persetujuan.');

    controller.applyTemplate('template-table');
    form = container.read(draftComposerControllerProvider).requireValue.form;
    expect(form.bodyReadOnly, isTrue);
    expect(form.bodyHtml, isEmpty);
    expect(form.sourceBodyHtml, contains('<table>'));
  });

  test('simplifying a locked body seeds plain-paragraph HTML', () async {
    final repository = _FakeDraftRepository();
    final container = _createContainer(
      repository: repository,
      autosaveDelay: const Duration(hours: 1),
    );
    final subscription = container.listen(
      draftComposerControllerProvider,
      (_, __) {},
      fireImmediately: true,
    );
    addTearDown(() {
      subscription.close();
      container.dispose();
    });

    await container.read(draftComposerControllerProvider.future);
    final controller = container.read(draftComposerControllerProvider.notifier);

    controller.applyTemplate('template-table');
    controller.simplifyBodyFormatting();

    final form =
        container.read(draftComposerControllerProvider).requireValue.form;
    expect(form.bodyReadOnly, isFalse);
    expect(form.sourceBodyHtml, isEmpty);
    expect(form.bodyHtml, plainTextToLetterHtml(form.bodyPlain));
  });
}

ProviderContainer _createContainer({
  required _FakeDraftRepository repository,
  required Duration autosaveDelay,
}) {
  return ProviderContainer(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(_creatorUser),
      ),
      draftRepositoryProvider.overrideWithValue(repository),
      draftAutosaveDelayProvider.overrideWithValue(autosaveDelay),
    ],
  );
}

DraftComposerForm _validForm(DraftComposerForm form) {
  return form.copyWith(
    subject: 'Permohonan pengadaan',
    bodyPlain: 'Mohon persetujuan pengadaan perangkat.',
    recipients: const [
      DraftRecipient(
        type: DraftRecipientType.to,
        targetType: DraftRecipientTargetType.position,
        targetId: 'recipient-position',
        label: 'General Manager - Direktorat Operasional',
      ),
    ],
  );
}

Future<void> _waitUntil(
  bool Function() condition, {
  Duration timeout = const Duration(seconds: 2),
}) async {
  final stopwatch = Stopwatch()..start();
  while (!condition()) {
    if (stopwatch.elapsed >= timeout) {
      fail('Timed out while waiting for the controller state.');
    }
    await Future<void>.delayed(const Duration(milliseconds: 5));
  }
}

const _creatorUser = User(
  id: 'user-1',
  email: 'creator@example.com',
  fullName: 'Creator',
  roles: ['creator'],
  positions: [
    UserPosition(
      positionId: 'creator-position',
      title: 'Staff Operasional',
      positionType: 'staff',
      orgUnit: 'Direktorat Operasional',
      assignmentType: 'definitive',
      companyId: 'company-1',
      companyCode: 'KSK',
      companyName: 'KSK Group',
    ),
  ],
);

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._user);

  final User _user;

  @override
  Future<AuthState> build() async => AuthState(user: _user);
}

class _FakeDraftRepository extends DraftRepository {
  _FakeDraftRepository({this.saveError}) : super(Dio());

  final Object? saveError;
  var loadBootstrapCalls = 0;
  var listDraftsCalls = 0;
  var saveDraftCalls = 0;
  var submitDraftCalls = 0;
  var failNextListDrafts = false;
  String? lastDraftId;
  DraftLetterPayload? lastPayload;
  Completer<List<DraftLetter>>? nextListDraftsCompleter;

  @override
  Future<DraftComposerBootstrap> loadBootstrap() async {
    loadBootstrapCalls++;
    return _bootstrap;
  }

  @override
  Future<List<DraftLetter>> listDrafts() async {
    listDraftsCalls++;
    if (failNextListDrafts) {
      failNextListDrafts = false;
      throw const AppException('refresh gagal');
    }
    final completer = nextListDraftsCompleter;
    if (completer != null) {
      nextListDraftsCompleter = null;
      return completer.future;
    }
    return const [];
  }

  @override
  Future<DraftSaveResult> saveDraft({
    required DraftLetterPayload payload,
    String? draftId,
  }) async {
    saveDraftCalls++;
    lastDraftId = draftId;
    lastPayload = payload;
    if (saveError case final error?) throw error;
    return const DraftSaveResult(id: 'draft-created', version: 1);
  }

  @override
  Future<DraftSubmitResult> submitDraft(String draftId) async {
    submitDraftCalls++;
    return DraftSubmitResult(
      id: draftId,
      status: DraftLetterStatus.inApproval,
      approvalCycle: 1,
      qrToken: 'qr-token',
      verifyUrl: 'https://example.test/verify',
      approvalSteps: const [
        DraftSubmitApprovalStep(
          stepOrder: 1,
          flowGroup: 1,
          positionId: 'approver-position',
          positionType: 'gm',
          title: 'General Manager',
        ),
      ],
    );
  }
}

const _bootstrap = DraftComposerBootstrap(
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
  templates: [
    DraftLetterTemplate(
      id: 'template-simple',
      letterTypeId: 'type-1',
      letterTypeCode: 'ND',
      letterTypeName: 'Nota Dinas',
      companyId: 'company-1',
      companyCode: 'KSK',
      companyName: 'KSK Group',
      version: 1,
      layoutConfig: {},
      bodySkeleton: '<p>Yth. {{tujuan}}</p>'
          '<div>Dengan hormat,</div>'
          '<p><b>Mohon persetujuan.</b></p>',
      isActive: true,
      createdAt: '2026-07-01T00:00:00Z',
    ),
    DraftLetterTemplate(
      id: 'template-table',
      letterTypeId: 'type-1',
      letterTypeCode: 'ND',
      letterTypeName: 'Nota Dinas',
      companyId: 'company-1',
      companyCode: 'KSK',
      companyName: 'KSK Group',
      version: 1,
      layoutConfig: {},
      bodySkeleton: '<p>Rincian:</p>'
          '<table><tr><td>Item</td><td>Jumlah</td></tr></table>',
      isActive: true,
      createdAt: '2026-07-01T00:00:00Z',
    ),
  ],
  positions: [
    DraftPosition(
      id: 'creator-position',
      title: 'Staff Operasional',
      positionType: 'staff',
      isApprover: false,
      isActive: true,
      reportsToTitle: '',
      orgUnitId: 'directorate-1',
      orgUnitName: 'Direktorat Operasional',
      orgUnitLevel: 'directorate',
      holderName: 'Creator',
      holderUserId: 'user-1',
      identityLocked: true,
    ),
    DraftPosition(
      id: 'recipient-position',
      title: 'General Manager',
      positionType: 'gm',
      isApprover: true,
      isActive: true,
      reportsToTitle: '',
      orgUnitId: 'directorate-1',
      orgUnitName: 'Direktorat Operasional',
      orgUnitLevel: 'directorate',
      holderName: 'Recipient',
      holderUserId: 'user-2',
      identityLocked: true,
    ),
  ],
  orgUnits: [
    DraftOrgUnit(
      id: 'directorate-1',
      code: 'OPS',
      name: 'Direktorat Operasional',
      unitLevel: 'directorate',
      isActive: true,
    ),
  ],
);
