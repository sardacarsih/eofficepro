BEGIN;

CREATE TABLE approval_categories (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code        varchar(30) NOT NULL UNIQUE,
    name        varchar(100) NOT NULL,
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

INSERT INTO approval_categories (code, name) VALUES
    ('OPERASIONAL', 'Operasional'),
    ('SDM', 'Sumber Daya Manusia'),
    ('KEUANGAN', 'Keuangan'),
    ('LEGAL', 'Legal');

ALTER TABLE approval_matrices
    ADD COLUMN approval_category_id uuid REFERENCES approval_categories(id),
    ADD COLUMN resolution_mode varchar(20) NOT NULL DEFAULT 'fixed'
        CHECK (resolution_mode IN ('fixed', 'user_selected', 'scope_derived')),
    ADD COLUMN min_final_level varchar(30),
    ADD COLUMN max_final_level varchar(30);

DROP INDEX IF EXISTS idx_approval_matrices_active_default;
DROP INDEX IF EXISTS idx_approval_matrices_active_specific;
CREATE UNIQUE INDEX idx_approval_matrices_active_policy
    ON approval_matrices (
        letter_type_id,
        COALESCE(originator_level, ''),
        COALESCE(approval_category_id, '00000000-0000-0000-0000-000000000000'::uuid)
    ) WHERE is_active;

CREATE TABLE coordination_scope_rules (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope       varchar(30) NOT NULL UNIQUE CHECK (scope IN
                ('same_unit','cross_department','cross_biro','cross_directorate','corporate')),
    final_level varchar(30) NOT NULL,
    is_active   boolean NOT NULL DEFAULT true,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

INSERT INTO coordination_scope_rules (scope, final_level) VALUES
    ('same_unit', 'dept_head'),
    ('cross_department', 'gm'),
    ('cross_biro', 'director'),
    ('cross_directorate', 'vp_director'),
    ('corporate', 'president_director');

ALTER TABLE letters
    ADD COLUMN approval_category_id uuid REFERENCES approval_categories(id),
    ADD COLUMN requested_final_level varchar(30),
    ADD COLUMN resolved_final_level varchar(30),
    ADD COLUMN coordination_scope varchar(30),
    ADD COLUMN approval_resolution_mode varchar(20);

INSERT INTO letter_types (code, name, default_classification, default_sla_hours, electronic_submission_enabled)
VALUES
    ('PRS', 'Persetujuan', 'biasa', 24, true),
    ('KOR', 'Koordinasi', 'biasa', 24, true)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    electronic_submission_enabled = true,
    is_active = true;

UPDATE letter_types SET is_active = code IN ('PRS', 'KOR');

INSERT INTO approval_matrices
    (letter_type_id, approval_category_id, originator_level, final_level,
     resolution_mode, min_final_level, max_final_level, flow_mode, is_active)
SELECT lt.id, category.id, originator.position_type, 'director',
       'user_selected', originator.min_level, originator.max_level, 'serial', true
FROM letter_types lt
CROSS JOIN approval_categories category
CROSS JOIN (VALUES
    ('staff', 'dept_head', 'director'),
    ('assistant', 'dept_head', 'director'),
    ('division_head', 'dept_head', 'director'),
    ('sub_dept_head', 'dept_head', 'director'),
    ('dept_head', 'gm', 'vp_director'),
    ('gm', 'director', 'vp_director'),
    ('director', 'vp_director', 'president_director'),
    ('vp_director', 'president_director', 'president_director'),
    ('secretary', 'dept_head', 'director')
) AS originator(position_type, min_level, max_level)
WHERE lt.code = 'PRS';

INSERT INTO approval_matrices
    (letter_type_id, originator_level, final_level, resolution_mode, flow_mode, is_active)
SELECT id, NULL, 'director', 'scope_derived', 'serial', true
FROM letter_types WHERE code = 'KOR';

COMMIT;
