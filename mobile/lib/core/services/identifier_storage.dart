import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

final identifierStorageProvider = Provider<IdentifierStorage>((ref) {
  return const IdentifierStorage(FlutterSecureStorage());
});

/// Menyimpan email/NIK terakhir untuk fitur "Ingat saya" di halaman login.
/// Hanya identifier yang disimpan, tidak pernah password.
class IdentifierStorage {
  const IdentifierStorage(this._storage);

  static const _identifierKey = 'eoffice_saved_identifier';

  final FlutterSecureStorage _storage;

  Future<String?> read() => _storage.read(key: _identifierKey);

  Future<void> save(String identifier) =>
      _storage.write(key: _identifierKey, value: identifier);

  Future<void> clear() => _storage.delete(key: _identifierKey);
}
