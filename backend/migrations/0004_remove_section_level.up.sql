BEGIN;

-- Division yang sebelumnya berada di bawah Section dipindahkan langsung ke
-- Department induk Section tersebut.
UPDATE org_units div
SET parent_id = section.parent_id
FROM org_units section
WHERE div.parent_id = section.id
  AND div.unit_level = 'division'
  AND section.unit_level = 'section';

-- Penempatan aktif pada Section Head ditutup agar tidak menjadi jabatan aktif
-- untuk struktur baru. Baris historis tetap dipertahankan.
UPDATE user_positions up
SET valid_to = current_date
FROM positions p
WHERE up.position_id = p.id
  AND p.position_type = 'section_head'
  AND current_date >= up.valid_from
  AND (up.valid_to IS NULL OR current_date < up.valid_to);

UPDATE positions
SET is_active = false
WHERE position_type = 'section_head'
  AND is_active;

UPDATE org_units
SET is_active = false,
    valid_to = current_date
WHERE unit_level = 'section'
  AND is_active;

ALTER TABLE org_units
    DROP CONSTRAINT IF EXISTS org_units_unit_level_check;

ALTER TABLE org_units
    ADD CONSTRAINT org_units_unit_level_check
    CHECK (
        unit_level IN ('directorate','biro','department','division','office')
        OR (unit_level = 'section' AND NOT is_active)
    );

ALTER TABLE positions
    DROP CONSTRAINT IF EXISTS positions_position_type_check;

ALTER TABLE positions
    ADD CONSTRAINT positions_position_type_check
    CHECK (
        position_type IN ('president_director','vp_director','director','gm','dept_head',
                          'division_head','assistant','secretary','staff','auditor')
        OR (position_type = 'section_head' AND NOT is_active)
    );

COMMIT;
