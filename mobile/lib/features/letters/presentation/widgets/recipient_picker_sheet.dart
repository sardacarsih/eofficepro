import 'package:eoffice_mobile/features/letters/domain/draft_composer_state.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:flutter/material.dart';

Future<DraftRecipient?> showRecipientPickerSheet(
  BuildContext context,
  DraftComposerState state,
) {
  return showModalBottomSheet<DraftRecipient>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    useSafeArea: true,
    builder: (context) => _RecipientPickerSheet(state: state),
  );
}

class _RecipientPickerSheet extends StatefulWidget {
  const _RecipientPickerSheet({required this.state});

  final DraftComposerState state;

  @override
  State<_RecipientPickerSheet> createState() => _RecipientPickerSheetState();
}

class _RecipientPickerSheetState extends State<_RecipientPickerSheet> {
  final _searchController = TextEditingController();
  final _listController = ScrollController();

  DraftRecipientType _recipientType = DraftRecipientType.to;
  DraftRecipientTargetType _targetType = DraftRecipientTargetType.position;
  String _query = '';

  @override
  void dispose() {
    _searchController.dispose();
    _listController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final options = _filteredOptions;
    final keyboardInset = MediaQuery.viewInsetsOf(context).bottom;

    return AnimatedPadding(
      padding: EdgeInsets.only(bottom: keyboardInset),
      duration: const Duration(milliseconds: 180),
      curve: Curves.easeOut,
      child: FractionallySizedBox(
        heightFactor: 0.9,
        child: Scrollbar(
          controller: _listController,
          child: CustomScrollView(
            controller: _listController,
            keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
            slivers: [
              SliverToBoxAdapter(child: _buildHeader(context)),
              const SliverToBoxAdapter(child: Divider(height: 1)),
              SliverToBoxAdapter(child: _buildControls()),
              if (options.isEmpty)
                SliverToBoxAdapter(
                  child: _RecipientEmptyState(
                    hasQuery: _query.trim().isNotEmpty,
                    targetType: _targetType,
                    query: _query.trim(),
                  ),
                )
              else
                SliverPadding(
                  padding: const EdgeInsets.fromLTRB(8, 0, 8, 16),
                  sliver: SliverList(
                    delegate: SliverChildBuilderDelegate(
                      (context, index) {
                        if (index.isOdd) {
                          return const Divider(height: 1, indent: 72);
                        }
                        return _buildOption(
                          context,
                          options[index ~/ 2],
                        );
                      },
                      childCount: options.length * 2 - 1,
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildControls() {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SegmentedButton<DraftRecipientType>(
            expandedInsets: EdgeInsets.zero,
            showSelectedIcon: false,
            segments: const [
              ButtonSegment(
                value: DraftRecipientType.to,
                icon: Icon(Icons.person_outline),
                label: Text('To'),
              ),
              ButtonSegment(
                value: DraftRecipientType.cc,
                icon: Icon(Icons.people_outline),
                label: Text('CC'),
              ),
            ],
            selected: {_recipientType},
            onSelectionChanged: (selection) {
              setState(() => _recipientType = selection.first);
            },
          ),
          const SizedBox(height: 12),
          SegmentedButton<DraftRecipientTargetType>(
            expandedInsets: EdgeInsets.zero,
            showSelectedIcon: false,
            segments: const [
              ButtonSegment(
                value: DraftRecipientTargetType.position,
                icon: Icon(Icons.badge_outlined),
                label: Text('Jabatan'),
              ),
              ButtonSegment(
                value: DraftRecipientTargetType.orgUnit,
                icon: Icon(Icons.account_tree_outlined),
                label: Text('Unit'),
              ),
            ],
            selected: {_targetType},
            onSelectionChanged: (selection) {
              setState(() => _targetType = selection.first);
            },
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _searchController,
            textInputAction: TextInputAction.search,
            decoration: InputDecoration(
              labelText: 'Cari penerima',
              hintText: _targetType == DraftRecipientTargetType.position
                  ? 'Nama jabatan, unit, atau pemegang'
                  : 'Nama, kode, level, atau region unit',
              prefixIcon: const Icon(Icons.search),
              suffixIcon: _query.isEmpty
                  ? null
                  : IconButton(
                      tooltip: 'Hapus pencarian',
                      onPressed: () {
                        _searchController.clear();
                        setState(() => _query = '');
                      },
                      icon: const Icon(Icons.clear),
                    ),
            ),
            onChanged: (value) => setState(() => _query = value),
          ),
        ],
      ),
    );
  }

  Widget _buildHeader(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 0, 8, 12),
      child: Row(
        children: [
          Expanded(
            child: Text(
              'Pilih Penerima',
              style: Theme.of(context).textTheme.titleLarge,
            ),
          ),
          IconButton(
            tooltip: 'Tutup',
            onPressed: () => Navigator.of(context).pop(),
            icon: const Icon(Icons.close),
          ),
        ],
      ),
    );
  }

  Widget _buildOption(BuildContext context, _RecipientOption option) {
    final duplicate = widget.state.form.recipients.any(
      (recipient) =>
          recipient.type == _recipientType &&
          recipient.targetType == option.targetType &&
          recipient.targetId == option.id,
    );
    final statusLabel = duplicate ? 'Sudah ditambahkan' : 'Pilih penerima';
    final semanticLabel = [
      option.label,
      if (option.subtitle.isNotEmpty) option.subtitle,
      statusLabel,
    ].join('. ');
    final selectRecipient = duplicate
        ? null
        : () {
            Navigator.of(context).pop(
              DraftRecipient(
                type: _recipientType,
                targetType: option.targetType,
                targetId: option.id,
                label: option.recipientLabel,
              ),
            );
          };

    return Semantics(
      key: ValueKey('${option.targetType.wireValue}-${option.id}'),
      button: true,
      enabled: !duplicate,
      label: semanticLabel,
      onTap: selectRecipient,
      child: ExcludeSemantics(
        child: ListTile(
          enabled: !duplicate,
          minTileHeight: 64,
          minVerticalPadding: 10,
          contentPadding: const EdgeInsets.symmetric(horizontal: 12),
          leading: CircleAvatar(
            child: Icon(
              option.targetType == DraftRecipientTargetType.position
                  ? Icons.badge_outlined
                  : Icons.account_tree_outlined,
            ),
          ),
          title: Text(
            option.label,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
          ),
          subtitle: option.subtitle.isEmpty
              ? null
              : Text(
                  option.subtitle,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
          trailing: duplicate
              ? Tooltip(
                  message: 'Sudah ditambahkan',
                  child: Icon(
                    Icons.check_circle_outline,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                )
              : const Icon(Icons.chevron_right),
          onTap: selectRecipient,
        ),
      ),
    );
  }

  List<_RecipientOption> get _filteredOptions {
    final normalizedQuery = _query.trim().toLowerCase();
    final options = _targetType == DraftRecipientTargetType.position
        ? widget.state.availableRecipientPositions.map(
            (position) {
              final details = [
                position.orgUnitName,
                if (position.holderName.isNotEmpty)
                  'Pemegang: ${position.holderName}',
              ];
              return _RecipientOption(
                id: position.id,
                targetType: DraftRecipientTargetType.position,
                label: position.title,
                recipientLabel: position.label,
                subtitle: details.join(' | '),
                searchableText: [
                  position.title,
                  position.orgUnitName,
                  position.holderName,
                  position.positionType,
                ].join(' '),
              );
            },
          )
        : widget.state.availableRecipientOrgUnits.map(
            (unit) {
              final details = [
                unit.code,
                _humanizeIdentifier(unit.unitLevel),
                if (unit.region?.trim().isNotEmpty ?? false) unit.region!,
              ];
              return _RecipientOption(
                id: unit.id,
                targetType: DraftRecipientTargetType.orgUnit,
                label: unit.name,
                recipientLabel: unit.label,
                subtitle: details.join(' | '),
                searchableText: [
                  unit.name,
                  unit.code,
                  unit.unitLevel,
                  unit.region ?? '',
                ].join(' '),
              );
            },
          );

    if (normalizedQuery.isEmpty) return options.toList(growable: false);
    return options
        .where(
          (option) =>
              option.searchableText.toLowerCase().contains(normalizedQuery),
        )
        .toList(growable: false);
  }
}

class _RecipientOption {
  const _RecipientOption({
    required this.id,
    required this.targetType,
    required this.label,
    required this.recipientLabel,
    required this.subtitle,
    required this.searchableText,
  });

  final String id;
  final DraftRecipientTargetType targetType;
  final String label;
  final String recipientLabel;
  final String subtitle;
  final String searchableText;
}

class _RecipientEmptyState extends StatelessWidget {
  const _RecipientEmptyState({
    required this.hasQuery,
    required this.targetType,
    required this.query,
  });

  final bool hasQuery;
  final DraftRecipientTargetType targetType;
  final String query;

  @override
  Widget build(BuildContext context) {
    final targetLabel =
        targetType == DraftRecipientTargetType.position ? 'jabatan' : 'unit';
    final title = hasQuery
        ? 'Penerima tidak ditemukan'
        : 'Tidak ada $targetLabel tersedia';
    final message = hasQuery
        ? 'Tidak ada hasil yang cocok dengan "$query".'
        : 'Daftar $targetLabel tujuan masih kosong.';

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              hasQuery ? Icons.search_off : Icons.inbox_outlined,
              size: 48,
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
            const SizedBox(height: 16),
            Text(
              title,
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 8),
            Text(
              message,
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
            ),
          ],
        ),
      ),
    );
  }
}

String _humanizeIdentifier(String value) {
  if (value.isEmpty) return '';
  return value
      .split('_')
      .where((part) => part.isNotEmpty)
      .map(
        (part) => '${part[0].toUpperCase()}${part.substring(1).toLowerCase()}',
      )
      .join(' ');
}
