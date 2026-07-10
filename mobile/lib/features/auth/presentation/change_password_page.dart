import 'package:eoffice_mobile/features/auth/data/auth_repository.dart';
import 'package:eoffice_mobile/features/auth/presentation/auth_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

String? validateCurrentPassword(String? value) {
  if (value == null || value.isEmpty) return 'Password lama wajib diisi';
  return null;
}

String? validateChangedPassword(String? value, String currentPassword) {
  if (value == null || value.length < 10) {
    return 'Password baru minimal 10 karakter';
  }
  if (value == currentPassword) {
    return 'Password baru harus berbeda dari password lama';
  }
  return null;
}

String? validatePasswordConfirmation(String? value, String newPassword) {
  if (value != newPassword) return 'Konfirmasi password tidak sama';
  return null;
}

class ChangePasswordPage extends ConsumerStatefulWidget {
  const ChangePasswordPage({super.key});

  @override
  ConsumerState<ChangePasswordPage> createState() => _ChangePasswordPageState();
}

class _ChangePasswordPageState extends ConsumerState<ChangePasswordPage> {
  final _formKey = GlobalKey<FormState>();
  final _currentPasswordController = TextEditingController();
  final _newPasswordController = TextEditingController();
  final _confirmationController = TextEditingController();
  var _busy = false;
  var _showCurrentPassword = false;
  var _showNewPassword = false;
  var _showConfirmation = false;
  String? _errorMessage;

  @override
  void dispose() {
    _currentPasswordController.dispose();
    _newPasswordController.dispose();
    _confirmationController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    FocusScope.of(context).unfocus();
    setState(() {
      _busy = true;
      _errorMessage = null;
    });
    try {
      final message = await ref.read(authRepositoryProvider).changePassword(
            currentPassword: _currentPasswordController.text,
            newPassword: _newPasswordController.text,
          );
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        barrierDismissible: false,
        builder: (context) => AlertDialog(
          title: const Text('Password diubah'),
          content: Text(message),
          actions: [
            FilledButton(
              onPressed: () => Navigator.of(context).pop(),
              child: const Text('Masuk kembali'),
            ),
          ],
        ),
      );
      if (!mounted) return;
      await ref.read(authControllerProvider.notifier).logout();
    } catch (error) {
      if (mounted) setState(() => _errorMessage = error.toString());
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final textTheme = Theme.of(context).textTheme;
    return Scaffold(
      appBar: AppBar(title: const Text('Ubah password')),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 480),
              child: AutofillGroup(
                child: Form(
                  key: _formKey,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Text('Perbarui password', style: textTheme.headlineSmall),
                      const SizedBox(height: 8),
                      Text(
                        'Gunakan minimal 10 karakter. Setelah berhasil, Anda perlu masuk kembali.',
                        style: textTheme.bodyMedium?.copyWith(
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                      if (_errorMessage != null) ...[
                        const SizedBox(height: 24),
                        _ErrorBanner(message: _errorMessage!),
                      ],
                      const SizedBox(height: 28),
                      _PasswordField(
                        controller: _currentPasswordController,
                        label: 'Password lama',
                        visible: _showCurrentPassword,
                        autofillHints: const [AutofillHints.password],
                        textInputAction: TextInputAction.next,
                        onToggle: () => setState(
                          () => _showCurrentPassword = !_showCurrentPassword,
                        ),
                        validator: validateCurrentPassword,
                      ),
                      const SizedBox(height: 16),
                      _PasswordField(
                        controller: _newPasswordController,
                        label: 'Password baru',
                        visible: _showNewPassword,
                        autofillHints: const [AutofillHints.newPassword],
                        textInputAction: TextInputAction.next,
                        onToggle: () => setState(
                          () => _showNewPassword = !_showNewPassword,
                        ),
                        validator: (value) => validateChangedPassword(
                          value,
                          _currentPasswordController.text,
                        ),
                      ),
                      const SizedBox(height: 16),
                      _PasswordField(
                        controller: _confirmationController,
                        label: 'Konfirmasi password baru',
                        visible: _showConfirmation,
                        autofillHints: const [AutofillHints.newPassword],
                        textInputAction: TextInputAction.done,
                        onToggle: () => setState(
                          () => _showConfirmation = !_showConfirmation,
                        ),
                        onSubmitted: (_) => _busy ? null : _submit(),
                        validator: (value) => validatePasswordConfirmation(
                          value,
                          _newPasswordController.text,
                        ),
                      ),
                      const SizedBox(height: 28),
                      FilledButton.icon(
                        onPressed: _busy ? null : _submit,
                        icon: _busy
                            ? const SizedBox.square(
                                dimension: 18,
                                child:
                                    CircularProgressIndicator(strokeWidth: 2),
                              )
                            : const Icon(Icons.lock_reset_outlined),
                        label: Text(_busy ? 'Menyimpan...' : 'Simpan password'),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _PasswordField extends StatelessWidget {
  const _PasswordField({
    required this.controller,
    required this.label,
    required this.visible,
    required this.autofillHints,
    required this.textInputAction,
    required this.onToggle,
    required this.validator,
    this.onSubmitted,
  });

  final TextEditingController controller;
  final String label;
  final bool visible;
  final Iterable<String> autofillHints;
  final TextInputAction textInputAction;
  final VoidCallback onToggle;
  final String? Function(String?) validator;
  final ValueChanged<String>? onSubmitted;

  @override
  Widget build(BuildContext context) => TextFormField(
        controller: controller,
        obscureText: !visible,
        textInputAction: textInputAction,
        autofillHints: autofillHints,
        onFieldSubmitted: onSubmitted,
        decoration: InputDecoration(
          labelText: label,
          prefixIcon: const Icon(Icons.lock_outline),
          suffixIcon: IconButton(
            tooltip: visible ? 'Sembunyikan password' : 'Tampilkan password',
            onPressed: onToggle,
            icon: Icon(
              visible
                  ? Icons.visibility_off_outlined
                  : Icons.visibility_outlined,
            ),
          ),
        ),
        validator: validator,
      );
}

class _ErrorBanner extends StatelessWidget {
  const _ErrorBanner({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: scheme.errorContainer,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.error_outline, color: scheme.onErrorContainer),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              message,
              style: TextStyle(color: scheme.onErrorContainer),
            ),
          ),
        ],
      ),
    );
  }
}
