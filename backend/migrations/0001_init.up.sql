-- eOffice Pro — skema inti v1
-- Referensi desain: docs/DATABASE-SCHEMA.md
-- Prinsip: jabatan != orang, snapshot rute per surat, audit append-only, tanpa hard delete.

BEGIN;

-- ============ 1. Organisasi & Pengguna ============

CREATE TABLE companies (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code              varchar(10) NOT NULL UNIQUE,
    name              varchar(150) NOT NULL,
    letterhead_config jsonb NOT NULL DEFAULT '{}',
    is_active         boolean NOT NULL DEFAULT true,
    created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE org_units (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id  uuid NOT NULL REFERENCES companies(id),
    parent_id   uuid REFERENCES org_units(id),
    code        varchar(20) NOT NULL,
    name        varchar(150) NOT NULL,
    unit_level  varchar(20) NOT NULL CHECK (unit_level IN
                  ('directorate','biro','department','division','office')),
    region      varchar(10) CHECK (region IN ('HO','REG1','REG2','REPO_JKT','REPO_PKB')),
    path        varchar(500) NOT NULL DEFAULT '',  -- materialized path utk query hierarki
    valid_from  date NOT NULL DEFAULT current_date,
    valid_to    date,
    is_active   boolean NOT NULL DEFAULT true,
    UNIQUE (company_id, code)
);
CREATE INDEX idx_org_units_parent ON org_units(parent_id);
CREATE INDEX idx_org_units_path ON org_units(path);

CREATE TABLE positions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_unit_id   uuid NOT NULL REFERENCES org_units(id),
    title         varchar(150) NOT NULL,
    position_type varchar(30) NOT NULL CHECK (position_type IN
                    ('president_director','vp_director','director','gm','dept_head',
                     'sub_dept_head','division_head','assistant','secretary','staff','auditor')),
    reports_to    uuid REFERENCES positions(id),  -- rantai atasan = dasar rute approval
    is_approver   boolean NOT NULL DEFAULT false,
    is_active     boolean NOT NULL DEFAULT true
);
CREATE INDEX idx_positions_reports_to ON positions(reports_to);
CREATE INDEX idx_positions_org_unit ON positions(org_unit_id);

