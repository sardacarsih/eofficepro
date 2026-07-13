import 'package:eoffice_mobile/features/letters/domain/draft_composer_state.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_labels.dart';
import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';
import 'package:eoffice_mobile/features/letters/presentation/compose_controller.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/letter_body_editor.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/on_behalf_notice.dart';
import 'package:eoffice_mobile/features/letters/presentation/widgets/recipient_picker_sheet.dart';
import 'package:flutter/material.dart';

class ComposeFormFields extends StatelessWidget {
  const ComposeFormFields({
    required this.state,
    required this.controller,
    required this.subjectController,
    this.useNativeBodyEditor,
    super.key,
  });

  final DraftComposerState state;
  final DraftComposerController controller;
  final TextEditingController subjectController;

  /// Overrides platform detection for the body editor; used by tests.
  final bool? useNativeBodyEditor;

  @override
  Widget build(BuildContext context) {
    return Form(
      autovalidateMode: AutovalidateMode.onUserInteraction,
      child: LayoutBuilder(
        builder: (context, constraints) {
          final twoColumns = constraints.maxWidth >= 720;
          final fieldWidth = twoColumns
              ? (constraints.maxWidth - 16) / 2
              : constraints.maxWidth;
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Wrap(
                spacing: 16,
                runSpacing: 16,
                children: [
                  SizedBox(
                    width: fieldWidth,
                    child: _companyField(),
                  ),
                  SizedBox(
                    width: fieldWidth,
                    child: _letterTypeField(),
                  ),
                  SizedBox(
                    width: fieldWidth,
                    child: _creatorPositionField(),
                  ),
                  if (_selectedLetterTypeCode == 'PRS') ...[
                    SizedBox(
                        width: fieldWidth, child: _approvalCategoryField()),
                    SizedBox(width: fieldWidth, child: _finalLevelField()),
                  ],
                  SizedBox(
                    width: fieldWidth,
                    child: _templateField(),
                  ),
                  SizedBox(
                    width: fieldWidth,
                    child: _classificationField(),
                  ),
                  SizedBox(
                    width: fieldWidth,
                    child: _priorityField(context),
                  ),
                ],
              ),
              if (state.selectedCreatorPosition?.positionType ==
                  'secretary') ...[
                const SizedBox(height: 16),
                OnBehalfNotice(
                  title: state.onBehalfPosition?.title ??
                      state.selectedCreatorPosition?.reportsToTitle ??
                      'Atasan langsung belum tersedia',
                ),
              ],
              const SizedBox(height: 16),
              OutlinedButton.icon(
                onPressed:
                    state.formLocked ? null : controller.previewApprovalRoute,
                icon: const Icon(Icons.route_outlined),
                label: const Text('Tampilkan rute approval'),
              ),
              if (state.approvalRoute case final route?) ...[
                const SizedBox(height: 8),
                Semantics(
                  label: 'Rute approval sampai ${route.finalLevel}',
                  child: Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Text([
                        if (route.coordinationScope != null)
                          'Cakupan: ${route.coordinationScope}',
                        'Level akhir: ${route.finalLevel}',
                        route.steps.join(' → '),
                      ].join('\n')),
                    ),
                  ),
                ),
              ],
              const SizedBox(height: 24),
              _RecipientsField(
                state: state,
                onAdd: () => _addRecipient(context),
                onRemove: controller.removeRecipient,
              ),
              const SizedBox(height: 24),
              TextFormField(
                controller: subjectController,
                enabled: !state.formLocked,
                maxLength: 255,
                textCapitalization: TextCapitalization.sentences,
                textInputAction: TextInputAction.next,
                decoration: const InputDecoration(
                  labelText: 'Perihal',
                  hintText: 'Ringkas dan spesifik',
                  prefixIcon: Icon(Icons.subject_outlined),
                ),
                validator: (value) {
                  if (value == null || value.trim().isEmpty) {
                    return 'Perihal wajib diisi';
                  }
                  return null;
                },
                onChanged: (value) {
                  controller.editForm(
                    (form) => form.copyWith(subject: value),
                  );
                },
              ),
              const SizedBox(height: 16),
              LetterBodyEditor(
                value: state.form.bodyHtml,
                plainValue: state.form.bodyPlain,
                enabled: !state.formLocked,
                readOnly: state.form.bodyReadOnly,
                useNativeEditor: useNativeBodyEditor,
                onChanged: (html, plain) {
                  controller.editForm(
                    (form) => form.copyWith(bodyHtml: html, bodyPlain: plain),
                  );
                },
                onSimplifyRequested: () => _confirmSimplifyBody(context),
              ),
            ],
          );
        },
      ),
    );
  }

  Widget _companyField() {
    return DropdownButtonFormField<String>(
      key: ValueKey('company-${state.form.companyId}'),
      initialValue: state.activeCompanyId,
      isExpanded: true,
      decoration: const InputDecoration(
        labelText: 'Perusahaan',
        prefixIcon: Icon(Icons.business_outlined),
      ),
      items: state.availableCompanies
          .map(
            (company) => DropdownMenuItem(
              value: company.id,
              child: Text(
                '${company.code} - ${company.name}',
                overflow: TextOverflow.ellipsis,
              ),
            ),
          )
          .toList(growable: false),
      onChanged: state.formLocked
          ? null
          : (value) {
              if (value != null) controller.selectCompany(value);
            },
    );
  }

  Widget _letterTypeField() {
    return DropdownButtonFormField<String>(
      key: ValueKey('letter-type-${state.form.letterTypeId}'),
      initialValue: state.activeLetterTypeId,
      isExpanded: true,
      decoration: const InputDecoration(
        labelText: 'Jenis Surat',
        prefixIcon: Icon(Icons.description_outlined),
      ),
      items: state.bootstrap.letterTypes
          .map(
            (type) => DropdownMenuItem(
              value: type.id,
              child: Text(
                '${type.code} - ${type.name}',
                overflow: TextOverflow.ellipsis,
              ),
            ),
          )
          .toList(growable: false),
      onChanged: state.formLocked
          ? null
          : (value) {
              if (value != null) controller.selectLetterType(value);
            },
    );
  }

  Widget _creatorPositionField() {
    return DropdownButtonFormField<String>(
      key: ValueKey('creator-${state.form.creatorPositionId}'),
      initialValue: state.activeCreatorPositionId,
      isExpanded: true,
      decoration: const InputDecoration(
        labelText: 'Jabatan Pembuat',
        prefixIcon: Icon(Icons.badge_outlined),
      ),
      items: state.creatorPositions
          .map(
            (position) => DropdownMenuItem(
              value: position.id,
              child: Text(
                '${position.title} - ${position.orgUnitName}',
                overflow: TextOverflow.ellipsis,
              ),
            ),
          )
          .toList(growable: false),
      onChanged: state.formLocked
          ? null
          : (value) {
              if (value != null) controller.selectCreatorPosition(value);
            },
    );
  }

  String get _selectedLetterTypeCode {
    for (final type in state.bootstrap.letterTypes) {
      if (type.id == state.form.letterTypeId) return type.code;
    }
    return '';
  }

  Widget _approvalCategoryField() => DropdownButtonFormField<String>(
        key: ValueKey('approval-category-${state.form.approvalCategoryId}'),
        initialValue: state.form.approvalCategoryId.isEmpty
            ? null
            : state.form.approvalCategoryId,
        decoration: const InputDecoration(
            labelText: 'Kategori Persetujuan',
            prefixIcon: Icon(Icons.category_outlined)),
        items: state.bootstrap.approvalCategories
            .map((item) =>
                DropdownMenuItem(value: item.id, child: Text(item.name)))
            .toList(),
        onChanged: state.formLocked
            ? null
            : (value) {
                if (value != null) controller.selectApprovalCategory(value);
              },
      );

  Widget _finalLevelField() {
    final levels = state.approvalRoute?.allowedLevels ?? const <String>[];
    return DropdownButtonFormField<String>(
      key: ValueKey('final-level-${state.form.requestedFinalLevel}'),
      initialValue: state.form.requestedFinalLevel.isEmpty
          ? null
          : state.form.requestedFinalLevel,
      decoration: const InputDecoration(
          labelText: 'Level Akhir',
          prefixIcon: Icon(Icons.account_tree_outlined)),
      items: levels
          .map((level) => DropdownMenuItem(
              value: level, child: Text(level.replaceAll('_', ' '))))
          .toList(),
      onChanged: state.formLocked || levels.isEmpty
          ? null
          : (value) {
              if (value != null) controller.selectRequestedFinalLevel(value);
            },
    );
  }

  Widget _templateField() {
    return DropdownButtonFormField<String>(
      key: ValueKey('template-${state.form.selectedTemplateId}'),
      initialValue: state.form.selectedTemplateId,
      isExpanded: true,
      decoration: const InputDecoration(
        labelText: 'Template',
        prefixIcon: Icon(Icons.article_outlined),
      ),
      items: [
        const DropdownMenuItem(
          value: '',
          child: Text('Tanpa template'),
        ),
        ...state.matchingTemplates.map(
          (template) => DropdownMenuItem(
            value: template.id,
            child: Text(
              '${template.letterTypeCode} v${template.version} - '
              '${template.companyCode}',
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ),
      ],
      onChanged: state.formLocked
          ? null
          : (value) {
              if (value != null) controller.applyTemplate(value);
            },
    );
  }

  Widget _classificationField() {
    return DropdownButtonFormField<LetterClassification>(
      key: ValueKey('classification-${state.form.classification.name}'),
      initialValue: state.form.classification,
      isExpanded: true,
      decoration: const InputDecoration(
        labelText: 'Klasifikasi',
        prefixIcon: Icon(Icons.shield_outlined),
      ),
      items: LetterClassification.values
          .map(
            (value) => DropdownMenuItem(
              value: value,
              child: Text(value.displayLabel),
            ),
          )
          .toList(growable: false),
      onChanged: state.formLocked
          ? null
          : (value) {
              if (value != null) {
                controller.editForm(
                  (form) => form.copyWith(classification: value),
                );
              }
            },
    );
  }

  Widget _priorityField(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final compact = constraints.maxWidth < 420;
        return InputDecorator(
          decoration: InputDecoration(
            labelText: 'Prioritas',
            prefixIcon: compact ? null : const Icon(Icons.flag_outlined),
          ),
          child: SegmentedButton<LetterPriority>(
            expandedInsets: EdgeInsets.zero,
            showSelectedIcon: false,
            segments: [
              ButtonSegment(
                value: LetterPriority.normal,
                icon: const Icon(Icons.low_priority),
                label: compact ? null : const Text('Normal'),
                tooltip: 'Prioritas normal',
              ),
              ButtonSegment(
                value: LetterPriority.urgent,
                icon: const Icon(Icons.priority_high),
                label: compact ? null : const Text('Urgent'),
                tooltip: 'Prioritas urgent',
              ),
            ],
            selected: {state.form.priority},
            onSelectionChanged: state.formLocked
                ? null
                : (selection) {
                    controller.editForm(
                      (form) => form.copyWith(priority: selection.first),
                    );
                  },
          ),
        );
      },
    );
  }

  Future<void> _confirmSimplifyBody(BuildContext context) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Edit sebagai teks biasa?'),
        content: const Text(
          'Format lanjutan pada isi surat akan dihapus saat draft disimpan.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Lanjutkan'),
          ),
        ],
      ),
    );
    if (confirmed == true) controller.simplifyBodyFormatting();
  }

  Future<void> _addRecipient(BuildContext context) async {
    final recipient = await showRecipientPickerSheet(context, state);
    if (recipient != null) controller.addRecipient(recipient);
  }
}

