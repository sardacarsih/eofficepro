import 'package:dio/dio.dart';
import 'package:eoffice_mobile/core/network/api_client.dart';
import 'package:eoffice_mobile/features/letters/domain/disposition_models.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:eoffice_mobile/features/letters/domain/letter_models.dart';
import 'package:eoffice_mobile/features/letters/domain/search_models.dart';
import 'package:eoffice_mobile/shared/pagination.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';

final letterRepositoryProvider = Provider<LetterRepository>((ref) {
  return LetterRepository(ref.watch(dioProvider));
});

final approvalInboxProvider =
    FutureProvider<Paginated<ApprovalInboxItem>>((ref) {
  return ref.watch(letterRepositoryProvider).approvalInbox();
});

final incomingLettersProvider =
    FutureProvider<Paginated<IncomingLetter>>((ref) {
  return ref.watch(letterRepositoryProvider).incomingLetters();
});

final dispositionInboxProvider =
    FutureProvider<Paginated<DispositionInboxItem>>((ref) {
  return ref.watch(letterRepositoryProvider).dispositionInbox();
});

final sentLettersProvider = FutureProvider<Paginated<DraftLetter>>((ref) {
  return ref.watch(letterRepositoryProvider).sentLetters();
});

final positionsProvider = FutureProvider<Paginated<PositionOption>>((ref) {
  return ref.watch(letterRepositoryProvider).positions();
});

final letterDetailProvider =
    FutureProvider.family<LetterDetail, String>((ref, letterId) {
  return ref.watch(letterRepositoryProvider).letterDetail(letterId);
});

final letterDispositionsProvider =
    FutureProvider.family<Paginated<DispositionItem>, String>((ref, letterId) {
  return ref.watch(letterRepositoryProvider).letterDispositions(letterId);
});

final letterSearchProvider =
    FutureProvider.family<List<LetterSearchResult>, String>((ref, query) {
  if (query.trim().isEmpty) return Future.value(const []);
  return ref.watch(letterRepositoryProvider).search(query.trim());
});

class LetterRepository {
  const LetterRepository(this._dio);

  final Dio _dio;

  Future<Paginated<ApprovalInboxItem>> approvalInbox() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/approvals/inbox',
        queryParameters: const {'page': 1, 'page_size': 50},
      );
      return Paginated.fromJson(
          response.data ?? {}, ApprovalInboxItem.fromJson);
    } catch (error) {
      throw mapDioException(error, 'Gagal memuat approval');
    }
  }

  Future<Paginated<IncomingLetter>> incomingLetters() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/letters/inbox',
        queryParameters: const {'page': 1, 'page_size': 50},
      );
      return Paginated.fromJson(response.data ?? {}, IncomingLetter.fromJson);
    } catch (error) {
      throw mapDioException(error, 'Gagal memuat surat masuk');
    }
  }

  Future<LetterDetail> letterDetail(String letterId) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/letters/view/$letterId',
      );
      final letter = response.data?['letter'] as Map<String, dynamic>? ?? {};
      return LetterDetail.fromJson(letter);
    } catch (error) {
      throw mapDioException(error, 'Gagal memuat detail surat');
    }
  }

  Future<void> actApproval({
    required String stepId,
    required String action,
    String? note,
    String? signatureImageBase64,
  }) async {
    try {
      final data = <String, dynamic>{
        'action': action,
        'note': note,
        'client_action_id': const Uuid().v4(),
        'device_info': 'android-tablet-online',
      };
      if (signatureImageBase64 != null) {
        data['signature_image_base64'] = signatureImageBase64;
        data['signature_mime_type'] = 'image/png';
      }
      await _dio.post<Map<String, dynamic>>(
        '/approvals/steps/$stepId/actions',
        data: data,
      );
    } catch (error) {
      throw mapDioException(error, 'Aksi approval gagal');
    }
  }

  Future<Paginated<DispositionInboxItem>> dispositionInbox() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/dispositions/inbox',
        queryParameters: const {'page': 1, 'page_size': 50},
      );
      return Paginated.fromJson(
        response.data ?? {},
        DispositionInboxItem.fromJson,
      );
    } catch (error) {
      throw mapDioException(error, 'Gagal memuat disposisi');
    }
  }

  Future<Paginated<DraftLetter>> sentLetters() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/letters/mine',
        queryParameters: const {'page': 1, 'page_size': 50},
      );
      return Paginated.fromJson(response.data ?? {}, DraftLetter.fromJson);
    } catch (error) {
      throw mapDioException(error, 'Gagal memuat surat terkirim');
    }
  }

  Future<Paginated<DispositionItem>> letterDispositions(String letterId) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/letters/view/$letterId/dispositions',
        queryParameters: const {'page': 1, 'page_size': 50},
      );
      return Paginated.fromJson(response.data ?? {}, DispositionItem.fromJson);
    } catch (error) {
      throw mapDioException(error, 'Gagal memuat riwayat disposisi');
    }
  }

  Future<void> createDisposition({
    required String letterId,
    required String fromPositionId,
    required String instruction,
    required List<String> recipientPositionIds,
    String? dueDate,
  }) async {
    try {
      await _dio.post<Map<String, dynamic>>(
        '/letters/view/$letterId/dispositions',
        data: {
          'from_position_id': fromPositionId,
          'instruction': instruction,
          'due_date': dueDate,
          'recipient_position_ids': recipientPositionIds,
        },
      );
    } catch (error) {
      throw mapDioException(error, 'Gagal membuat disposisi');
    }
  }

  Future<void> updateDispositionStatus({
    required String recipientId,
    required String status,
    String? followupNote,
  }) async {
    try {
      await _dio.put<Map<String, dynamic>>(
        '/dispositions/recipients/$recipientId/status',
        data: {'status': status, 'followup_note': followupNote},
      );
    } catch (error) {
      throw mapDioException(error, 'Gagal memperbarui disposisi');
    }
  }

  Future<Paginated<PositionOption>> positions() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/positions',
        queryParameters: const {'page': 1, 'page_size': 100},
      );
      return Paginated.fromJson(response.data ?? {}, PositionOption.fromJson);
    } catch (error) {
      throw mapDioException(error, 'Gagal memuat daftar jabatan');
    }
  }

  Future<List<LetterSearchResult>> search(String query) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/letters/search',
        queryParameters: {'q': query},
      );
      return (response.data?['results'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(LetterSearchResult.fromJson)
          .toList();
    } catch (error) {
      throw mapDioException(error, 'Pencarian gagal');
    }
  }
}
