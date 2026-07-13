class User {
  const User({
    required this.id,
    required this.email,
    required this.fullName,
    required this.roles,
    required this.positions,
    this.accessibleCompanies = const [],
    this.companyRoles = const [],
    this.isSuperAdmin = false,
    this.nik,
    this.status,
  });

  final String id;
  final String? nik;
  final String email;
  final String fullName;
  final String? status;
  final List<String> roles;
  final List<UserPosition> positions;
  final List<AccessibleCompany> accessibleCompanies;
  final List<UserCompanyRole> companyRoles;
  final bool isSuperAdmin;

  factory User.fromJson(Map<String, dynamic> json) {
    return User(
      id: json['id'] as String,
      nik: json['nik'] as String?,
      email: json['email'] as String? ?? '',
      fullName: json['full_name'] as String? ?? '',
      status: json['status'] as String?,
      roles: (json['roles'] as List<dynamic>? ?? const [])
          .whereType<String>()
          .toList(),
      positions: (json['positions'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(UserPosition.fromJson)
          .toList(),
      accessibleCompanies:
          (json['accessible_companies'] as List<dynamic>? ?? const [])
              .whereType<Map<String, dynamic>>()
              .map(AccessibleCompany.fromJson)
              .toList(),
      companyRoles: (json['company_roles'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(UserCompanyRole.fromJson)
          .toList(),
      isSuperAdmin: (json['capabilities']
              as Map<String, dynamic>?)?['is_super_admin'] as bool? ??
          false,
    );
  }
}

class AccessibleCompany {
  const AccessibleCompany({
    required this.id,
    required this.code,
    required this.name,
    required this.isActive,
  });

  final String id;
  final String code;
  final String name;
  final bool isActive;

  factory AccessibleCompany.fromJson(Map<String, dynamic> json) =>
      AccessibleCompany(
        id: json['id'] as String? ?? '',
        code: json['code'] as String? ?? '',
        name: json['name'] as String? ?? '',
        isActive: json['is_active'] as bool? ?? false,
      );
}

class UserCompanyRole {
  const UserCompanyRole({
    required this.companyId,
    required this.companyCode,
    required this.companyName,
    required this.roleCode,
    this.validFrom,
    this.validTo,
  });

  final String companyId;
  final String companyCode;
  final String companyName;
  final String roleCode;
  final String? validFrom;
  final String? validTo;

  factory UserCompanyRole.fromJson(Map<String, dynamic> json) =>
      UserCompanyRole(
        companyId: json['company_id'] as String? ?? '',
        companyCode: json['company_code'] as String? ?? '',
        companyName: json['company_name'] as String? ?? '',
        roleCode: json['role_code'] as String? ?? '',
        validFrom: json['valid_from'] as String?,
        validTo: json['valid_to'] as String?,
      );
}

class UserPosition {
  const UserPosition({
    required this.positionId,
    required this.title,
    required this.positionType,
    required this.orgUnit,
    required this.assignmentType,
    this.companyId = '',
    this.companyCode = '',
    this.companyName = '',
  });

  final String positionId;
  final String title;
  final String positionType;
  final String orgUnit;
  final String assignmentType;
  final String companyId;
  final String companyCode;
  final String companyName;

  factory UserPosition.fromJson(Map<String, dynamic> json) {
    return UserPosition(
      positionId: json['position_id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      positionType: json['position_type'] as String? ?? '',
      orgUnit: json['org_unit'] as String? ?? '',
      assignmentType: json['assignment_type'] as String? ?? '',
      companyId: json['company_id'] as String? ?? '',
      companyCode: json['company_code'] as String? ?? '',
      companyName: json['company_name'] as String? ?? '',
    );
  }
}
