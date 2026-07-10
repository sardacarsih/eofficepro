import 'package:eoffice_mobile/core/exceptions/app_exception.dart';
import 'package:eoffice_mobile/core/network/api_client.dart';
import 'package:eoffice_mobile/core/services/authenticated_file_opener.dart';
import 'package:eoffice_mobile/core/utils/date_format.dart';
import 'package:eoffice_mobile/core/widgets/async_state_view.dart';
import 'package:eoffice_mobile/features/auth/domain/user.dart';
import 'package:eoffice_mobile/features/auth/presentation/auth_controller.dart';
import 'package:eoffice_mobile/features/home/data/dashboard_repository.dart';
import 'package:eoffice_mobile/features/home/presentation/dashboard_pane.dart';
import 'package:eoffice_mobile/features/letters/data/letter_repository.dart';
import 'package:eoffice_mobile/features/letters/domain/disposition_models.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:eoffice_mobile/features/letters/domain/letter_models.dart';
import 'package:eoffice_mobile/features/letters/domain/search_models.dart';
import 'package:eoffice_mobile/features/letters/presentation/signature_pad.dart';
import 'package:eoffice_mobile/shared/pagination.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

enum WorkSection { dashboard, approvals, inbox, dispositions, sent, search }

const _composeRoles = {'admin', 'creator', 'secretary'};

String _draftLetterStatusLabel(DraftLetterStatus status) {
  return switch (status) {
    DraftLetterStatus.draft => 'Draft',
    DraftLetterStatus.submitted => 'Diajukan',
    DraftLetterStatus.inApproval => 'Menunggu approval',
    DraftLetterStatus.revision => 'Revisi',
    DraftLetterStatus.approved => 'Disetujui',
    DraftLetterStatus.published => 'Terbit',
    DraftLetterStatus.cancelled => 'Dibatalkan',
    DraftLetterStatus.archived => 'Arsip',
  };
}

bool canComposeLetters(User? user) {
  return user != null &&
      user.positions.isNotEmpty &&
      user.roles.any(_composeRoles.contains);
}

WorkSection workSectionFromRouteValue(String? value) {
  return switch (value) {
    'dashboard' => WorkSection.dashboard,
    'inbox' => WorkSection.inbox,
    'dispositions' => WorkSection.dispositions,
    'sent' => WorkSection.sent,
    'search' => WorkSection.search,
    _ => WorkSection.approvals,
  };
}

class TabletLayoutSpec {
  const TabletLayoutSpec(this.size);

  static const twoPaneMinWidth = 900.0;
  static const minListPaneWidth = 380.0;
  static const maxListPaneWidth = 420.0;
  static const maxDialogWidth = 640.0;

  final Size size;

  bool get useTwoPane =>
      size.width >= twoPaneMinWidth && size.width > size.height;

  double get listPaneWidth {
    final target = size.width * 0.34;
    return target.clamp(minListPaneWidth, maxListPaneWidth).toDouble();
  }

  EdgeInsets get detailPadding => EdgeInsets.all(useTwoPane ? 24 : 16);

  BoxConstraints get dialogConstraints {
    final maxWidth = (size.width - 48).clamp(320.0, maxDialogWidth).toDouble();
    final maxHeight = (size.height * 0.82).clamp(360.0, 720.0).toDouble();
    return BoxConstraints(maxWidth: maxWidth, maxHeight: maxHeight);
  }
}

class HomePage extends ConsumerStatefulWidget {
  const HomePage({
    this.initialSection = WorkSection.approvals,
    this.initialLetterId,
    super.key,
  });

  final WorkSection initialSection;
  final String? initialLetterId;

  @override
  ConsumerState<HomePage> createState() => _HomePageState();
}

class _HomePageState extends ConsumerState<HomePage> {
  late WorkSection _section;
  String? _selectedLetterId;
  String? _selectedApprovalStepId;
  DispositionInboxItem? _selectedDisposition;
  var _searchQuery = '';

  @override
  void initState() {
    super.initState();
    _section = widget.initialSection;
    _selectedLetterId = _normalizedInitialLetterId;
  }

