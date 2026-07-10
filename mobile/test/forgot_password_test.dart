import 'package:eoffice_mobile/features/auth/presentation/forgot_password_page.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('validateResetEmail', () {
    test('menolak email kosong', () {
      expect(validateResetEmail(null), isNotNull);
      expect(validateResetEmail(''), isNotNull);
      expect(validateResetEmail('   '), isNotNull);
    });

    test('menolak format tanpa @', () {
      expect(validateResetEmail('bukan-email'), isNotNull);
    });

    test('menerima email valid', () {
      expect(validateResetEmail('user@example.com'), isNull);
      expect(validateResetEmail('  user@example.com  '), isNull);
    });
  });

  group('validateResetCode', () {
    test('menolak kode kosong atau kurang dari 6 digit', () {
      expect(validateResetCode(null), isNotNull);
      expect(validateResetCode(''), isNotNull);
      expect(validateResetCode('12345'), isNotNull);
    });

    test('menolak kode lebih dari 6 digit atau non-digit', () {
      expect(validateResetCode('1234567'), isNotNull);
      expect(validateResetCode('12a456'), isNotNull);
    });

    test('menerima kode 6 digit', () {
      expect(validateResetCode('123456'), isNull);
      expect(validateResetCode('000000'), isNull);
    });
  });

  group('validateNewPassword', () {
    test('menolak password kurang dari 10 karakter', () {
      expect(validateNewPassword(null), isNotNull);
      expect(validateNewPassword('pendek123'), isNotNull);
    });

    test('menerima password 10 karakter atau lebih', () {
      expect(validateNewPassword('cukupPanjang1'), isNull);
      expect(validateNewPassword('1234567890'), isNull);
    });
  });

  group('validateConfirmPassword', () {
    test('menolak konfirmasi yang berbeda', () {
      expect(validateConfirmPassword('beda', 'passwordBaru'), isNotNull);
      expect(validateConfirmPassword(null, 'passwordBaru'), isNotNull);
    });

    test('menerima konfirmasi yang sama', () {
      expect(validateConfirmPassword('passwordBaru', 'passwordBaru'), isNull);
    });
  });
}
