import 'dart:async';

import 'package:eoffice_mobile/core/services/auth_session_events.dart';
import 'package:eoffice_mobile/features/auth/data/auth_repository.dart';
import 'package:eoffice_mobile/features/auth/domain/user.dart';
import 'package:eoffice_mobile/core/services/push_notification_service.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

final authControllerProvider =
    AsyncNotifierProvider<AuthController, AuthState>(AuthController.new);

class AuthState {
  const AuthState({this.user});

  final User? user;
}

class AuthController extends AsyncNotifier<AuthState> {
  @override
  Future<AuthState> build() async {
    ref.listen<AuthSessionEvent?>(authSessionEventsProvider, (_, event) {
      if (event == null) return;
      if (event.type == AuthSessionEventType.expired) {
        state = const AsyncData(AuthState());
      } else {
        unawaited(_reloadProfile());
      }
    });
    final user = await ref.read(authRepositoryProvider).restoreSession();
    if (user != null) {
      await _registerPushToken();
    }
    return AuthState(user: user);
  }

  Future<void> login(
    String identifier,
    String password, {
    bool rememberMe = false,
  }) async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(() async {
      final user = await ref.read(authRepositoryProvider).login(
            identifier: identifier,
            password: password,
            rememberMe: rememberMe,
          );
      await _registerPushToken();
      return AuthState(user: user);
    });
  }

  Future<void> logout() async {
    state = const AsyncLoading();
    await ref.read(pushNotificationServiceProvider).unregisterCurrentDevice();
    await ref.read(authRepositoryProvider).logout();
    state = const AsyncData(AuthState());
  }

  Future<void> _registerPushToken() async {
    try {
      await ref.read(pushNotificationServiceProvider).registerCurrentDevice();
    } catch (_) {
      // Push registration is best-effort; auth must remain usable.
    }
  }

  Future<void> _reloadProfile() async {
    if (state.valueOrNull?.user == null) return;
    try {
      final user = await ref.read(authRepositoryProvider).me();
      state = AsyncData(AuthState(user: user));
    } catch (_) {
      // A terminal 401 is handled by the session-expired event.
    }
  }
}
