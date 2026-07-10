import 'package:eoffice_mobile/features/letters/domain/letter_body_codec.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('letterHtmlToPlainText', () {
    test('removes the template recipient paragraph and placeholders', () {
      const source = '''
        <p>Yth. <strong>{{tujuan}}</strong></p>
        <p>Halo <strong>Tim</strong>,</p>
        <p>Mohon diproses untuk {{tujuan}} hari ini.</p>
      ''';

      expect(
        letterHtmlToPlainText(source),
        'Halo Tim,\nMohon diproses untuk hari ini.',
      );
    });

    test('preserves paragraph and explicit line breaks', () {
      const source = '<p>Baris pertama<br>Baris kedua</p><p>Paragraf baru</p>';

      expect(
        letterHtmlToPlainText(source),
        'Baris pertama\nBaris kedua\nParagraf baru',
      );
    });

    test('ignores executable and styling elements', () {
      const source = '<p>Aman</p><script>alert(1)</script><style>p{}</style>';

      expect(letterHtmlToPlainText(source), 'Aman');
    });

    test('flattens list items onto separate lines', () {
      const source = '<p>Agenda:</p><ul><li>Satu</li><li>Dua</li></ul>';

      expect(letterHtmlToPlainText(source), 'Agenda:\nSatu\nDua');
    });

    test('returns empty text for empty HTML', () {
      expect(letterHtmlToPlainText('<p><br></p>'), isEmpty);
    });
  });

  group('plainTextToLetterHtml', () {
    test('escapes unsafe text and preserves lines', () {
      expect(
        plainTextToLetterHtml('Anggaran < 10 & > 2\n\nSetuju'),
        '<p>Anggaran &lt; 10 &amp; &gt; 2</p>'
        '<p><br></p>'
        '<p>Setuju</p>',
      );
    });

    test('round-trips plain text without template placeholders', () {
      const source = 'Pembuka\nIsi surat';

      expect(letterHtmlToPlainText(plainTextToLetterHtml(source)), source);
    });

    test('returns empty HTML for whitespace-only text', () {
      expect(plainTextToLetterHtml('  \n  '), isEmpty);
    });
  });

  group('hasUnsupportedLetterFormatting', () {
    test('accepts the natively editable subset', () {
      const source = '<p>Halo <strong>Tim</strong> dan <em>rekan</em>,</p>'
          '<p><u>Penting</u></p>'
          '<ul><li>Satu</li><li>Dua</li></ul>'
          '<ol><li>Pertama</li></ol>'
          '<p><br></p>';

      expect(hasUnsupportedLetterFormatting(source), isFalse);
    });

    test('accepts legacy aliases b, i, and div', () {
      expect(
        hasUnsupportedLetterFormatting('<div><b>Tebal</b> <i>miring</i></div>'),
        isFalse,
      );
    });

    test('rejects tables', () {
      expect(
        hasUnsupportedLetterFormatting('<table><tr><td>a</td></tr></table>'),
        isTrue,
      );
    });

    test('rejects any attribute', () {
      expect(
        hasUnsupportedLetterFormatting('<p style="color:red">a</p>'),
        isTrue,
      );
    });

    test('rejects nested lists', () {
      expect(
        hasUnsupportedLetterFormatting(
          '<ul><li>a<ul><li>b</li></ul></li></ul>',
        ),
        isTrue,
      );
    });

    test('rejects headings and links', () {
      expect(hasUnsupportedLetterFormatting('<h1>Judul</h1>'), isTrue);
      expect(
        hasUnsupportedLetterFormatting('<p><a href="#">tautan</a></p>'),
        isTrue,
      );
    });

    test('returns false for empty HTML', () {
      expect(hasUnsupportedLetterFormatting(''), isFalse);
      expect(hasUnsupportedLetterFormatting('  '), isFalse);
    });
  });

  // These golden cases must stay byte-identical to the Kotlin serializer
  // goldens in
  // mobile/android/app/src/test/kotlin/com/kskgroup/eoffice_mobile/richtext/LetterHtmlCodecTest.kt
  // so that "unedited body" detection by string equality keeps working.
  group('normalizeEditableLetterHtml', () {
    test('strips the recipient paragraph and inline placeholders', () {
      const source = '''
        <p>Yth. <strong>{{tujuan}}</strong></p>
        <p>Halo <strong>Tim</strong>,</p>
        <p>Mohon diproses untuk {{tujuan}} hari ini.</p>
      ''';

      expect(
        normalizeEditableLetterHtml(source),
        '<p>Halo <strong>Tim</strong>,</p>'
        '<p>Mohon diproses untuk hari ini.</p>',
      );
    });

    test('maps legacy aliases onto canonical tags', () {
      expect(
        normalizeEditableLetterHtml('<div>a <b>b</b> <i>c</i></div>'),
        '<p>a <strong>b</strong> <em>c</em></p>',
      );
    });

    test('splits explicit line breaks into paragraphs', () {
      expect(
        normalizeEditableLetterHtml('<p>a<br>b</p>'),
        '<p>a</p><p>b</p>',
      );
      expect(normalizeEditableLetterHtml('<p>a<br></p>'), '<p>a</p>');
      expect(normalizeEditableLetterHtml('<p><br></p>'), '<p><br></p>');
    });

    test('keeps blank paragraphs between content', () {
      expect(
        normalizeEditableLetterHtml('<p>a</p><p></p><p>b</p>'),
        '<p>a</p><p><br></p><p>b</p>',
      );
    });

    test('orders inline nesting as strong, em, u', () {
      expect(
        normalizeEditableLetterHtml('<p><u><em><strong>a</strong></em></u></p>'),
        '<p><strong><em><u>a</u></em></strong></p>',
      );
    });

    test('merges adjacent runs with identical formatting', () {
      expect(
        normalizeEditableLetterHtml('<p><strong>a</strong><strong>b</strong></p>'),
        '<p><strong>ab</strong></p>',
      );
    });

    test('serializes lists canonically and merges adjacent lists', () {
      expect(
        normalizeEditableLetterHtml(
          '<ul><li>Satu</li></ul><ul><li><strong>Dua</strong></li></ul>'
          '<ol><li>Pertama</li></ol>',
        ),
        '<ul><li>Satu</li><li><strong>Dua</strong></li></ul>'
        '<ol><li>Pertama</li></ol>',
      );
    });

    test('collapses whitespace inside blocks', () {
      expect(
        normalizeEditableLetterHtml('<p>  a \n  b  </p>'),
        '<p>a b</p>',
      );
    });

    test('escapes text content', () {
      expect(
        normalizeEditableLetterHtml('<p>1 &lt; 2 &amp; 3</p>'),
        '<p>1 &lt; 2 &amp; 3</p>',
      );
    });

    test('wraps stray top-level text in a paragraph', () {
      expect(
        normalizeEditableLetterHtml('<p>a</p>halo<p>b</p>'),
        '<p>a</p><p>halo</p><p>b</p>',
      );
    });

    test('matches plainTextToLetterHtml for plain input', () {
      const plain = 'Pembuka\n\nIsi surat';

      expect(
        normalizeEditableLetterHtml(plainTextToLetterHtml(plain)),
        plainTextToLetterHtml(plain),
      );
    });

    test('is idempotent over representative inputs', () {
      const sources = [
        '<p>Halo <strong>Tim</strong>,</p><p><br></p><p>Salam</p>',
        '<div>a <b>b</b> <i>c</i><br><u>d</u></div>',
        '<ul><li>Satu</li><li><em>Dua</em></li></ul><p>Penutup</p>',
        '<p><u><em><strong>semua</strong></em></u> gaya</p>',
        '<ol><li><br></li><li>Isi</li></ol>',
      ];

      for (final source in sources) {
        final once = normalizeEditableLetterHtml(source);
        expect(normalizeEditableLetterHtml(once), once, reason: source);
      }
    });

    test('returns empty for empty or recipient-only HTML', () {
      expect(normalizeEditableLetterHtml(''), isEmpty);
      expect(
        normalizeEditableLetterHtml('<p>Yth. {{tujuan}}</p>'),
        isEmpty,
      );
    });
  });
}
