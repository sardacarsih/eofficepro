import 'package:eoffice_mobile/features/auth/presentation/change_password_page.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('change password validators', () {
    test('requires the current password', () {
      expect(validateCurrentPassword(null), isNotNull);
      expect(validateCurrentPassword(''), isNotNull);
      expect(validateCurrentPassword('password-lama'), isNull);
    });

    test('requires a new password of at least ten characters', () {
      expect(validateChangedPassword('pendek', 'password-lama'), isNotNull);
      expect(
        validateChangedPassword('password-lama', 'password-lama'),
        isNotNull,
      );
      expect(
        validateChangedPassword('password-baru', 'password-lama'),
        isNull,
      );
    });

    test('requires matching confirmation', () {
      expect(
        validatePasswordConfirmation('berbeda', 'password-baru'),
        isNotNull,
      );
      expect(
        validatePasswordConfirmation('password-baru', 'password-baru'),
        isNull,
      );
    });
  });
}