  @override
  void didUpdateWidget(HomePage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.initialSection != widget.initialSection ||
        oldWidget.initialLetterId != widget.initialLetterId) {
      setState(() {
        _section = widget.initialSection;
        _selectedLetterId = _normalizedInitialLetterId;
        _selectedApprovalStepId = null;
        _selectedDisposition = null;
      });
    }
  }

  String? get _normalizedInitialLetterId {
    final letterId = widget.initialLetterId?.trim();
    if (letterId == null || letterId.isEmpty) return null;
    return letterId;
  }

  void _selectSection(WorkSection section) {
    setState(() {
      _section = section;
      _selectedLetterId = null;
      _selectedApprovalStepId = null;
      _selectedDisposition = null;
    });
  }

  void _selectLetter(
    String letterId, {
    String? approvalStepId,
    DispositionInboxItem? disposition,
  }) {
    setState(() {
      _selectedLetterId = letterId;
      _selectedApprovalStepId = approvalStepId;
      _selectedDisposition = disposition;
    });
  }

  void _openApproval(String letterId, String stepId) {
    setState(() {
      _section = WorkSection.approvals;
      _selectedLetterId = letterId;
      _selectedApprovalStepId = stepId;
      _selectedDisposition = null;
    });
  }

  void _refreshCurrent() {
    ref.invalidate(dashboardSummaryProvider);
    ref.invalidate(approvalInboxProvider);
    ref.invalidate(incomingLettersProvider);
    ref.invalidate(dispositionInboxProvider);
    ref.invalidate(sentLettersProvider);
    if (_selectedLetterId case final id?) {
      ref.invalidate(letterDetailProvider(id));
      ref.invalidate(letterDispositionsProvider(id));
    }
  }

  Future<void> _openCompose() async {
    await context.pushNamed('compose');
    if (mounted) _refreshCurrent();
  }

  @override
  Widget build(BuildContext context) {
    final auth = ref.watch(authControllerProvider);
    final user = auth.valueOrNull?.user;

    return LayoutBuilder(
      builder: (context, constraints) {
        final spec = TabletLayoutSpec(
          Size(constraints.maxWidth, constraints.maxHeight),
        );
        final showComposeAction = canComposeLetters(user) &&
            (spec.useTwoPane || _selectedLetterId == null);
        return Scaffold(
          appBar: AppBar(
            leading: !spec.useTwoPane && _selectedLetterId != null
                ? IconButton(
                    tooltip: 'Kembali ke daftar',
                    onPressed: () => setState(() {
                      _selectedLetterId = null;
                      _selectedApprovalStepId = null;
                      _selectedDisposition = null;
                    }),
                    icon: const Icon(Icons.arrow_back),
                  )
                : null,
            title: const Text('eOffice Pro'),
            actions: [
              if (user != null)
                ConstrainedBox(
                  constraints: BoxConstraints(
                    maxWidth: spec.useTwoPane ? 240 : 120,
                  ),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 12),
                    child: Center(
                      child: Text(
                        user.fullName,
                        overflow: TextOverflow.ellipsis,
                        maxLines: 1,
                      ),
                    ),
                  ),
                ),
              IconButton(
                tooltip: 'Profil',
                onPressed: () => context.pushNamed('profile'),
                icon: const Icon(Icons.account_circle_outlined),
              ),
              IconButton(
                tooltip: 'Muat ulang',
                onPressed: _refreshCurrent,
                icon: const Icon(Icons.refresh),
              ),
              IconButton(
                tooltip: 'Keluar',
                onPressed: () =>
                    ref.read(authControllerProvider.notifier).logout(),
                icon: const Icon(Icons.logout),
              ),
            ],
          ),
          body: SafeArea(
            child:
                spec.useTwoPane ? _buildTwoPaneBody(spec) : _buildCompactBody(),
          ),
          floatingActionButton: !showComposeAction
              ? null
              : spec.useTwoPane
                  ? FloatingActionButton.extended(
                      onPressed: _openCompose,
                      icon: const Icon(Icons.edit_outlined),
                      label: const Text('Tulis Surat'),
                    )
                  : FloatingActionButton(
                      onPressed: _openCompose,
                      tooltip: 'Tulis Surat',
                      child: const Icon(Icons.edit_outlined),
                    ),
          bottomNavigationBar: spec.useTwoPane
              ? null
              : NavigationBar(
                  selectedIndex: WorkSection.values.indexOf(_section),
                  onDestinationSelected: (index) {
                    _selectSection(WorkSection.values[index]);
                  },
                  destinations: const [
                    NavigationDestination(
                      icon: Icon(Icons.dashboard_outlined),
                      selectedIcon: Icon(Icons.dashboard),
                      label: 'Ringkas',
                    ),
                    NavigationDestination(
                      icon: Icon(Icons.approval_outlined),
                      selectedIcon: Icon(Icons.approval),
                      label: 'Approval',
                    ),
                    NavigationDestination(
                      icon: Icon(Icons.inbox_outlined),
                      selectedIcon: Icon(Icons.inbox),
                      label: 'Inbox',
                    ),
                    NavigationDestination(
                      icon: Icon(Icons.task_alt_outlined),
                      selectedIcon: Icon(Icons.task_alt),
                      label: 'Disposisi',
                    ),
                    NavigationDestination(
                      icon: Icon(Icons.send_outlined),
                      selectedIcon: Icon(Icons.send),
                      label: 'Terkirim',
                    ),
                    NavigationDestination(
                      icon: Icon(Icons.search),
                      selectedIcon: Icon(Icons.manage_search),
                      label: 'Cari',
                    ),
                  ],
                ),
        );
      },
    );
  }

  Widget _buildTwoPaneBody(TabletLayoutSpec spec) {
    return Row(
      children: [
        _buildRail(),
        const VerticalDivider(width: 1),
        if (_section == WorkSection.dashboard)
          Expanded(child: _buildDashboardPane())
        else ...[
          SizedBox(width: spec.listPaneWidth, child: _buildListPane()),
          const VerticalDivider(width: 1),
          Expanded(child: _buildDetailPane()),
        ],
      ],
    );
  }

  Widget _buildRail() {
    return NavigationRail(
      selectedIndex: WorkSection.values.indexOf(_section),
      onDestinationSelected: (index) {
        _selectSection(WorkSection.values[index]);
      },
      labelType: NavigationRailLabelType.all,
      destinations: const [
        NavigationRailDestination(
          icon: Icon(Icons.dashboard_outlined),
          selectedIcon: Icon(Icons.dashboard),
          label: Text('Ringkas'),
        ),
        NavigationRailDestination(
          icon: Icon(Icons.approval_outlined),
          selectedIcon: Icon(Icons.approval),
          label: Text('Approval'),
        ),
        NavigationRailDestination(
          icon: Icon(Icons.inbox_outlined),
          selectedIcon: Icon(Icons.inbox),
          label: Text('Inbox'),
        ),
        NavigationRailDestination(
          icon: Icon(Icons.task_alt_outlined),
          selectedIcon: Icon(Icons.task_alt),
          label: Text('Disposisi'),
        ),
        NavigationRailDestination(
          icon: Icon(Icons.send_outlined),
          selectedIcon: Icon(Icons.send),
          label: Text('Terkirim'),
        ),
        NavigationRailDestination(
          icon: Icon(Icons.search),
          selectedIcon: Icon(Icons.manage_search),
          label: Text('Cari'),
        ),
      ],
    );
  }

  Widget _buildDashboardPane() {
    return DashboardPane(
      onOpenApprovals: () => _selectSection(WorkSection.approvals),
      onOpenInbox: () => _selectSection(WorkSection.inbox),
      onOpenArchive: () => _selectSection(WorkSection.search),
      onOpenApproval: _openApproval,
    );
  }

  Widget _buildCompactBody() {
    return _selectedLetterId == null ? _buildListPane() : _buildDetailPane();
  }

  Widget _buildDetailPane() {
    if (_selectedLetterId == null) return const _EmptyDetailPane();
    return LetterDetailPane(
      letterId: _selectedLetterId!,
      approvalStepId: _selectedApprovalStepId,
      disposition: _selectedDisposition,
      onChanged: _refreshCurrent,
    );
  }

  Widget _buildListPane() {
    return switch (_section) {
      WorkSection.dashboard => _buildDashboardPane(),
      WorkSection.approvals => ApprovalListPane(
          selectedLetterId: _selectedLetterId,
          onSelected: (item) => _selectLetter(
            item.letterId,
            approvalStepId: item.stepId,
          ),
        ),
      WorkSection.inbox => IncomingListPane(
          selectedLetterId: _selectedLetterId,
          onSelected: (item) => _selectLetter(item.id),
        ),
      WorkSection.dispositions => DispositionListPane(
          selectedRecipientId: _selectedDisposition?.recipientId,
          onSelected: (item) => _selectLetter(
            item.letterId,
            disposition: item,
          ),
        ),
      WorkSection.sent => SentListPane(
          selectedLetterId: _selectedLetterId,
          onSelected: (item) => _selectLetter(item.id),
        ),
      WorkSection.search => SearchPane(
          query: _searchQuery,
          selectedLetterId: _selectedLetterId,
          onQueryChanged: (value) => setState(() => _searchQuery = value),
          onSelected: (item) => _selectLetter(item.id),
        ),
    };
  }
}

