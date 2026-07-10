BEGIN;

INSERT INTO roles (code, name, permissions)
VALUES ('approver', 'Penyetuju (Legacy)', '[]')
ON CONFLICT (code) DO NOTHING;

COMMIT;
