import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:eoffice_mobile/core/network/api_client.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

final draftRepositoryProvider = Provider<DraftRepository>((ref) {
  return DraftRepository(ref.watch(dioProvider));
});

class DraftRepository {
  const DraftRepository(this._dio);

  static const _lookupPageSize = 100;

  final Dio _dio;

  Future<DraftComposerBootstrap> loadBootstrap() async {
    try {
      final companiesFuture = _loadAllPages(
        '/companies',
        DraftCompany.fromJson,
      );
      final letterTypesFuture = _loadAllPages(
        '/letter-types',
        DraftLetterType.fromJson,
      );
      final templatesFuture = _loadAllPages(
        '/letter-templates',
        DraftLetterTemplate.fromJson,
      );
      final positionsFuture = _loadAllPages(
        '/positions',
        DraftPosition.fromJson,
      );
      final orgUnitsFuture = _loadOrgUnits();

      final results = await Future.wait<Object>([
        companiesFuture,
        letterTypesFuture,
        templatesFuture,
        positionsFuture,
        orgUnitsFuture,
      ]);
      return DraftComposerBootstrap(
        companies: results[0] as List<DraftCompany>,
        letterTypes: results[1] as List<DraftLetterType>,
        templates: results[2] as List<DraftLetterTemplate>,
        positions: results[3] as List<DraftPosition>,
        orgUnits: results[4] as List<DraftOrgUnit>,
      );
    } catch (error) {
      throw mapDioException(error, 'Gagal memuat data penulisan surat');
    }
  }

  Future<List<DraftLetter>> listDrafts() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/letters/drafts',
      );
      return (response.data?['letters'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(DraftLetter.fromJson)
          .toList();
    } catch (error) {
      throw mapDioException(error, 'Gagal memuat draft surat');
    }
  }

  Future<DraftLetter> getDraft(String draftId) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/letters/drafts/$draftId',
      );
      final json = response.data?['letter'] as Map<String, dynamic>? ?? {};
      return DraftLetter.fromJson(json);
    } catch (error) {
      throw mapDioException(error, 'Gagal memuat draft surat');
    }
  }

  Future<DraftSaveResult> saveDraft({
    required DraftLetterPayload payload,
    String? draftId,
  }) {
    final normalizedId = draftId?.trim();
    if (normalizedId == null || normalizedId.isEmpty) {
      return createDraft(payload);
    }
    return updateDraft(normalizedId, payload);
  }

  Future<DraftSaveResult> createDraft(DraftLetterPayload payload) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/letters/drafts',
        data: payload.toJson(),
      );
      return DraftSaveResult.fromJson(response.data ?? {});
    } catch (error) {
      throw mapDioException(error, 'Gagal membuat draft surat');
    }
  }

  Future<DraftSaveResult> updateDraft(
    String draftId,
    DraftLetterPayload payload,
  ) async {
    try {
      final response = await _dio.put<Map<String, dynamic>>(
        '/letters/drafts/$draftId',
        data: payload.toJson(),
      );
      return DraftSaveResult.fromJson(response.data ?? {});
    } catch (error) {
      throw mapDioException(error, 'Gagal memperbarui draft surat');
    }
  }

  Future<List<DraftAttachment>> listAttachments(String draftId) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/letters/drafts/$draftId/attachments',
      );
      return (response.data?['attachments'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(DraftAttachment.fromJson)
          .toList();
    } catch (error) {
      throw mapDioException(error, 'Gagal memuat lampiran draft');
    }
  }

  Future<String> uploadAttachment({
    required String draftId,
    required String fileName,
    required String mimeType,
    Uint8List? bytes,
    String? filePath,
    ProgressCallback? onSendProgress,
  }) async {
    try {
      if ((bytes == null) == (filePath == null)) {
        throw ArgumentError(
          'Tentukan tepat satu sumber lampiran: bytes atau filePath.',
        );
      }
      final multipart = filePath != null
          ? await MultipartFile.fromFile(
              filePath,
              filename: fileName,
              contentType: DioMediaType.parse(mimeType),
            )
          : MultipartFile.fromBytes(
              bytes!,
              filename: fileName,
              contentType: DioMediaType.parse(mimeType),
            );
      final form = FormData.fromMap({
        'file': multipart,
      });
      final response = await _dio.post<Map<String, dynamic>>(
        '/letters/drafts/$draftId/attachments',
        data: form,
        onSendProgress: onSendProgress,
      );
      return response.data?['id'] as String? ?? '';
    } catch (error) {
      throw mapDioException(error, 'Gagal mengunggah lampiran draft');
    }
  }

  Future<void> deleteAttachment({
    required String draftId,
    required String attachmentId,
  }) async {
    try {
      await _dio.delete<Map<String, dynamic>>(
        '/letters/drafts/$draftId/attachments/$attachmentId',
      );
    } catch (error) {
      throw mapDioException(error, 'Gagal menghapus lampiran draft');
    }
  }

  Future<DraftPreviewResult> previewDraft(String draftId) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/letters/drafts/$draftId/preview',
      );
      return DraftPreviewResult.fromJson(response.data ?? {});
    } catch (error) {
      throw mapDioException(error, 'Gagal membuat preview surat');
    }
  }

  Future<DraftSubmitResult> submitDraft(String draftId) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/letters/drafts/$draftId/submit',
      );
      return DraftSubmitResult.fromJson(response.data ?? {});
    } catch (error) {
      throw mapDioException(error, 'Gagal mengajukan draft surat');
    }
  }

  Future<List<T>> _loadAllPages<T>(
    String path,
    T Function(Map<String, dynamic>) fromJson,
  ) async {
    final result = <T>[];
    var page = 1;
    var totalPages = 1;

    do {
      final response = await _dio.get<Map<String, dynamic>>(
        path,
        queryParameters: {'page': page, 'page_size': _lookupPageSize},
      );
      final data = response.data ?? const <String, dynamic>{};
      result.addAll(
        (data['data'] as List<dynamic>? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(fromJson),
      );
      final meta = data['meta'] as Map<String, dynamic>?;
      totalPages = _positiveInt(meta?['total_pages'], fallback: page);
      page++;
    } while (page <= totalPages);

    return result;
  }

  Future<List<DraftOrgUnit>> _loadOrgUnits() async {
    final response = await _dio.get<Map<String, dynamic>>('/org-units');
    final units = <DraftOrgUnit>[];

    void appendUnit(Map<String, dynamic> json) {
      units.add(DraftOrgUnit.fromJson(json));
      final children = json['children'] as List<dynamic>? ?? const [];
      for (final child in children.whereType<Map<String, dynamic>>()) {
        appendUnit(child);
      }
    }

    final tree = response.data?['tree'] as List<dynamic>? ?? const [];
    for (final root in tree.whereType<Map<String, dynamic>>()) {
      appendUnit(root);
    }
    return units;
  }
}

int _positiveInt(Object? value, {required int fallback}) {
  if (value is num && value.toInt() > 0) return value.toInt();
  return fallback;
}