class ApprovalListPane extends ConsumerWidget {
  const ApprovalListPane({
    required this.selectedLetterId,
    required this.onSelected,
    super.key,
  });

  final String? selectedLetterId;
  final ValueChanged<ApprovalInboxItem> onSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final approvals = ref.watch(approvalInboxProvider);
    return AsyncStateView<Paginated<ApprovalInboxItem>>(
      value: approvals,
      onRetry: () => ref.invalidate(approvalInboxProvider),
      data: (page) {
        if (page.data.isEmpty) {
          return const _InlineEmpty(message: 'Tidak ada approval pending.');
        }
        return ListView.separated(
          padding: const EdgeInsets.all(12),
          itemCount: page.data.length,
          separatorBuilder: (_, __) => const SizedBox(height: 8),
          itemBuilder: (context, index) {
            final item = page.data[index];
            return _LetterListCard(
              selected: item.letterId == selectedLetterId,
              title: item.subject,
              subtitle: '${item.creatorName} - ${item.creatorPosition}',
              meta: '${item.companyCode}/${item.letterTypeCode} '
                  '${formatDateTime(item.updatedAt)}',
              body: item.bodyPlain,
              priority: item.priority,
              onTap: () => onSelected(item),
            );
          },
        );
      },
    );
  }
}