CREATE TABLE users (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nik                 varchar(30) NOT NULL UNIQUE,
    email               varchar(150) NOT NULL UNIQUE,
    full_name           varchar(150) NOT NULL,
    password_hash       varchar(255) NOT NULL,
    phone               varchar(30),
    signature_image_key varchar(255),
    status              varchar(10) NOT NULL DEFAULT 'active'
                          CHECK (status IN ('active','inactive','locked')),
    failed_login_count  int NOT NULL DEFAULT 0,
    locked_until        timestamptz,
    last_login_at       timestamptz,
    external_ref        varchar(50),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_positions (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users(id),
    position_id     uuid NOT NULL REFERENCES positions(id),
    assignment_type varchar(10) NOT NULL DEFAULT 'definitive'
                      CHECK (assignment_type IN ('definitive','plt','plh')),
    valid_from      date NOT NULL DEFAULT current_date,
    valid_to        date,
    UNIQUE (user_id, position_id, valid_from)
);
CREATE INDEX idx_user_positions_user ON user_positions(user_id);
CREATE INDEX idx_user_positions_position ON user_positions(position_id);

CREATE TABLE roles (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code        varchar(20) NOT NULL UNIQUE,  -- admin, creator, approver, secretary, auditor
    name        varchar(100) NOT NULL,
    permissions jsonb NOT NULL DEFAULT '[]'
);

CREATE TABLE user_roles (
    user_id uuid NOT NULL REFERENCES users(id),
    role_id uuid NOT NULL REFERENCES roles(id),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE delegations (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    delegator_position_id  uuid NOT NULL REFERENCES positions(id),
    delegate_user_id       uuid NOT NULL REFERENCES users(id),
    reason                 varchar(255) NOT NULL,
    valid_from             timestamptz NOT NULL,
    valid_to               timestamptz NOT NULL,
    created_by             uuid NOT NULL REFERENCES users(id),
    created_at             timestamptz NOT NULL DEFAULT now(),
    CHECK (valid_to > valid_from)
);
CREATE INDEX idx_delegations_active ON delegations(delegator_position_id, valid_from, valid_to);

-- ============ 2. Master Surat ============

CREATE TABLE letter_types (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code                   varchar(5) NOT NULL UNIQUE,  -- ND, MI, SE, SK, SPT, UND, BA, SP
    name                   varchar(100) NOT NULL,
    default_classification varchar(10) NOT NULL DEFAULT 'biasa'
                             CHECK (default_classification IN ('biasa','terbatas','rahasia')),
    default_sla_hours      int NOT NULL DEFAULT 24,
    is_active              boolean NOT NULL DEFAULT true
);

CREATE TABLE letter_templates (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    letter_type_id uuid NOT NULL REFERENCES letter_types(id),
    company_id     uuid NOT NULL REFERENCES companies(id),
    version        int NOT NULL DEFAULT 1,
    layout_config  jsonb NOT NULL DEFAULT '{}',
    body_skeleton  text NOT NULL DEFAULT '',
    is_active      boolean NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (letter_type_id, company_id, version)
);

CREATE TABLE numbering_formats (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id     uuid NOT NULL REFERENCES companies(id),
    letter_type_id uuid REFERENCES letter_types(id),
    org_unit_id    uuid REFERENCES org_units(id),
    pattern        varchar(150) NOT NULL,  -- '{seq:3}/{unit}/{type}/{roman_month}/{year}'
    reset_period   varchar(10) NOT NULL DEFAULT 'yearly'
                     CHECK (reset_period IN ('yearly','monthly')),
    is_active      boolean NOT NULL DEFAULT true
);

CREATE TABLE numbering_counters (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    format_id     uuid NOT NULL REFERENCES numbering_formats(id),
    scope_key     varchar(100) NOT NULL,  -- mis. 'HRGA-HO|ND|2026'
    current_value int NOT NULL DEFAULT 0,
    UNIQUE (format_id, scope_key)
    -- Pengambilan nomor WAJIB: SELECT ... FOR UPDATE dalam transaksi approval final.
);

CREATE TABLE approval_matrices (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    letter_type_id   uuid NOT NULL REFERENCES letter_types(id),
    originator_level varchar(30),
    final_level      varchar(30) NOT NULL,  -- level tertinggi wajib, mis. president_director
    flow_mode        varchar(10) NOT NULL DEFAULT 'serial'
                       CHECK (flow_mode IN ('serial','parallel')),
    extra_steps      jsonb NOT NULL DEFAULT '[]',
    is_active        boolean NOT NULL DEFAULT true
);

-- ============ 3. Surat & Siklus Hidup ============

CREATE TABLE letters (
    id                       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id               uuid NOT NULL REFERENCES companies(id),
    letter_type_id           uuid NOT NULL REFERENCES letter_types(id),
    letter_number            varchar(100) UNIQUE,  -- terisi saat approval final
    subject                  varchar(255) NOT NULL,
    classification           varchar(10) NOT NULL DEFAULT 'biasa'
                               CHECK (classification IN ('biasa','terbatas','rahasia')),
    priority                 varchar(10) NOT NULL DEFAULT 'normal'
                               CHECK (priority IN ('normal','urgent')),
    status                   varchar(15) NOT NULL DEFAULT 'draft'
                               CHECK (status IN ('draft','submitted','in_approval','revision',
                                                 'approved','published','cancelled','archived')),
    creator_user_id          uuid NOT NULL REFERENCES users(id),
    creator_position_id      uuid NOT NULL REFERENCES positions(id),
    on_behalf_of_position_id uuid REFERENCES positions(id),  -- mode sekretaris (a.n.)
    current_step_order       int,
    route_snapshot           jsonb NOT NULL DEFAULT '[]',
    org_snapshot_rev         varchar(20) NOT NULL DEFAULT 'REV-8',
    final_pdf_key            varchar(255),
    qr_token                 varchar(64) UNIQUE,
    refers_to_letter_id      uuid REFERENCES letters(id),
    external_ref             varchar(100),
    published_at             timestamptz,
    archived_at              timestamptz,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_letters_status_creator ON letters(status, creator_user_id);
CREATE INDEX idx_letters_company_published ON letters(company_id, published_at);

CREATE TABLE letter_versions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    letter_id  uuid NOT NULL REFERENCES letters(id),
    version    int NOT NULL,
    body_html  text NOT NULL DEFAULT '',
    body_plain text NOT NULL DEFAULT '',  -- untuk full-text search
    edited_by  uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (letter_id, version)
);
-- Full-text search: subject + isi versi terbaru (konfigurasi 'simple' agar netral bahasa)
CREATE INDEX idx_letter_versions_fts ON letter_versions
    USING GIN (to_tsvector('simple', body_plain));

CREATE TABLE letter_recipients (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    letter_id        uuid NOT NULL REFERENCES letters(id),
    recipient_type   varchar(2) NOT NULL CHECK (recipient_type IN ('to','cc')),
    position_id      uuid REFERENCES positions(id),
    org_unit_id      uuid REFERENCES org_units(id),
    resolved_user_id uuid REFERENCES users(id),  -- pemegang jabatan saat terbit (snapshot)
    delivered_at     timestamptz,
    CHECK (position_id IS NOT NULL OR org_unit_id IS NOT NULL)
);
CREATE INDEX idx_letter_recipients_letter ON letter_recipients(letter_id);
CREATE INDEX idx_letter_recipients_user ON letter_recipients(resolved_user_id);

CREATE TABLE letter_attachments (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    letter_id       uuid NOT NULL REFERENCES letters(id),
    file_name       varchar(255) NOT NULL,
    mime_type       varchar(100) NOT NULL,
    size_bytes      bigint NOT NULL,
    storage_key     varchar(255) NOT NULL,  -- key MinIO; akses via pre-signed URL
    checksum_sha256 varchar(64) NOT NULL,
    uploaded_by     uuid NOT NULL REFERENCES users(id),
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- ============ 4. Approval, Disposisi, Jejak ============

CREATE TABLE approval_steps (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    letter_id            uuid NOT NULL REFERENCES letters(id),
    step_order           int NOT NULL,
    approver_position_id uuid NOT NULL REFERENCES positions(id),
    flow_group           int NOT NULL DEFAULT 1,  -- sama = paralel
    status               varchar(10) NOT NULL DEFAULT 'pending'
                           CHECK (status IN ('pending','waiting','approved','rejected','skipped')),
    sla_deadline         timestamptz,
    UNIQUE (letter_id, step_order)
);
CREATE INDEX idx_approval_steps_waiting ON approval_steps(approver_position_id) WHERE status = 'waiting';

CREATE TABLE approval_actions (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    approval_step_id        uuid NOT NULL REFERENCES approval_steps(id),
    action                  varchar(20) NOT NULL
                              CHECK (action IN ('approve','reject','request_revision')),
    acted_by_user_id        uuid NOT NULL REFERENCES users(id),
    on_behalf_delegation_id uuid REFERENCES delegations(id),  -- terisi bila "a.n."
    note                    text,
    client_action_id        uuid UNIQUE,  -- idempotency utk retry offline Android
    device_info             varchar(255),
    ip_address              varchar(45),
    acted_at                timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE dispositions (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    letter_id             uuid NOT NULL REFERENCES letters(id),
    parent_disposition_id uuid REFERENCES dispositions(id),  -- disposisi berantai
    from_position_id      uuid NOT NULL REFERENCES positions(id),
    instruction           text NOT NULL,
    due_date              date,
    created_by            uuid NOT NULL REFERENCES users(id),
    created_at            timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE disposition_recipients (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    disposition_id          uuid NOT NULL REFERENCES dispositions(id),
    position_id             uuid NOT NULL REFERENCES positions(id),
    status                  varchar(15) NOT NULL DEFAULT 'open'
                              CHECK (status IN ('open','in_progress','done')),
    followup_note           text,
    followup_attachment_key varchar(255),
    completed_at            timestamptz
);

CREATE TABLE read_receipts (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    letter_id     uuid NOT NULL REFERENCES letters(id),
    user_id       uuid NOT NULL REFERENCES users(id),
    first_read_at timestamptz NOT NULL DEFAULT now(),
    last_read_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (letter_id, user_id)
);

CREATE TABLE comments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    letter_id  uuid NOT NULL REFERENCES letters(id),
    user_id    uuid NOT NULL REFERENCES users(id),
    body       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE notifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id),
    event_type varchar(50) NOT NULL,
    letter_id  uuid REFERENCES letters(id),
    title      varchar(255) NOT NULL,
    body       text NOT NULL DEFAULT '',
    channels   jsonb NOT NULL DEFAULT '["inapp"]',
    read_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_notifications_user_unread ON notifications(user_id) WHERE read_at IS NULL;

CREATE TABLE notification_preferences (
    user_id    uuid NOT NULL REFERENCES users(id),
    event_type varchar(50) NOT NULL,
    channels   jsonb NOT NULL DEFAULT '["inapp","push","email"]',
    PRIMARY KEY (user_id, event_type)
);

-- Append-only: aplikasi hanya INSERT; UPDATE/DELETE dicabut dari role aplikasi
-- saat provisioning production (lihat docs/DATABASE-SCHEMA.md).
CREATE TABLE audit_logs (
    id            bigserial PRIMARY KEY,
    entity_type   varchar(30) NOT NULL,
    entity_id     uuid,
    action        varchar(30) NOT NULL,
    actor_user_id uuid REFERENCES users(id),  -- NULL = aksi sistem
    detail        jsonb NOT NULL DEFAULT '{}',
    ip_address    varchar(45),
    user_agent    varchar(255),
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);

-- ============ 5. Seed minimal ============

INSERT INTO companies (code, name) VALUES ('KSK', 'PT Kalimantan Sawit Kusuma');

INSERT INTO roles (code, name) VALUES
    ('admin',     'Administrator Sistem'),
    ('creator',   'Pembuat Surat'),
    ('approver',  'Penyetuju'),
    ('secretary', 'Sekretaris'),
    ('auditor',   'Auditor Inspectorate');

INSERT INTO letter_types (code, name, default_classification, default_sla_hours) VALUES
    ('ND',  'Nota Dinas',        'biasa',    24),
    ('MI',  'Memo Internal',     'biasa',    24),
    ('SE',  'Surat Edaran',      'biasa',    48),
    ('SK',  'Surat Keputusan',   'terbatas', 48),
    ('SPT', 'Surat Perintah/Tugas', 'biasa', 24),
    ('UND', 'Surat Undangan',    'biasa',    12),
    ('BA',  'Berita Acara',      'biasa',    48),
    ('SP',  'Surat Peringatan',  'rahasia',  48);

COMMIT;
