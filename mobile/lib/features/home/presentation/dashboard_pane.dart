import 'package:eoffice_mobile/core/utils/date_format.dart';
import 'package:eoffice_mobile/core/widgets/async_state_view.dart';
import 'package:eoffice_mobile/features/auth/presentation/auth_controller.dart';
import 'package:eoffice_mobile/features/home/data/dashboard_repository.dart';
import 'package:eoffice_mobile/features/home/domain/dashboard_summary.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Halaman Ringkasan. Menyesuaikan diri dengan lebar pane-nya sendiri:
/// >= 840 memakai KPI 4 kolom + konten dua kolom, di bawahnya satu kolom.
class DashboardPane extends ConsumerWidget {
  const DashboardPane({
    required this.onOpenApprovals,
    required this.onOpenInbox,
    required this.onOpenArchive,
    required this.onOpenApproval,
    super.key,
  });

  final VoidCallback onOpenApprovals;
  final VoidCallback onOpenInbox;
  final VoidCallback onOpenArchive;
  final void Function(String letterId, String stepId) onOpenApproval;

  static const _wideMinWidth = 840.0;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final summary = ref.watch(dashboardSummaryProvider);
    final userName = ref.watch(authControllerProvider).valueOrNull?.user;
    return AsyncStateView<DashboardSummary>(
      value: summary,
      onRetry: () => ref.invalidate(dashboardSummaryProvider),
      data: (data) {
        return LayoutBuilder(
          builder: (context, constraints) {
            final wide = constraints.maxWidth >= _wideMinWidth;
            return ListView(
              padding: EdgeInsets.all(wide ? 24 : 16),
              children: [
                _DashboardHeader(userName: userName?.fullName),
                const SizedBox(height: 16),
                _buildKpiGrid(data, columns: wide ? 4 : 2),
                const SizedBox(height: 24),
                if (wide)
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Expanded(
                        flex: 5,
                        child: _PendingApprovalsCard(
                          items: data.pendingApprovals,
                          onOpenApprovals: onOpenApprovals,
                          onOpenApproval: onOpenApproval,
                        ),
                      ),
                      const SizedBox(width: 16),
                      Expanded(
                        flex: 4,
                        child: Column(
                          children: [
                            _IncomingTrendCard(points: data.incomingTrend),
                            const SizedBox(height: 16),
                            _RecentActivityCard(
                              activities: data.recentActivities,
                            ),
                          ],
                        ),
                      ),
                    ],
                  )
                else ...[
                  _PendingApprovalsCard(
                    items: data.pendingApprovals,
                    onOpenApprovals: onOpenApprovals,
                    onOpenApproval: onOpenApproval,
                  ),
                  const SizedBox(height: 16),
                  _IncomingTrendCard(points: data.incomingTrend),
                  const SizedBox(height: 16),
                  _RecentActivityCard(activities: data.recentActivities),
                ],
              ],
            );
          },
        );
      },
    );
  }

  Widget _buildKpiGrid(DashboardSummary data, {required int columns}) {
    final cards = [
      _KpiCard(
        icon: Icons.approval_outlined,
        label: 'Approval pending',
        value: data.stats.pendingApprovals,
        onTap: onOpenApprovals,
      ),
      _KpiCard(
        icon: Icons.mark_email_unread_outlined,
        label: 'Inbox belum dibaca',
        value: data.stats.inboxUnread,
        onTap: onOpenInbox,
      ),
      _KpiCard(
        icon: Icons.send_outlined,
        label: 'Terkirim bulan ini',
        value: data.stats.sentThisMonth,
      ),
      _KpiCard(
        icon: Icons.archive_outlined,
        label: 'Arsip akses saya',
        value: data.stats.archivedTotal,
        onTap: onOpenArchive,
      ),
    ];
    final rows = <Widget>[];
    for (var start = 0; start < cards.length; start += columns) {
      if (rows.isNotEmpty) rows.add(const SizedBox(height: 12));
      rows.add(
        IntrinsicHeight(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              for (var i = start; i < start + columns; i++) ...[
                if (i > start) const SizedBox(width: 12),
                Expanded(
                  child: i < cards.length ? cards[i] : const SizedBox.shrink(),
                ),
              ],
            ],
          ),
        ),
      );
    }
    return Column(children: rows);
  }
}

class _DashboardHeader extends StatelessWidget {
  const _DashboardHeader({this.userName});

  final String? userName;