class IncomingListPane extends ConsumerWidget {
  const IncomingListPane({
    required this.selectedLetterId,
    required this.onSelected,
    super.key,
  });

  final String? selectedLetterId;
  final ValueChanged<IncomingLetter> onSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final incoming = ref.watch(incomingLettersProvider);
    return AsyncStateView<Paginated<IncomingLetter>>(
      value: incoming,
      onRetry: () => ref.invalidate(incomingLettersProvider),
      data: (page) {
        if (page.data.isEmpty) {
          return const _InlineEmpty(message: 'Surat masuk kosong.');
        }
        return ListView.separated(
          padding: const EdgeInsets.all(12),
          itemCount: page.data.length,
          separatorBuilder: (_, __) => const SizedBox(height: 8),
          itemBuilder: (context, index) {
            final item = page.data[index];
            return _LetterListCard(
              selected: item.id == selectedLetterId,
              title: item.subject,
              subtitle: '${item.creatorName} - ${item.creatorPositionTitle}',
              meta: '${item.companyCode}/${item.letterTypeCode} '
                  '${item.isRead ? "dibaca" : "baru"}',
              body: item.bodyPlain,
              priority: item.priority,
              onTap: () => onSelected(item),
            );
          },
        );
      },
    );
  }
}

class DispositionListPane extends ConsumerWidget {
  const DispositionListPane({
    required this.selectedRecipientId,
    required this.onSelected,
    super.key,
  });

  final String? selectedRecipientId;
  final ValueChanged<DispositionInboxItem> onSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final dispositions = ref.watch(dispositionInboxProvider);
    return AsyncStateView<Paginated<DispositionInboxItem>>(
      value: dispositions,
      onRetry: () => ref.invalidate(dispositionInboxProvider),
      data: (page) {
        if (page.data.isEmpty) {
          return const _InlineEmpty(message: 'Tidak ada disposisi pending.');
        }
        return ListView.separated(
          padding: const EdgeInsets.all(12),
          itemCount: page.data.length,
          separatorBuilder: (_, __) => const SizedBox(height: 8),
          itemBuilder: (context, index) {
            final item = page.data[index];
            return _LetterListCard(
              selected: item.recipientId == selectedRecipientId,
              title: item.letterSubject,
              subtitle: 'Dari ${item.fromPositionTitle}',
              meta: '${item.status} - tenggat ${item.dueDate ?? "-"}',
              body: item.instruction,
              priority: item.status == 'open' ? 'urgent' : 'normal',
              onTap: () => onSelected(item),
            );
          },
        );
      },
    );
  }
}

class SentListPane extends ConsumerWidget {
  const SentListPane({
    required this.selectedLetterId,
    required this.onSelected,
    super.key,
  });

  final String? selectedLetterId;
  final ValueChanged<DraftLetter> onSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sentLetters = ref.watch(sentLettersProvider);
    return AsyncStateView<Paginated<DraftLetter>>(
      value: sentLetters,
      onRetry: () => ref.invalidate(sentLettersProvider),
      data: (page) {
        final items = page.data
            .where(
              (item) =>
                  item.status != DraftLetterStatus.draft &&
                  item.status != DraftLetterStatus.revision,
            )
            .toList();
        if (items.isEmpty) {
          return const _InlineEmpty(message: 'Belum ada surat terkirim.');
        }
        return ListView.separated(
          padding: const EdgeInsets.all(12),
          itemCount: items.length,
          separatorBuilder: (_, __) => const SizedBox(height: 8),
          itemBuilder: (context, index) {
            final item = items[index];
            final status = _draftLetterStatusLabel(item.status);
            final subject = item.subject.trim().isEmpty
                ? 'Tanpa perihal'
                : item.subject.trim();
            return _LetterListCard(
              selected: item.id == selectedLetterId,
              title: subject,
              subtitle: '$status - ${item.creatorPositionTitle}',
              meta: '${item.companyCode}/${item.letterTypeCode} '
                  '${item.letterNumber ?? "-"} - ${formatDateTime(item.updatedAt)}',
              body: item.bodyPlain,
              priority: item.priority.wireValue,
              onTap: () => onSelected(item),
            );
          },
        );
      },
    );
  }
}

class SearchPane extends ConsumerWidget {
  const SearchPane({
    required this.query,
    required this.selectedLetterId,
    required this.onQueryChanged,
    required this.onSelected,
    super.key,
  });

