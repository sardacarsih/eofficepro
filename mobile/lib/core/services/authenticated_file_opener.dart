import 'dart:io';

import 'package:dio/dio.dart';
import 'package:eoffice_mobile/core/exceptions/app_exception.dart';
import 'package:eoffice_mobile/core/network/api_client.dart';
import 'package:open_filex/open_filex.dart';
import 'package:path_provider/path_provider.dart';

class AuthenticatedFileOpener {
  const AuthenticatedFileOpener(this._dio);

  final Dio _dio;

  Future<void> open(String endpoint, String fileName) async {
    final directory = await getTemporaryDirectory();
    final safeName = fileName.replaceAll(RegExp(r'[^A-Za-z0-9._-]'), '_');
    final path = '${directory.path}${Platform.pathSeparator}$safeName';
    try {
      await _dio.download(endpoint, path, deleteOnError: true);
      final result = await OpenFilex.open(path);
      if (result.type != ResultType.done) {
        throw AppException(result.message.isEmpty
            ? 'Aplikasi tidak dapat membuka file.'
            : result.message);
      }
    } on AppException {
      rethrow;
    } catch (error) {
      throw mapDioException(error, 'Gagal mengunduh file.');
    }
  }
}
