BEGIN;

INSERT INTO roles (code, name, permissions)
VALUES (
    'management_viewer',
    'Pengakses Dashboard Manajemen',
    '["management.effectiveness.read"]'
)
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name;

UPDATE roles
SET permissions = '["audit.letter.read","audit.trail.read","audit.export"]'
WHERE code = 'auditor'
  AND permissions = '[]'::jsonb;

CREATE TABLE audit_assignments (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            uuid NOT NULL REFERENCES users(id),
    org_unit_id        uuid NOT NULL REFERENCES org_units(id),
    max_classification varchar(10) NOT NULL DEFAULT 'terbatas'
                         CHECK (max_classification IN ('biasa','terbatas','rahasia')),
    can_export         boolean NOT NULL DEFAULT false,
    valid_from         date NOT NULL DEFAULT current_date,
    valid_to           date,
    created_at         timestamptz NOT NULL DEFAULT now(),
    CHECK (valid_to IS NULL OR valid_to > valid_from)
);

CREATE INDEX idx_audit_assignments_active
    ON audit_assignments (user_id, org_unit_id)
    WHERE valid_to IS NULL;

COMMIT;