  final String query;
  final String? selectedLetterId;
  final ValueChanged<String> onQueryChanged;
  final ValueChanged<LetterSearchResult> onSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final results = ref.watch(letterSearchProvider(query));
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(12),
          child: TextField(
            decoration: const InputDecoration(
              labelText: 'Cari arsip',
              prefixIcon: Icon(Icons.search),
            ),
            onChanged: onQueryChanged,
          ),
        ),
        Expanded(
          child: query.trim().isEmpty
              ? const _InlineEmpty(message: 'Masukkan kata kunci pencarian.')
              : AsyncStateView<List<LetterSearchResult>>(
                  value: results,
                  onRetry: () => ref.invalidate(letterSearchProvider(query)),
                  data: (items) {
                    if (items.isEmpty) {
                      return const _InlineEmpty(message: 'Tidak ada hasil.');
                    }
                    return ListView.separated(
                      padding: const EdgeInsets.all(12),
                      itemCount: items.length,
                      separatorBuilder: (_, __) => const SizedBox(height: 8),
                      itemBuilder: (context, index) {
                        final item = items[index];
                        return _LetterListCard(
                          selected: item.id == selectedLetterId,
                          title: item.subject,
                          subtitle: '${item.creatorName} - ${item.status}',
                          meta: '${item.companyCode}/${item.letterTypeCode} '
                              '${item.letterNumber ?? "-"}',
                          body: item.snippet,
                          priority: 'normal',
                          onTap: () => onSelected(item),
                        );
                      },
                    );
                  },
                ),
        ),
      ],
    );
  }
}

class LetterDetailPane extends ConsumerWidget {
  const LetterDetailPane({
    required this.letterId,
    required this.onChanged,
    this.approvalStepId,
    this.disposition,
    super.key,
  });

