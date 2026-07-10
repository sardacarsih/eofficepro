import 'package:eoffice_mobile/app/navigation.dart';
import 'package:eoffice_mobile/features/auth/presentation/auth_controller.dart';
import 'package:eoffice_mobile/features/auth/presentation/change_password_page.dart';
import 'package:eoffice_mobile/features/auth/presentation/profile_page.dart';
import 'package:eoffice_mobile/features/auth/presentation/forgot_password_page.dart';
import 'package:eoffice_mobile/features/auth/presentation/login_page.dart';
import 'package:eoffice_mobile/features/home/presentation/home_page.dart';
import 'package:eoffice_mobile/features/home/presentation/effectiveness_page.dart';
import 'package:eoffice_mobile/features/letters/presentation/compose_page.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

final routerProvider = Provider<GoRouter>((ref) {
  final refresh = GoRouterRefreshNotifier(ref);
  ref.onDispose(refresh.dispose);

  return GoRouter(
    navigatorKey: rootNavigatorKey,
    initialLocation: '/app',
    refreshListenable: refresh,
    redirect: (context, state) {
      final auth = ref.read(authControllerProvider);
      final signedIn = auth.valueOrNull?.user != null;
      final loading = auth.isLoading;
      final onLogin = state.matchedLocation == '/login';
      final onAuthPage = onLogin || state.matchedLocation == '/forgot-password';

      if (loading) return null;
      if (!signedIn && !onAuthPage) return '/login';
      if (signedIn && onLogin) return '/app';
      return null;
    },
    routes: [
      GoRoute(
        path: '/login',
        name: 'login',
        builder: (context, state) => LoginPage(
          showResetSuccess: state.uri.queryParameters['reset'] == '1',
        ),
      ),
      GoRoute(
        path: '/forgot-password',
        name: 'forgot-password',
        builder: (context, state) => ForgotPasswordPage(
          initialEmail: state.uri.queryParameters['email'],
        ),
      ),
      GoRoute(
        path: '/app',
        name: 'app',
        builder: (context, state) => HomePage(
          initialSection: workSectionFromRouteValue(
            state.uri.queryParameters['section'],
          ),
          initialLetterId: state.uri.queryParameters['letter_id'],
        ),
      ),
      GoRoute(
        path: '/management/effectiveness',
        name: 'effectiveness',
        redirect: (context, state) {
          final user = ref.read(authControllerProvider).valueOrNull?.user;
          return user != null &&
                  user.roles.any(
                    (role) => role == 'admin' || role == 'management_viewer',
                  )
              ? null
              : '/app';
        },
        builder: (context, state) => const EffectivenessPage(),
      ),
      GoRoute(
        path: '/profile',
        name: 'profile',
        builder: (context, state) => const ProfilePage(),
      ),
      GoRoute(
        path: '/change-password',
        name: 'change-password',
        builder: (context, state) => const ChangePasswordPage(),
      ),
      GoRoute(
        path: '/compose',
        name: 'compose',
        redirect: (context, state) {
          final auth = ref.read(authControllerProvider);
          if (auth.isLoading) return null;
          return canComposeLetters(auth.valueOrNull?.user) ? null : '/app';
        },
        builder: (context, state) => const ComposePage(),
      ),
    ],
  );
});

class GoRouterRefreshNotifier extends ChangeNotifier {
  GoRouterRefreshNotifier(this.ref) {
    _subscription = ref.listen(authControllerProvider, (_, __) {
      notifyListeners();
    });
  }

  final Ref ref;
  late final ProviderSubscription<AsyncValue<AuthState>> _subscription;

  @override
  void dispose() {
    _subscription.close();
    super.dispose();
  }
}
