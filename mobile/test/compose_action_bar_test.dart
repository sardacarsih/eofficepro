import 'package:eoffice_mobile/features/letters/domain/draft_composer_state.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/compose_action_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final width in [320.0, 600.0, 1000.0]) {
    testWidgets(
      'action bar has no overflow at width $width and 200% text',
      (tester) async {
        tester.view.devicePixelRatio = 1;
        tester.view.physicalSize = Size(width, 800);
        addTearDown(tester.view.resetDevicePixelRatio);
        addTearDown(tester.view.resetPhysicalSize);

        await tester.pumpWidget(
          MaterialApp(
            home: MediaQuery(
              data: MediaQueryData(
                size: Size(width, 800),
                textScaler: const TextScaler.linear(2),
              ),
              child: Scaffold(
                bottomNavigationBar: ComposeActionBar(
                  state: _state,
                  onSave: () {},
                  onPreview: () {},
                  onSubmit: () {},
                ),
              ),
            ),
          ),
        );

        expect(tester.takeException(), isNull);
        if (width < 832) {
          expect(find.byTooltip('Simpan draft'), findsOneWidget);
          expect(find.byTooltip('Preview PDF'), findsOneWidget);
          expect(find.byTooltip('Ajukan surat'), findsOneWidget);
        } else {
          expect(find.text('Simpan Draft'), findsOneWidget);
          expect(find.text('Preview'), findsOneWidget);
          expect(find.text('Ajukan'), findsOneWidget);
        }
      },
    );
  }
}

const _state = DraftComposerState(
  bootstrap: DraftComposerBootstrap(
    companies: [],
    letterTypes: [],
    templates: [],
    positions: [],
    orgUnits: [],
  ),
  drafts: [],
  creatorPositionIds: ['position-1'],
  form: DraftComposerForm(
    companyId: 'company-1',
    letterTypeId: 'type-1',
    creatorPositionId: 'position-1',
    subject: 'Perihal',
    classification: LetterClassification.biasa,
    priority: LetterPriority.normal,
    bodyPlain: 'Isi',
    recipients: [],
  ),
  dirty: true,
);