  final String letterId;
  final String? approvalStepId;
  final DispositionInboxItem? disposition;
  final VoidCallback onChanged;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final detail = ref.watch(letterDetailProvider(letterId));
    final layout = TabletLayoutSpec(MediaQuery.sizeOf(context));
    return AsyncStateView<LetterDetail>(
      value: detail,
      onRetry: () => ref.invalidate(letterDetailProvider(letterId)),
      data: (letter) {
        final isPublished = letter.status == 'published';
        final dispositions = isPublished
            ? ref.watch(letterDispositionsProvider(letterId))
            : null;
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (approvalStepId != null)
              ApprovalActionBar(
                stepId: approvalStepId!,
                onChanged: onChanged,
              ),
            if (disposition != null)
              DispositionActionBar(
                item: disposition!,
                onChanged: onChanged,
              ),
            Expanded(
              child: ListView(
                padding: layout.detailPadding,
                children: [
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      _Chip(label: letter.companyCode),
                      _Chip(label: letter.letterTypeCode),
                      _Chip(label: letter.classification),
                      _Chip(label: letter.priority),
                      _Chip(label: letter.status),
                    ],
                  ),
                  const SizedBox(height: 16),
                  Text(
                    letter.subject,
                    style: Theme.of(context).textTheme.headlineSmall,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    '${letter.creatorName} - ${letter.creatorPositionTitle}',
                    style: Theme.of(context).textTheme.bodyMedium,
                  ),
                  const SizedBox(height: 8),
                  Text('Nomor: ${letter.letterNumber ?? "-"}'),
                  Text('Dibuat: ${formatDateTime(letter.createdAt)}'),
                  if (letter.publishedAt != null)
                    Text('Terbit: ${formatDateTime(letter.publishedAt)}'),
                  const SizedBox(height: 16),
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      if (letter.status == 'published')
                        OutlinedButton.icon(
                          onPressed: () => _openSecureFile(
                            context,
                            ref,
                            '/letters/view/${letter.id}/final-pdf',
                            '${letter.letterNumber ?? 'surat'}.pdf',
                          ),
                          icon: const Icon(Icons.picture_as_pdf_outlined),
                          label: const Text('PDF final'),
                        ),
                      if (letter.verifyUrl != null)
                        OutlinedButton.icon(
                          onPressed: () => _openUrl(letter.verifyUrl!),
                          icon: const Icon(Icons.qr_code_2),
                          label: const Text('Verifikasi'),
                        ),
                      if (isPublished)
                        FilledButton.tonalIcon(
                          onPressed: () => showDialog<void>(
                            context: context,
                            builder: (_) => CreateDispositionDialog(
                              letterId: letter.id,
                              onCreated: onChanged,
                            ),
                          ),
                          icon: const Icon(Icons.low_priority),
                          label: const Text('Disposisi'),
                        ),
                    ],
                  ),
                  const SizedBox(height: 24),
                  const _SectionTitle('Isi surat'),
                  SelectableText(
                    letter.bodyPlain.isEmpty ? '-' : letter.bodyPlain,
                    style: Theme.of(context).textTheme.bodyLarge,
                  ),
                  const SizedBox(height: 24),
                  const _SectionTitle('Penerima'),
                  for (final recipient in letter.recipients)
                    ListTile(
                      dense: true,
                      contentPadding: EdgeInsets.zero,
                      leading: Icon(
                        recipient.type == 'to'
                            ? Icons.person_outline
                            : Icons.people_outline,
                      ),
                      title: Text(recipient.label),
                      subtitle: Text(recipient.type.toUpperCase()),
                    ),
                  if (letter.recipients.isEmpty)
                    const _InlineEmpty(message: 'Belum ada penerima.'),
                  const SizedBox(height: 16),
                  const _SectionTitle('Lampiran'),
                  for (final attachment in letter.attachments)
                    ListTile(
                      contentPadding: EdgeInsets.zero,
                      leading: const Icon(Icons.attach_file),
                      title: Text(attachment.fileName),
                      subtitle: Text(
                        '${attachment.mimeType} - ${attachment.scanStatus}',
                      ),
                      trailing: attachment.scanStatus != 'clean'
                          ? null
                          : IconButton(
                              tooltip: 'Buka lampiran',
                              icon: const Icon(Icons.open_in_new),
                              onPressed: () => _openSecureFile(
                                context,
                                ref,
                                '/letters/view/${letter.id}/attachments/${attachment.id}/download',
                                attachment.fileName,
                              ),
                            ),
                    ),
                  if (letter.attachments.isEmpty)
                    const _InlineEmpty(message: 'Tidak ada lampiran.'),
                  const SizedBox(height: 16),
                  const _SectionTitle('Timeline approval'),
                  for (final step in letter.approvalSteps)
                    ListTile(
                      dense: true,
                      contentPadding: EdgeInsets.zero,
                      leading: CircleAvatar(child: Text('${step.stepOrder}')),
                      title: Text(step.positionTitle),
                      subtitle: Text(
                        '${step.status} - SLA ${formatDateTime(step.slaDeadline)}',
                      ),
                    ),
                  for (final action in letter.approvalActions)
                    ListTile(
                      dense: true,
                      contentPadding: EdgeInsets.zero,
                      leading: const Icon(Icons.history),
                      title: Text('${action.actorName} - ${action.action}'),
                      subtitle: Text(
                        '${formatDateTime(action.createdAt)}'
                        '${action.note == null ? "" : "\n${action.note}"}',
                      ),
                    ),
                  const SizedBox(height: 16),
                  const _SectionTitle('Disposisi'),
                  if (dispositions == null)
                    const _InlineEmpty(
                      message: 'Disposisi tersedia setelah surat terbit.',
                    )
                  else
                    AsyncStateView<Paginated<DispositionItem>>(
                      value: dispositions,
                      onRetry: () => ref.invalidate(
                        letterDispositionsProvider(letterId),
                      ),
                      data: (page) {
                        if (page.data.isEmpty) {
                          return const _InlineEmpty(
                            message: 'Belum ada disposisi.',
                          );
                        }
                        return Column(
                          children: [
                            for (final item in page.data)
                              Card(
                                child: Padding(
                                  padding: const EdgeInsets.all(12),
                                  child: Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Text(
                                        item.fromPositionTitle,
                                        style: Theme.of(context)
                                            .textTheme
                                            .titleSmall,
                                      ),
                                      const SizedBox(height: 4),
                                      Text(item.instruction),
                                      const SizedBox(height: 8),
                                      Wrap(
                                        spacing: 8,
                                        children: [
                                          for (final recipient
                                              in item.recipients)
                                            _Chip(
                                              label:
                                                  '${recipient.positionTitle}: ${recipient.status}',
                                            ),
                                        ],
                                      ),
                                    ],
                                  ),
                                ),
                              ),
                          ],
                        );
                      },
                    ),
                ],
              ),
            ),
          ],
        );
      },
    );
  }

  Future<void> _openUrl(String value) async {
    final uri = Uri.parse(value);
    await launchUrl(uri, mode: LaunchMode.externalApplication);
  }

  Future<void> _openSecureFile(
    BuildContext context,
    WidgetRef ref,
    String endpoint,
    String fileName,
  ) async {
    try {
      await AuthenticatedFileOpener(ref.read(dioProvider))
          .open(endpoint, fileName);
    } on AppException catch (error) {
      if (context.mounted) {
        ScaffoldMessenger.of(context)
          ..hideCurrentSnackBar()
          ..showSnackBar(SnackBar(content: Text(error.message)));
      }
    }
  }
}

class ApprovalActionBar extends ConsumerStatefulWidget {
  const ApprovalActionBar({
    required this.stepId,
    required this.onChanged,
    super.key,
  });

  final String stepId;
  final VoidCallback onChanged;

  @override
  ConsumerState<ApprovalActionBar> createState() => _ApprovalActionBarState();
}

class _ApprovalActionBarState extends ConsumerState<ApprovalActionBar> {
  var _busy = false;

