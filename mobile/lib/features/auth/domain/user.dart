class User {
  const User({
    required this.id,
    required this.email,
    required this.fullName,
    required this.roles,
    required this.positions,
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
    );
  }
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
