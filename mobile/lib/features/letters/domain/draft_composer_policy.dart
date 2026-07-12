part of 'draft_composer_state.dart';

String? defaultOnBehalfPositionId(
  String creatorPositionId,
  List<DraftPosition> positions,
) {
  for (final position in positions) {
    if (position.id == creatorPositionId &&
        position.positionType == 'secretary') {
      return position.reportsTo;
    }
  }
  return null;
}

bool isManagerOrAbovePositionType(String positionType) {
  return const {
    'division_head',
    'sub_dept_head',
    'dept_head',
    'gm',
    'director',
    'vp_director',
    'president_director',
  }.contains(positionType);
}

String? directorateIdForOrgUnit(
  String orgUnitId,
  List<DraftOrgUnit> units,
) {
  final byId = {for (final unit in units) unit.id: unit};
  var current = byId[orgUnitId];
  final visited = <String>{};
  while (current != null && visited.add(current.id)) {
    if (current.unitLevel == 'directorate') return current.id;
    current = current.parentId == null ? null : byId[current.parentId];
  }
  return null;
}

String? recipientPolicyMessage(
  DraftComposerForm form,
  DraftComposerBootstrap bootstrap,
) {
  DraftPosition? creator;
  for (final position in bootstrap.positions) {
    if (position.id == form.creatorPositionId) {
      creator = position;
      break;
    }
  }
  if (creator == null) return null;
  final creatorDirectorate = directorateIdForOrgUnit(
    creator.orgUnitId,
    bootstrap.orgUnits,
  );
  if (creatorDirectorate == null) return null;

  for (final recipient in form.recipients) {
    String? targetDirectorate;
    if (recipient.targetType == DraftRecipientTargetType.position) {
      DraftPosition? target;
      for (final position in bootstrap.positions) {
        if (position.id == recipient.targetId) {
          target = position;
          break;
        }
      }
      if (target != null) {
        targetDirectorate = directorateIdForOrgUnit(
          target.orgUnitId,
          bootstrap.orgUnits,
        );
      }
    } else {
      targetDirectorate = directorateIdForOrgUnit(
        recipient.targetId,
        bootstrap.orgUnits,
      );
    }

    if (targetDirectorate == null || targetDirectorate == creatorDirectorate) {
      continue;
    }
    if (!isManagerOrAbovePositionType(creator.positionType)) {
      return 'Surat lintas direktorat hanya dapat dibuat oleh level '
          'Sub Department Head ke atas.';
    }
    if (recipient.targetType == DraftRecipientTargetType.orgUnit) {
      return 'Penerima unit lintas direktorat tidak diizinkan. '
          'Pilih jabatan tujuan.';
    }
  }
  return null;
}

String? onBehalfPolicyMessage(
  DraftComposerForm form,
  DraftComposerBootstrap bootstrap,
) {
  DraftPosition? creator;
  DraftPosition? onBehalf;
  for (final position in bootstrap.positions) {
    if (position.id == form.creatorPositionId) creator = position;
    if (position.id == form.onBehalfOfPositionId) onBehalf = position;
  }
  if (creator == null) return null;
  if (creator.positionType != 'secretary') {
    return form.onBehalfOfPositionId == null
        ? null
        : 'Atas nama hanya dapat digunakan oleh jabatan Secretary.';
  }
  if (form.onBehalfOfPositionId == null ||
      creator.reportsTo != form.onBehalfOfPositionId ||
      onBehalf == null) {
    return 'Jabatan atas nama harus atasan langsung Secretary.';
  }
  if (onBehalf.positionType != 'director' && onBehalf.positionType != 'gm') {
    return 'Jabatan atas nama harus Director atau GM.';
  }
  return null;
}
