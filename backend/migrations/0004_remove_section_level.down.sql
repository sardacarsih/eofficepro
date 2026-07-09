BEGIN;

ALTER TABLE positions
    DROP CONSTRAINT IF EXISTS positions_position_type_check;

ALTER TABLE positions
    ADD CONSTRAINT positions_position_type_check
    CHECK (position_type IN
           ('president_director','vp_director','director','gm','dept_head',
            'section_head','division_head','assistant','secretary','staff','auditor'));

ALTER TABLE org_units
    DROP CONSTRAINT IF EXISTS org_units_unit_level_check;

ALTER TABLE org_units
    ADD CONSTRAINT org_units_unit_level_check
    CHECK (unit_level IN
           ('directorate','biro','department','section','division','office'));

COMMIT;