class _RecipientsField extends StatelessWidget {
  const _RecipientsField({
    required this.state,
    required this.onAdd,
    required this.onRemove,
  });

  final DraftComposerState state;
  final VoidCallback onAdd;
  final ValueChanged<DraftRecipient> onRemove;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final toRecipients = state.form.recipients
        .where((item) => item.type == DraftRecipientType.to)
        .toList(growable: false);
    final ccRecipients = state.form.recipients
        .where((item) => item.type == DraftRecipientType.cc)
        .toList(growable: false);

    return DecoratedBox(
      decoration: BoxDecoration(
        border: Border.all(color: scheme.outlineVariant),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                const Icon(Icons.group_outlined),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Penerima',
                    style: Theme.of(context).textTheme.titleSmall,
                  ),
                ),
                FilledButton.tonalIcon(
                  onPressed: state.formLocked ? null : onAdd,
                  icon: const Icon(Icons.person_add_alt_1),
                  label: const Text('Tambah'),
                ),
              ],
            ),
            const SizedBox(height: 16),
            _RecipientGroup(
              label: 'To',
              recipients: toRecipients,
              onRemove: state.formLocked ? null : onRemove,
            ),
            const SizedBox(height: 12),
            _RecipientGroup(
              label: 'CC',
              recipients: ccRecipients,
              onRemove: state.formLocked ? null : onRemove,
            ),
          ],
        ),
      ),
    );
  }
}

class _RecipientGroup extends StatelessWidget {
  const _RecipientGroup({
    required this.label,
    required this.recipients,
    required this.onRemove,
  });

  final String label;
  final List<DraftRecipient> recipients;
  final ValueChanged<DraftRecipient>? onRemove;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: Theme.of(context).textTheme.labelMedium),
        const SizedBox(height: 6),
        if (recipients.isEmpty)
          Text(
            label == 'To' ? 'Belum ada penerima tujuan' : 'Tidak ada tembusan',
            style: Theme.of(context).textTheme.bodySmall,
          )
        else
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: recipients
                .map(
                  (recipient) => InputChip(
                    avatar: Icon(
                      recipient.targetType == DraftRecipientTargetType.position
                          ? Icons.badge_outlined
                          : Icons.account_tree_outlined,
                      size: 18,
                    ),
                    label: ConstrainedBox(
                      constraints: const BoxConstraints(maxWidth: 260),
                      child: Text(
                        recipient.label,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    onDeleted:
                        onRemove == null ? null : () => onRemove!(recipient),
                    deleteButtonTooltipMessage: 'Hapus ${recipient.label}',
                  ),
                )
                .toList(growable: false),
          ),
      ],
    );
  }
}
