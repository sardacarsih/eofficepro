part of 'draft_models.dart';

class DraftAttachment {
  const DraftAttachment({
    required this.id,
    required this.fileName,
    required this.mimeType,
    required this.sizeBytes,
    required this.storageKey,
    required this.checksumSha256,
    required this.scanStatus,
    required this.createdAt,
    this.downloadUrl,
  });

  final String id;
  final String fileName;
  final String mimeType;
  final int sizeBytes;
  final String storageKey;
  final String checksumSha256;
  final String scanStatus;
  final String? downloadUrl;
  final String createdAt;

  factory DraftAttachment.fromJson(Map<String, dynamic> json) {
    return DraftAttachment(
      id: json['id'] as String? ?? '',
      fileName: json['file_name'] as String? ?? '',
      mimeType: json['mime_type'] as String? ?? '',
      sizeBytes: _asInt(json['size_bytes']),
      storageKey: json['storage_key'] as String? ?? '',
      checksumSha256: json['checksum_sha256'] as String? ?? '',
      scanStatus: json['scan_status'] as String? ?? '',
      downloadUrl: json['download_url'] as String?,
      createdAt: json['created_at'] as String? ?? '',
    );
  }
}

class DraftPreviewResult {
  const DraftPreviewResult({
    required this.storageKey,
    required this.previewUrl,
    required this.expiresIn,
  });

  final String storageKey;
  final String previewUrl;
  final int expiresIn;

  factory DraftPreviewResult.fromJson(Map<String, dynamic> json) {
    return DraftPreviewResult(
      storageKey: json['storage_key'] as String? ?? '',
      previewUrl: json['preview_url'] as String? ?? '',
      expiresIn: _asInt(json['expires_in']),
    );
  }
}

class DraftSubmitApprovalStep {
  const DraftSubmitApprovalStep({
    required this.stepOrder,
    required this.flowGroup,
    required this.positionId,
    required this.positionType,
    required this.title,
  });

  final int stepOrder;
  final int flowGroup;
  final String positionId;
  final String positionType;
  final String title;

  factory DraftSubmitApprovalStep.fromJson(Map<String, dynamic> json) {
    return DraftSubmitApprovalStep(
      stepOrder: _asInt(json['step_order']),
      flowGroup: _asInt(json['flow_group']),
      positionId: json['position_id'] as String? ?? '',
      positionType: json['position_type'] as String? ?? '',
      title: json['title'] as String? ?? '',
    );
  }
}

class DraftSubmitResult {
  const DraftSubmitResult({
    required this.id,
    required this.status,
    required this.approvalCycle,
    required this.qrToken,
    required this.verifyUrl,
    required this.approvalSteps,
  });

  final String id;
  final DraftLetterStatus status;
  final int approvalCycle;
  final String qrToken;
  final String verifyUrl;
  final List<DraftSubmitApprovalStep> approvalSteps;

  factory DraftSubmitResult.fromJson(Map<String, dynamic> json) {
    return DraftSubmitResult(
      id: json['id'] as String? ?? '',
      status: DraftLetterStatus.fromWire(json['status'] as String?),
      approvalCycle: _asInt(json['approval_cycle']),
      qrToken: json['qr_token'] as String? ?? '',
      verifyUrl: json['verify_url'] as String? ?? '',
      approvalSteps: (json['approval_steps'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(DraftSubmitApprovalStep.fromJson)
          .toList(),
    );
  }
}
