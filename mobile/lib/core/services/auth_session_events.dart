import 'package:flutter_riverpod/flutter_riverpod.dart';

enum AuthSessionEventType { refreshed, expired }

class AuthSessionEvent {
  const AuthSessionEvent({required this.type, required this.sequence});

  final AuthSessionEventType type;
  final int sequence;
}

final authSessionEventsProvider =
    NotifierProvider<AuthSessionEvents, AuthSessionEvent?>(
  AuthSessionEvents.new,
);

class AuthSessionEvents extends Notifier<AuthSessionEvent?> {
  var _sequence = 0;

  @override
  AuthSessionEvent? build() => null;

  void refreshed() => _emit(AuthSessionEventType.refreshed);

  void expired() => _emit(AuthSessionEventType.expired);

  void _emit(AuthSessionEventType type) {
    state = AuthSessionEvent(type: type, sequence: ++_sequence);
  }
}
