import 'package:eoffice_mobile/features/auth/domain/user.dart';
import 'package:eoffice_mobile/features/auth/presentation/auth_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

class ProfilePage extends ConsumerWidget {
  const ProfilePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authControllerProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('Profil')),
      body: SafeArea(
        child: auth.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, stackTrace) => Center(
            child: OutlinedButton.icon(
              onPressed: () => ref.invalidate(authControllerProvider),
              icon: const Icon(Icons.refresh),
              label: const Text('Coba lagi'),
            ),
          ),
          data: (state) {
            final user = state.user;
            if (user == null) {
              return const Center(child: Text('Sesi pengguna tidak tersedia.'));
            }
            return _ProfileContent(user: user);
          },
        ),
      ),
    );
  }
}

class _ProfileContent extends StatelessWidget {
  const _ProfileContent({required this.user});

  final User user;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final textTheme = Theme.of(context).textTheme;
    return LayoutBuilder(
      builder: (context, constraints) => SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Center(
          child: ConstrainedBox(
            constraints: BoxConstraints(
              maxWidth: constraints.maxWidth >= 900 ? 720 : 600,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Row(
                  children: [
                    CircleAvatar(
                      radius: 32,
                      child: Text(_initials(user.fullName),
                          style: textTheme.titleLarge),
                    ),
                    const SizedBox(width: 16),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(user.fullName, style: textTheme.headlineSmall),
                          const SizedBox(height: 4),
                          Text(
                            user.email,
                            style: textTheme.bodyMedium?.copyWith(
                              color: scheme.onSurfaceVariant,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 32),
                Text('Informasi akun', style: textTheme.titleMedium),
                const SizedBox(height: 8),
                _InfoRow(label: 'NIK', value: user.nik ?? '-'),
                _InfoRow(label: 'Email', value: user.email),
                _InfoRow(label: 'Status', value: user.status ?? '-'),
                const SizedBox(height: 28),
                Text('Peran', style: textTheme.titleMedium),
                const SizedBox(height: 10),
                if (user.roles.isEmpty)
                  const Text('Belum ada peran aktif.')
                else
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      for (final role in user.roles) Chip(label: Text(role))
                    ],
                  ),
                const SizedBox(height: 28),
                Text('Jabatan aktif', style: textTheme.titleMedium),
                const SizedBox(height: 8),
                if (user.positions.isEmpty)
                  const Padding(
                    padding: EdgeInsets.symmetric(vertical: 16),
                    child: Text('Belum ada jabatan aktif.'),
                  )
                else
                  ...user.positions
                      .map((position) => _PositionRow(position: position)),
                const SizedBox(height: 32),
                if (user.roles.any(
                  (role) => role == 'admin' || role == 'management_viewer',
                )) ...[
                  FilledButton.icon(
                    onPressed: () => context.pushNamed('effectiveness'),
                    icon: const Icon(Icons.insights_outlined),
                    label: const Text('Efektivitas aplikasi'),
                  ),
                  const SizedBox(height: 12),
                ],
                FilledButton.icon(
                  onPressed: () => context.pushNamed('change-password'),
                  icon: const Icon(Icons.lock_outline),
                  label: const Text('Ubah password'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  const _InfoRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 10),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(width: 112, child: Text(label)),
            Expanded(child: Text(value)),
          ],
        ),
      );
}

class _PositionRow extends StatelessWidget {
  const _PositionRow({required this.position});

  final UserPosition position;

  @override
  Widget build(BuildContext context) => ListTile(
        contentPadding: EdgeInsets.zero,
        leading: const Icon(Icons.badge_outlined),
        title: Text(position.title),
        subtitle: Text('${position.orgUnit} - ${position.assignmentType}'),
      );
}

String _initials(String name) {
  final words =
      name.trim().split(RegExp(r'\s+')).where((word) => word.isNotEmpty);
  final initials = words.take(2).map((word) => word[0].toUpperCase()).join();
  return initials.isEmpty ? '?' : initials;
}