  Future<void> _act(String action) async {
    String? signatureImageBase64;
    String? note;
    if (action == 'approve') {
      signatureImageBase64 = await showDialog<String>(
        context: context,
        builder: (_) => const SignatureCaptureDialog(),
      );
    } else {
      note = await showDialog<String>(
        context: context,
        builder: (_) => NoteDialog(
          title: action == 'reject' ? 'Alasan tolak' : 'Catatan revisi',
        ),
      );
    }
    if (!mounted || (action != 'approve' && note == null)) return;
    if (action == 'approve' && signatureImageBase64 == null) return;

    setState(() => _busy = true);
    try {
      await ref.read(letterRepositoryProvider).actApproval(
            stepId: widget.stepId,
            action: action,
            note: note,
            signatureImageBase64: signatureImageBase64,
          );
      widget.onChanged();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Aksi approval berhasil')),
        );
      }
    } on AppException catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(error.message)),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Theme.of(context).colorScheme.surfaceContainerHighest,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Wrap(
          spacing: 8,
          runSpacing: 8,
          alignment: WrapAlignment.end,
          children: [
            OutlinedButton.icon(
              onPressed: _busy ? null : () => _act('request_revision'),
              icon: const Icon(Icons.edit_note),
              label: const Text('Minta revisi'),
            ),
            OutlinedButton.icon(
              onPressed: _busy ? null : () => _act('reject'),
              icon: const Icon(Icons.close),
              label: const Text('Tolak'),
            ),
            FilledButton.icon(
              onPressed: _busy ? null : () => _act('approve'),
              icon: _busy
                  ? const SizedBox.square(
                      dimension: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.check),
              label: const Text('Setujui'),
            ),
          ],
        ),
      ),
    );
  }
}

class DispositionActionBar extends ConsumerStatefulWidget {
  const DispositionActionBar({
    required this.item,
    required this.onChanged,
    super.key,
  });

  final DispositionInboxItem item;
  final VoidCallback onChanged;

  @override
  ConsumerState<DispositionActionBar> createState() =>
      _DispositionActionBarState();
}

class _DispositionActionBarState extends ConsumerState<DispositionActionBar> {
  var _busy = false;

  Future<void> _update(String status) async {
    final note = status == 'done'
        ? await showDialog<String>(
            context: context,
            builder: (_) => const NoteDialog(title: 'Laporan tindak lanjut'),
          )
        : null;
    if (!mounted || (status == 'done' && note == null)) return;

    setState(() => _busy = true);
    try {
      await ref.read(letterRepositoryProvider).updateDispositionStatus(
            recipientId: widget.item.recipientId,
            status: status,
            followupNote: note,
          );
      widget.onChanged();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Status disposisi diperbarui')),
        );
      }
    } on AppException catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(error.message)),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final done = widget.item.status == 'done';
    return Material(
      color: Theme.of(context).colorScheme.surfaceContainerHighest,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Wrap(
          spacing: 8,
          alignment: WrapAlignment.end,
          children: [
            OutlinedButton.icon(
              onPressed: _busy || done ? null : () => _update('in_progress'),
              icon: const Icon(Icons.play_arrow),
              label: const Text('Proses'),
            ),
            FilledButton.icon(
              onPressed: _busy || done ? null : () => _update('done'),
              icon: const Icon(Icons.task_alt),
              label: const Text('Selesai'),
            ),
          ],
        ),
      ),
    );
  }
}

class CreateDispositionDialog extends ConsumerStatefulWidget {
  const CreateDispositionDialog({
    required this.letterId,
    required this.onCreated,
    super.key,
  });

  final String letterId;
  final VoidCallback onCreated;

  @override
  ConsumerState<CreateDispositionDialog> createState() =>
      _CreateDispositionDialogState();
}

