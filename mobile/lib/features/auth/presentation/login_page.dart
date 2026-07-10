import 'package:eoffice_mobile/core/services/identifier_storage.dart';
import 'package:eoffice_mobile/features/auth/presentation/auth_controller.dart';
import 'package:eoffice_mobile/features/auth/presentation/widgets/brand_mark.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

class LoginPage extends ConsumerStatefulWidget {
  const LoginPage({super.key, this.showResetSuccess = false});

  /// Bernilai true saat kembali dari alur reset password yang berhasil.
  final bool showResetSuccess;

  @override
  ConsumerState<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends ConsumerState<LoginPage> {
  static const _twoPaneMinWidth = 900.0;
  static const _formMaxWidth = 440.0;

  final _formKey = GlobalKey<FormState>();
  final _identifierController = TextEditingController();
  final _passwordController = TextEditingController();
  var _obscurePassword = true;
  var _rememberMe = false;

  @override
  void initState() {
    super.initState();
    _restoreSavedIdentifier();
    if (widget.showResetSuccess) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Password berhasil diubah, silakan login'),
          ),
        );
      });
    }
  }

  @override
  void dispose() {
    _identifierController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  Future<void> _restoreSavedIdentifier() async {
    final saved = await ref.read(identifierStorageProvider).read();
    if (!mounted || saved == null || saved.isEmpty) return;
    setState(() {
      _identifierController.text = saved;
      _rememberMe = true;
    });
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    final identifier = _identifierController.text.trim();
    await ref.read(authControllerProvider.notifier).login(
          identifier,
          _passwordController.text,
          rememberMe: _rememberMe,
        );
    if (!mounted) return;
    if (ref.read(authControllerProvider).error == null) {
      final storage = ref.read(identifierStorageProvider);
      if (_rememberMe) {
        await storage.save(identifier);
      } else {
        await storage.clear();
      }
    }
  }

  void _openForgotPassword() {
    final identifier = _identifierController.text.trim();
    // Prefill hanya bila identifier berbentuk email (bukan NIK).
    final query = identifier.contains('@') ? {'email': identifier} : null;
    context.push(
      Uri(path: '/forgot-password', queryParameters: query).toString(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final auth = ref.watch(authControllerProvider);
    final busy = auth.isLoading;
    final errorMessage = !busy && auth.hasError ? auth.error.toString() : null;

    return Scaffold(
      body: LayoutBuilder(
        builder: (context, constraints) {
          final useTwoPane = constraints.maxWidth >= _twoPaneMinWidth &&
              constraints.maxWidth > constraints.maxHeight;
          final form = _LoginForm(
            formKey: _formKey,
            identifierController: _identifierController,
            passwordController: _passwordController,
            obscurePassword: _obscurePassword,
            rememberMe: _rememberMe,
            busy: busy,
            errorMessage: errorMessage,
            showBrandHeader: !useTwoPane,
            onToggleObscure: () =>
                setState(() => _obscurePassword = !_obscurePassword),
            onRememberChanged: (value) =>
                setState(() => _rememberMe = value ?? false),
            onForgotPassword: _openForgotPassword,
            onSubmit: _submit,
          );
          if (!useTwoPane) {
            return SafeArea(
              child: Center(
                child: SingleChildScrollView(
                  padding: const EdgeInsets.all(24),
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: _formMaxWidth),
                    child: form,
                  ),
                ),
              ),
            );
          }
          return Row(
            children: [
              Expanded(flex: 45, child: _BrandPanel()),
              Expanded(
                flex: 55,
                child: SafeArea(
                  child: Center(
                    child: SingleChildScrollView(
                      padding: const EdgeInsets.all(48),
                      child: ConstrainedBox(
                        constraints:
                            const BoxConstraints(maxWidth: _formMaxWidth),
                        child: form,
                      ),
                    ),
                  ),
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _BrandPanel extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final textTheme = Theme.of(context).textTheme;
    final panelDark = Color.lerp(scheme.primary, Colors.black, 0.35)!;
    return Container(
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [scheme.primary, panelDark],
        ),
      ),
      child: Stack(
        children: [
          const Positioned(
            top: -80,
            right: -60,
            child: _DecorCircle(diameter: 280),
          ),
          const Positioned(
            bottom: -100,
            left: -80,
            child: _DecorCircle(diameter: 360),
          ),
          SafeArea(
            child: Padding(
              padding: const EdgeInsets.all(48),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  const BrandMark(size: 88),
                  const SizedBox(height: 32),
                  Text(
                    'eOffice Pro',
                    style: textTheme.displaySmall?.copyWith(
                      color: Colors.white,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: 12),
                  Text(
                    'Persuratan internal FKK Group',
                    style: textTheme.titleMedium?.copyWith(
                      color: Colors.white.withValues(alpha: 0.9),
                    ),
                  ),
                  const SizedBox(height: 24),
                  Text(
                    'Kelola persetujuan surat, disposisi, dan notifikasi '
                    'dalam satu aplikasi.',
                    style: textTheme.bodyLarge?.copyWith(
                      color: Colors.white.withValues(alpha: 0.75),
                      height: 1.6,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _DecorCircle extends StatelessWidget {
  const _DecorCircle({required this.diameter});

  final double diameter;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: diameter,
      height: diameter,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        color: Colors.white.withValues(alpha: 0.06),
      ),
    );
  }
}

class _LoginForm extends StatelessWidget {
  const _LoginForm({
    required this.formKey,
    required this.identifierController,
    required this.passwordController,
    required this.obscurePassword,
    required this.rememberMe,
    required this.busy,
    required this.errorMessage,
    required this.showBrandHeader,
    required this.onToggleObscure,
    required this.onRememberChanged,
    required this.onForgotPassword,
    required this.onSubmit,
  });

  final GlobalKey<FormState> formKey;
  final TextEditingController identifierController;
  final TextEditingController passwordController;
  final bool obscurePassword;
  final bool rememberMe;
  final bool busy;
  final String? errorMessage;
  final bool showBrandHeader;
  final VoidCallback onToggleObscure;
  final ValueChanged<bool?> onRememberChanged;
  final VoidCallback onForgotPassword;
  final VoidCallback onSubmit;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final textTheme = Theme.of(context).textTheme;

    return Form(
      key: formKey,
      child: AutofillGroup(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (showBrandHeader) ...[
              const Center(child: BrandMark(size: 72)),
              const SizedBox(height: 24),
            ],
            Text(
              'Selamat datang',
              textAlign: showBrandHeader ? TextAlign.center : TextAlign.start,
              style: textTheme.headlineSmall,
            ),
            const SizedBox(height: 8),
            Text(
              'Masuk untuk mengelola persetujuan surat',
              textAlign: showBrandHeader ? TextAlign.center : TextAlign.start,
              style: textTheme.bodyMedium?.copyWith(
                color: scheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 32),
            AnimatedSize(
              duration: const Duration(milliseconds: 250),
              curve: Curves.easeOut,
              alignment: Alignment.topCenter,
              child: errorMessage == null
                  ? const SizedBox.shrink()
                  : Padding(
                      padding: const EdgeInsets.only(bottom: 20),
                      child: Container(
                        padding: const EdgeInsets.all(16),
                        decoration: BoxDecoration(
                          color: scheme.errorContainer,
                          borderRadius: BorderRadius.circular(12),
                        ),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Icon(
                              Icons.error_outline,
                              color: scheme.onErrorContainer,
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: Text(
                                errorMessage ?? '',
                                style: textTheme.bodyMedium?.copyWith(
                                  color: scheme.onErrorContainer,
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
            ),
            TextFormField(
              controller: identifierController,
              textInputAction: TextInputAction.next,
              keyboardType: TextInputType.emailAddress,
              autocorrect: false,
              autofillHints: const [AutofillHints.username],
              decoration: const InputDecoration(
                labelText: 'Email atau NIK',
                prefixIcon: Icon(Icons.person_outline),
              ),
              validator: (value) {
                if (value == null || value.trim().isEmpty) {
                  return 'Email atau NIK wajib diisi';
                }
                return null;
              },
            ),
            const SizedBox(height: 16),
            TextFormField(
              controller: passwordController,
              obscureText: obscurePassword,
              textInputAction: TextInputAction.done,
              autofillHints: const [AutofillHints.password],
              onFieldSubmitted: (_) => busy ? null : onSubmit(),
              decoration: InputDecoration(
                labelText: 'Password',
                prefixIcon: const Icon(Icons.lock_outline),
                suffixIcon: IconButton(
                  tooltip: obscurePassword
                      ? 'Tampilkan password'
                      : 'Sembunyikan password',
                  onPressed: onToggleObscure,
                  icon: Icon(
                    obscurePassword
                        ? Icons.visibility_outlined
                        : Icons.visibility_off_outlined,
                  ),
                ),
              ),
              validator: (value) {
                if (value == null || value.isEmpty) {
                  return 'Password wajib diisi';
                }
                return null;
              },
            ),
            const SizedBox(height: 8),
            Row(
              children: [
                Expanded(
                  child: InkWell(
                    onTap: () => onRememberChanged(!rememberMe),
                    borderRadius: BorderRadius.circular(8),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Checkbox(
                          value: rememberMe,
                          onChanged: onRememberChanged,
                        ),
                        Flexible(
                          child: Text(
                            'Ingat saya',
                            style: textTheme.bodyMedium,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
                TextButton(
                  onPressed: onForgotPassword,
                  child: const Text('Lupa password?'),
                ),
              ],
            ),
            const SizedBox(height: 24),
            FilledButton(
              onPressed: busy ? null : onSubmit,
              style: FilledButton.styleFrom(
                minimumSize: const Size.fromHeight(52),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(12),
                ),
                textStyle: textTheme.labelLarge?.copyWith(fontSize: 15),
              ),
              child: busy
                  ? const SizedBox.square(
                      dimension: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('Masuk'),
            ),
            const SizedBox(height: 32),
            Text(
              'eOffice Pro • FKK Group',
              textAlign: TextAlign.center,
              style: textTheme.bodySmall?.copyWith(
                color: scheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
