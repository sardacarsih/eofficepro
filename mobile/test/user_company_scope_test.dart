import 'package:eoffice_mobile/features/auth/domain/user.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('parses company access scope from current profile payload', () {
    final user = User.fromJson({
      'id': 'user-1',
      'email': 'admin@example.com',
      'full_name': 'Company Admin',
      'roles': ['admin'],
      'positions': [
        {
          'position_id': 'position-1',
          'title': 'Administrator',
          'position_type': 'staff',
          'org_unit': 'Head Office',
          'assignment_type': 'definitive',
          'company_id': 'company-1',
          'company_code': 'KSK',
          'company_name': 'KSK Group',
        },
      ],
      'accessible_companies': [
        {
          'id': 'company-1',
          'code': 'KSK',
          'name': 'KSK Group',
          'is_active': true,
        },
      ],
      'company_roles': [
        {
          'company_id': 'company-1',
          'company_code': 'KSK',
          'company_name': 'KSK Group',
          'role_code': 'admin',
          'valid_from': '2026-07-01',
          'valid_to': null,
        },
      ],
      'capabilities': {'is_super_admin': false},
    });

    expect(user.accessibleCompanies.single.id, 'company-1');
    expect(user.accessibleCompanies.single.isActive, isTrue);
    expect(user.companyRoles.single.roleCode, 'admin');
    expect(user.companyRoles.single.validFrom, '2026-07-01');
    expect(user.companyRoles.single.validTo, isNull);
    expect(user.positions.single.companyCode, 'KSK');
    expect(user.isSuperAdmin, isFalse);
  });

  test('parses super admin capability', () {
    final user = User.fromJson({
      'id': 'user-1',
      'capabilities': {'is_super_admin': true},
    });

    expect(user.isSuperAdmin, isTrue);
  });

  test('keeps old profile payload backward compatible', () {
    final user = User.fromJson({
      'id': 'user-1',
      'email': 'user@example.com',
      'full_name': 'User',
      'roles': ['creator'],
      'positions': const [],
    });

    expect(user.accessibleCompanies, isEmpty);
    expect(user.companyRoles, isEmpty);
    expect(user.isSuperAdmin, isFalse);
  });

  test('ignores invalid entries in optional company scope lists', () {
    final user = User.fromJson({
      'id': 'user-1',
      'accessible_companies': [null, 'company-1'],
      'company_roles': [false, 1],
    });

    expect(user.accessibleCompanies, isEmpty);
    expect(user.companyRoles, isEmpty);
  });
}
