import 'package:eoffice_mobile/features/letters/presentation/widgets/letter_body_editor.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/native_rich_text_view.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/rich_text_toolbar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('LetterBodyEditor fallback', () {
    testWidgets('emits canonical HTML alongside the plain text',
        (tester) async {
      final changes = <(String, String)>[];
      await tester.pumpWidget(
        _formApp(
          LetterBodyEditor(
            value: '',
            plainValue: '',
            enabled: true,
            readOnly: false,
            useNativeEditor: false,
            onChanged: (html, plain) => changes.add((html, plain)),
            onSimplifyRequested: () {},
          ),
        ),
      );

      await tester.enterText(
        find.byType(TextFormField),
        'Baris satu\nBaris dua',
      );

      expect(changes, isNotEmpty);
      expect(changes.last.$1, '<p>Baris satu</p><p>Baris dua</p>');
      expect(changes.last.$2, 'Baris satu\nBaris dua');
    });

    testWidgets('reseeds the field when the value changes externally',
        (tester) async {
      await tester.pumpWidget(
        _formApp(
          LetterBodyEditor(
            value: '<p>Lama</p>',
            plainValue: 'Lama',
            enabled: true,
            readOnly: false,
            useNativeEditor: false,
            onChanged: (_, __) {},
            onSimplifyRequested: () {},
          ),
        ),
      );
      expect(find.text('Lama'), findsOneWidget);

      await tester.pumpWidget(
        _app(
          LetterBodyEditor(
            value: '<p>Baru</p>',
            plainValue: 'Baru',
            enabled: true,
            readOnly: false,
            useNativeEditor: false,
            onChanged: (_, __) {},
            onSimplifyRequested: () {},
          ),
        ),
      );

      expect(find.text('Baru'), findsOneWidget);
      expect(find.text('Lama'), findsNothing);
      expect(tester.takeException(), isNull);
    });

    testWidgets('does not render the formatting toolbar', (tester) async {
      await tester.pumpWidget(
        _app(
          LetterBodyEditor(
            value: '',
            plainValue: '',
            enabled: true,
            readOnly: false,
            useNativeEditor: false,
            onChanged: (_, __) {},
            onSimplifyRequested: () {},
          ),
        ),
      );

      expect(find.byType(RichTextToolbar), findsNothing);
    });
  });

  group('LetterBodyEditor locked', () {
    testWidgets('shows the preserved body read-only with a lock affordance',
        (tester) async {
      var simplifyRequests = 0;
      await tester.pumpWidget(
        _app(
          LetterBodyEditor(
            value: '',
            plainValue: 'Isi berformat',
            enabled: true,
            readOnly: true,
            onChanged: (_, __) {},
            onSimplifyRequested: () => simplifyRequests++,
          ),
        ),
      );

      expect(find.text('Isi berformat'), findsOneWidget);
      expect(
        find.text('Isi berformat lanjutan dipertahankan tanpa perubahan.'),
        findsOneWidget,
      );
      expect(find.byType(RichTextToolbar), findsNothing);

      await tester.enterText(find.byType(TextFormField), 'Coba ubah');
      expect(find.text('Isi berformat'), findsOneWidget);

      await tester.tap(find.byTooltip('Edit sebagai teks biasa'));
      expect(simplifyRequests, 1);
    });

    testWidgets('disables the lock action while the form is busy',
        (tester) async {
      var simplifyRequests = 0;
      await tester.pumpWidget(
        _app(
          LetterBodyEditor(
            value: '',
            plainValue: 'Isi berformat',
            enabled: false,
            readOnly: true,
            onChanged: (_, __) {},
            onSimplifyRequested: () => simplifyRequests++,
          ),
        ),
      );

      await tester.tap(
        find.byTooltip('Edit sebagai teks biasa'),
        warnIfMissed: false,
      );
      expect(simplifyRequests, 0);
    });
  });

  group('RichTextToolbar', () {
    testWidgets('sends format commands and reflects active formats',
        (tester) async {
      final commands = <String>[];
      await tester.pumpWidget(
        _app(
          RichTextToolbar(
            enabled: true,
            formatState: const RichTextFormatState(bold: true),
            onFormat: commands.add,
          ),
        ),
      );

      await tester.tap(find.byTooltip('Tebal'));
      await tester.tap(find.byTooltip('Daftar poin'));
      expect(commands, ['bold', 'bulletList']);

      final boldButton = tester.widget<IconButton>(
        find.ancestor(
          of: find.byIcon(Icons.format_bold),
          matching: find.byType(IconButton),
        ),
      );
      expect(boldButton.isSelected, isTrue);
    });

    testWidgets('disables all buttons when the editor is locked',
        (tester) async {
      final commands = <String>[];
      await tester.pumpWidget(
        _app(
          RichTextToolbar(
            enabled: false,
            formatState: const RichTextFormatState(),
            onFormat: commands.add,
          ),
        ),
      );

      await tester.tap(find.byTooltip('Miring'), warnIfMissed: false);
      expect(commands, isEmpty);
    });
  });
}

Widget _app(Widget child) {
  return MaterialApp(
    home: Scaffold(
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: child,
      ),
    ),
  );
}

Widget _formApp(Widget child) {
  return _app(
    Form(
      child: LayoutBuilder(
        builder: (context, constraints) => child,
      ),
    ),
  );
}