  String get _greeting {
    final hour = DateTime.now().hour;
    if (hour < 11) return 'Selamat pagi';
    if (hour < 15) return 'Selamat siang';
    if (hour < 18) return 'Selamat sore';
    return 'Selamat malam';
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final name = userName?.trim();
    final greeting = name == null || name.isEmpty
        ? _greeting
        : '$_greeting, ${name.split(' ').first}';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Ringkasan', style: theme.textTheme.headlineSmall),
        const SizedBox(height: 4),
        Text(
          '$greeting • ${formatFullDateId(DateTime.now())}',
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}

class _KpiCard extends StatelessWidget {
  const _KpiCard({
    required this.icon,
    required this.label,
    required this.value,
    this.onTap,
  });

  final IconData icon;
  final String label;
  final int value;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    return Card(
      color: scheme.surfaceContainerLow,
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              CircleAvatar(
                backgroundColor: scheme.secondaryContainer,
                foregroundColor: scheme.onSecondaryContainer,
                child: Icon(icon),
              ),
              const SizedBox(height: 12),
              Text('$value', style: theme.textTheme.headlineMedium),
              const SizedBox(height: 2),
              Text(
                label,
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: scheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _DashboardSectionCard extends StatelessWidget {
  const _DashboardSectionCard({
    required this.title,
    required this.child,
    this.trailing,
    this.onSeeAll,
  });

  final String title;
  final Widget child;
  final Widget? trailing;
  final VoidCallback? onSeeAll;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(title, style: theme.textTheme.titleMedium),
                ),
                if (trailing != null) trailing!,
                if (onSeeAll != null)
                  TextButton(
                    onPressed: onSeeAll,
                    child: const Text('Lihat semua'),
                  ),
              ],
            ),
            const SizedBox(height: 8),
            child,
          ],
        ),
      ),
    );
  }
}

class _PendingApprovalsCard extends StatelessWidget {
  const _PendingApprovalsCard({
    required this.items,
    required this.onOpenApprovals,
    required this.onOpenApproval,
  });

  final List<DashboardPendingApproval> items;
  final VoidCallback onOpenApprovals;
  final void Function(String letterId, String stepId) onOpenApproval;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return _DashboardSectionCard(
      title: 'Menunggu approval saya',
      onSeeAll: onOpenApprovals,
      child: items.isEmpty
          ? const _EmptyHint('Tidak ada approval menunggu.')
          : Column(
              children: [
                for (final item in items)
                  ListTile(
                    contentPadding: EdgeInsets.zero,
                    leading: CircleAvatar(
                      backgroundColor: scheme.secondaryContainer,
                      foregroundColor: scheme.onSecondaryContainer,
                      child: const Icon(Icons.approval_outlined),
                    ),
                    title: Text(
                      item.subject,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    subtitle: Text(
                      '${item.creatorName} • '
                      '${formatRelativeTime(item.updatedAt)}',
                    ),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: item.letterId.isNotEmpty
                        ? () => onOpenApproval(item.letterId, item.stepId)
                        : onOpenApprovals,
                  ),
              ],
            ),
    );
  }
}

class _RecentActivityCard extends StatelessWidget {
  const _RecentActivityCard({required this.activities});

  final List<DashboardActivity> activities;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return _DashboardSectionCard(
      title: 'Aktivitas terakhir',
      child: activities.isEmpty
          ? const _EmptyHint('Belum ada aktivitas.')
          : Column(
              children: [
                for (final activity in activities)
                  ListTile(
                    contentPadding: EdgeInsets.zero,
                    leading: Icon(
                      _activityIcon(activity.eventType),
                      color: scheme.onSurfaceVariant,
                    ),
                    title: Text(
                      activity.title,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    subtitle: Text(formatRelativeTime(activity.createdAt)),
                  ),
              ],
            ),
    );
  }
}

IconData _activityIcon(String eventType) {
  return switch (eventType) {
    'letter_incoming' => Icons.mark_email_unread_outlined,
    'approval_waiting' => Icons.approval_outlined,
    'approval_result' => Icons.fact_check_outlined,
    'disposition_assigned' => Icons.assignment_ind_outlined,
    'disposition_updated' => Icons.task_alt_outlined,
    'sla_reminder' => Icons.schedule_outlined,
    'sla_escalation' => Icons.warning_amber_outlined,
    _ => Icons.notifications_outlined,
  };
}

class _EmptyHint extends StatelessWidget {
  const _EmptyHint(this.message);

  final String message;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 12),
      child: Text(
        message,
        style: theme.textTheme.bodyMedium?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
        ),
      ),
    );
  }
}

class _IncomingTrendCard extends StatefulWidget {
  const _IncomingTrendCard({required this.points});

  final List<DashboardTrendPoint> points;

  @override
  State<_IncomingTrendCard> createState() => _IncomingTrendCardState();
}

