import 'package:eoffice_mobile/core/utils/date_format.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_composer_state.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:flutter/material.dart';

class DraftListPanel extends StatelessWidget {
  const DraftListPanel({
    required this.state,
    required this.onNewDraft,
    required this.onOpenDraft,
    this.compact = false,
    super.key,
  });

  final DraftComposerState state;
  final VoidCallback onNewDraft;
  final ValueChanged<DraftLetter> onOpenDraft;
  final bool compact;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _PanelHeader(
          count: state.drafts.length,
          compact: compact,
          onNewDraft: state.busy ? null : onNewDraft,
        ),
        const Divider(height: 1),
        Expanded(
          child: state.drafts.isEmpty
              ? _EmptyDrafts(onNewDraft: state.busy ? null : onNewDraft)
              : ListView.separated(
                  padding: EdgeInsets.all(compact ? 8 : 12),
                  itemCount: state.drafts.length,
                  separatorBuilder: (_, __) =>
                      SizedBox(height: compact ? 6 : 8),
                  itemBuilder: (context, index) {
                    final draft = state.drafts[index];
                    return _DraftListItem(
                      key: ValueKey(draft.id),
                      draft: draft,
                      compact: compact,
                      selected: state.form.draftId == draft.id,
                      onTap: () => onOpenDraft(draft),
                    );
                  },
                ),
        ),
      ],
    );
  }
}

class _PanelHeader extends StatelessWidget {
  const _PanelHeader({
    required this.count,
    required this.compact,
    required this.onNewDraft,
  });

  final int count;
  final bool compact;
  final VoidCallback? onNewDraft;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(
        horizontal: compact ? 12 : 16,
        vertical: 12,
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Draft Saya',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 2),
                Text(
                  '$count draft aktif',
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                ),
              ],
            ),
          ),
          if (compact)
            IconButton.filledTonal(
              onPressed: onNewDraft,
              tooltip: 'Draft baru',
              icon: const Icon(Icons.add),
            )
          else
            FilledButton.tonalIcon(
              onPressed: onNewDraft,
              icon: const Icon(Icons.add),
              label: const Text('Baru'),
            ),
        ],
      ),
    );
  }
}

class _DraftListItem extends StatelessWidget {
  const _DraftListItem({
    required this.draft,
    required this.selected,
    required this.compact,
    required this.onTap,
    super.key,
  });

  final DraftLetter draft;
  final bool selected;
  final bool compact;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final subject =
        draft.subject.trim().isEmpty ? 'Tanpa perihal' : draft.subject.trim();
    final typeLabel = draft.letterTypeCode.isEmpty
        ? draft.letterTypeName
        : draft.letterTypeCode;
    final updatedLabel = compact
        ? formatRelativeTime(draft.updatedAt)
        : formatDateTime(draft.updatedAt);

    return Semantics(
      button: true,
      selected: selected,
      excludeSemantics: true,
      onTap: onTap,
      label: '$subject, $typeLabel, versi ${draft.version}, '
          'diperbarui $updatedLabel',
      child: Material(
        color: selected ? colors.secondaryContainer : colors.surface,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(8),
          side: BorderSide(
            color: selected ? colors.primary : colors.outlineVariant,
          ),
        ),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: onTap,
          child: Padding(
            padding: EdgeInsets.all(compact ? 10 : 12),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(
                  selected ? Icons.description : Icons.description_outlined,
                  color: selected
                      ? colors.onSecondaryContainer
                      : colors.onSurfaceVariant,
                ),
                SizedBox(width: compact ? 8 : 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        subject,
                        maxLines: compact ? 1 : 2,
                        overflow: TextOverflow.ellipsis,
                        style: Theme.of(context).textTheme.titleSmall?.copyWith(
                              color: selected
                                  ? colors.onSecondaryContainer
                                  : colors.onSurface,
                            ),
                      ),
                      const SizedBox(height: 6),
                      Row(
                        children: [
                          Expanded(
                            child: Text(
                              '$typeLabel - v${draft.version}',
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: Theme.of(context)
                                  .textTheme
                                  .bodySmall
                                  ?.copyWith(
                                    color: selected
                                        ? colors.onSecondaryContainer
                                        : colors.onSurfaceVariant,
                                  ),
                            ),
                          ),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Tooltip(
                              message: formatDateTime(draft.updatedAt),
                              child: Text(
                                updatedLabel,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                textAlign: TextAlign.end,
                                style: Theme.of(context)
                                    .textTheme
                                    .bodySmall
                                    ?.copyWith(
                                      color: selected
                                          ? colors.onSecondaryContainer
                                          : colors.onSurfaceVariant,
                                    ),
                              ),
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _EmptyDrafts extends StatelessWidget {
  const _EmptyDrafts({required this.onNewDraft});

  final VoidCallback? onNewDraft;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.note_add_outlined,
              size: 40,
              color: colors.onSurfaceVariant,
            ),
            const SizedBox(height: 12),
            Text(
              'Belum ada draft',
              style: Theme.of(context).textTheme.titleSmall,
            ),
            const SizedBox(height: 4),
            Text(
              'Mulai surat baru untuk menyimpan draft pertama.',
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: colors.onSurfaceVariant,
                  ),
            ),
            const SizedBox(height: 16),
            OutlinedButton.icon(
              onPressed: onNewDraft,
              icon: const Icon(Icons.add),
              label: const Text('Draft baru'),
            ),
          ],
        ),
      ),
    );
  }
}