class _CreateDispositionDialogState
    extends ConsumerState<CreateDispositionDialog> {
  final _instructionController = TextEditingController();
  final _dueDateController = TextEditingController();
  final Set<String> _recipientIds = {};
  String? _fromPositionId;
  var _busy = false;

  @override
  void dispose() {
    _instructionController.dispose();
    _dueDateController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final instruction = _instructionController.text.trim();
    if (_fromPositionId == null ||
        instruction.isEmpty ||
        _recipientIds.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content:
              Text('Jabatan pengirim, instruksi, dan penerima wajib diisi'),
        ),
      );
      return;
    }

    setState(() => _busy = true);
    try {
      await ref.read(letterRepositoryProvider).createDisposition(
            letterId: widget.letterId,
            fromPositionId: _fromPositionId!,
            instruction: instruction,
            recipientPositionIds: _recipientIds.toList(),
            dueDate: _dueDateController.text.trim().isEmpty
                ? null
                : _dueDateController.text.trim(),
          );
      widget.onCreated();
      if (mounted) Navigator.of(context).pop();
    } on AppException catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(error.message)),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final auth = ref.watch(authControllerProvider).valueOrNull;
    final positions = ref.watch(positionsProvider);
    final myPositions = auth?.user?.positions ?? const [];
    _fromPositionId ??=
        myPositions.isEmpty ? null : myPositions.first.positionId;
    final layout = TabletLayoutSpec(MediaQuery.sizeOf(context));

    return AlertDialog(
      title: const Text('Buat disposisi'),
      content: ConstrainedBox(
        constraints: layout.dialogConstraints,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                initialValue: _fromPositionId,
                items: [
                  for (final position in myPositions)
                    DropdownMenuItem(
                      value: position.positionId,
                      child: Text(position.title),
                    ),
                ],
                onChanged: _busy
                    ? null
                    : (value) => setState(() => _fromPositionId = value),
                decoration:
                    const InputDecoration(labelText: 'Jabatan pengirim'),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _instructionController,
                maxLines: 4,
                decoration: const InputDecoration(labelText: 'Instruksi'),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _dueDateController,
                decoration: const InputDecoration(
                  labelText: 'Tenggat',
                  hintText: 'YYYY-MM-DD',
                ),
              ),
              const SizedBox(height: 16),
              Align(
                alignment: Alignment.centerLeft,
                child: Text(
                  'Penerima',
                  style: Theme.of(context).textTheme.titleSmall,
                ),
              ),
              const SizedBox(height: 8),
              AsyncStateView<Paginated<PositionOption>>(
                value: positions,
                onRetry: () => ref.invalidate(positionsProvider),
                data: (page) {
                  return Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      for (final position in page.data)
                        FilterChip(
                          selected: _recipientIds.contains(position.id),
                          label: Text(position.label),
                          onSelected: _busy
                              ? null
                              : (selected) {
                                  setState(() {
                                    if (selected) {
                                      _recipientIds.add(position.id);
                                    } else {
                                      _recipientIds.remove(position.id);
                                    }
                                  });
                                },
                        ),
                    ],
                  );
                },
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _busy ? null : () => Navigator.of(context).pop(),
          child: const Text('Batal'),
        ),
        FilledButton.icon(
          onPressed: _busy ? null : _submit,
          icon: const Icon(Icons.send),
          label: const Text('Kirim'),
        ),
      ],
    );
  }
}

class NoteDialog extends StatefulWidget {
  const NoteDialog({required this.title, super.key});

  final String title;

  @override
  State<NoteDialog> createState() => _NoteDialogState();
}

class _NoteDialogState extends State<NoteDialog> {
  final _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _submit() {
    final value = _controller.text.trim();
    if (value.isEmpty) return;
    Navigator.of(context).pop(value);
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.title),
      content: TextField(
        controller: _controller,
        maxLines: 4,
        autofocus: true,
        decoration: const InputDecoration(labelText: 'Catatan'),
        onSubmitted: (_) => _submit(),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Batal'),
        ),
        FilledButton(
          onPressed: _submit,
          child: const Text('Kirim'),
        ),
      ],
    );
  }
}

class _LetterListCard extends StatelessWidget {
  const _LetterListCard({
    required this.selected,
    required this.title,
    required this.subtitle,
    required this.meta,
    required this.body,
    required this.priority,
    required this.onTap,
  });

  final bool selected;
  final String title;
  final String subtitle;
  final String meta;
  final String body;
  final String priority;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Card(
      color: selected ? scheme.secondaryContainer : scheme.surfaceContainerLow,
      child: InkWell(
        borderRadius: BorderRadius.circular(8),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: Text(
                      title,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                  ),
                  if (priority == 'urgent')
                    const Padding(
                      padding: EdgeInsets.only(left: 8),
                      child: Icon(Icons.priority_high, size: 18),
                    ),
                ],
              ),
              const SizedBox(height: 6),
              Text(
                subtitle,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              const SizedBox(height: 4),
              Text(meta, style: Theme.of(context).textTheme.bodySmall),
              if (body.isNotEmpty) ...[
                const SizedBox(height: 8),
                Text(
                  body,
                  maxLines: 3,
                  overflow: TextOverflow.ellipsis,
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _SectionTitle extends StatelessWidget {
  const _SectionTitle(this.text);

  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Text(text, style: Theme.of(context).textTheme.titleMedium),
    );
  }
}

class _Chip extends StatelessWidget {
  const _Chip({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    return Chip(label: Text(label));
  }
}

class _InlineEmpty extends StatelessWidget {
  const _InlineEmpty({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Text(message, textAlign: TextAlign.center),
      ),
    );
  }
}

class _EmptyDetailPane extends StatelessWidget {
  const _EmptyDetailPane();

  @override
  Widget build(BuildContext context) {
    return const Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.description_outlined, size: 48),
          SizedBox(height: 12),
          Text('Pilih surat untuk melihat detail.'),
        ],
      ),
    );
  }
}
