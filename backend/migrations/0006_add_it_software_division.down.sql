BEGIN;

UPDATE user_positions assignment
SET valid_to = current_date
FROM positions position
JOIN org_units division ON division.id = position.org_unit_id
WHERE assignment.position_id = position.id
  AND division.code = 'DIV-IT-SW'
  AND position.position_type = 'division_head'
  AND current_date >= assignment.valid_from
  AND (assignment.valid_to IS NULL OR current_date < assignment.valid_to);

UPDATE positions position
SET is_active = false
FROM org_units division
WHERE position.org_unit_id = division.id
  AND division.code = 'DIV-IT-SW'
  AND position.position_type = 'division_head'
  AND position.is_active;

UPDATE org_units
SET is_active = false,
    valid_to = current_date
WHERE code = 'DIV-IT-SW'
  AND unit_level = 'division'
  AND is_active;

COMMIT;