class _IncomingTrendCardState extends State<_IncomingTrendCard> {
  static const _plotHeight = 140.0;
  static const _barGap = 2.0;

  int? _selectedIndex;

  int get _total => widget.points.fold(0, (sum, point) => sum + point.total);

  String _pointLabel(DashboardTrendPoint point) {
    final date = DateTime.tryParse(point.date);
    final dateLabel = date == null ? point.date : formatShortDateId(date);
    return '$dateLabel • ${point.total} surat';
  }

  String _edgeDateLabel(DashboardTrendPoint point) {
    final date = DateTime.tryParse(point.date);
    return date == null ? point.date : formatShortDateId(date);
  }

  void _selectAt(double dx, double width) {
    final points = widget.points;
    if (points.isEmpty) return;
    final slot = width / points.length;
    final index = (dx / slot).floor().clamp(0, points.length - 1);
    if (index != _selectedIndex) {
      setState(() => _selectedIndex = index);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final points = widget.points;
    final allZero = points.every((point) => point.total == 0);
    final selected = _selectedIndex != null && _selectedIndex! < points.length
        ? points[_selectedIndex!]
        : null;

    return _DashboardSectionCard(
      title: 'Surat masuk 30 hari',
      trailing: Text('Total: $_total', style: theme.textTheme.labelLarge),
      child: points.isEmpty || allZero
          ? const _EmptyHint('Belum ada surat masuk 30 hari terakhir.')
          : Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SizedBox(
                  height: 20,
                  child: Text(
                    selected == null
                        ? 'Ketuk grafik untuk detail harian.'
                        : _pointLabel(selected),
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: selected == null
                          ? scheme.onSurfaceVariant
                          : scheme.onSurface,
                    ),
                  ),
                ),
                const SizedBox(height: 4),
                LayoutBuilder(
                  builder: (context, constraints) {
                    final width = constraints.maxWidth;
                    return GestureDetector(
                      onTapDown: (details) =>
                          _selectAt(details.localPosition.dx, width),
                      onHorizontalDragUpdate: (details) =>
                          _selectAt(details.localPosition.dx, width),
                      child: CustomPaint(
                        size: Size(width, _plotHeight),
                        painter: _TrendBarPainter(
                          points: points,
                          selectedIndex: _selectedIndex,
                          gap: _barGap,
                          barColor: scheme.primary,
                          selectedColor: scheme.tertiary,
                          baselineColor: scheme.outlineVariant,
                        ),
                      ),
                    );
                  },
                ),
                const SizedBox(height: 4),
                Row(
                  children: [
                    Text(
                      _edgeDateLabel(points.first),
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                    const Spacer(),
                    Text(
                      _edgeDateLabel(points.last),
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ),
              ],
            ),
    );
  }
}

class _TrendBarPainter extends CustomPainter {
  _TrendBarPainter({
    required this.points,
    required this.selectedIndex,
    required this.gap,
    required this.barColor,
    required this.selectedColor,
    required this.baselineColor,
  });

  final List<DashboardTrendPoint> points;
  final int? selectedIndex;
  final double gap;
  final Color barColor;
  final Color selectedColor;
  final Color baselineColor;

  @override
  void paint(Canvas canvas, Size size) {
    canvas.drawLine(
      Offset(0, size.height - 0.5),
      Offset(size.width, size.height - 0.5),
      Paint()
        ..color = baselineColor
        ..strokeWidth = 1,
    );
    if (points.isEmpty) return;
    var maxTotal = 0;
    for (final point in points) {
      if (point.total > maxTotal) maxTotal = point.total;
    }
    if (maxTotal == 0) return;

    final barWidth = (size.width - gap * (points.length - 1)) / points.length;
    final paint = Paint();
    for (var i = 0; i < points.length; i++) {
      final total = points[i].total;
      if (total == 0) continue;
      final height = (size.height - 1) * total / maxTotal;
      final left = i * (barWidth + gap);
      paint.color = i == selectedIndex ? selectedColor : barColor;
      canvas.drawRRect(
        RRect.fromRectAndCorners(
          Rect.fromLTWH(left, size.height - 1 - height, barWidth, height),
          topLeft: const Radius.circular(2),
          topRight: const Radius.circular(2),
        ),
        paint,
      );
    }
  }

  @override
  bool shouldRepaint(_TrendBarPainter oldDelegate) {
    return oldDelegate.points != points ||
        oldDelegate.selectedIndex != selectedIndex ||
        oldDelegate.barColor != barColor ||
        oldDelegate.selectedColor != selectedColor ||
        oldDelegate.baselineColor != baselineColor;
  }
}
