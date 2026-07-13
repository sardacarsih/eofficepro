BEGIN;

-- Kategori mempunyai profil kewenangan berbeda. Nilai ini merupakan default
-- operasional dan tetap dapat disesuaikan administrator melalui API policy.
UPDATE approval_matrices am
SET max_final_level = CASE
        WHEN ac.code IN ('SDM', 'KEUANGAN') AND am.originator_level IN ('dept_head', 'gm') THEN 'vp_director'
        WHEN ac.code IN ('SDM', 'KEUANGAN') AND am.originator_level = 'director' THEN 'president_director'
        WHEN ac.code = 'LEGAL' AND am.originator_level IN ('division_head', 'sub_dept_head') THEN 'vp_director'
        WHEN ac.code = 'LEGAL' AND am.originator_level IN ('dept_head', 'gm', 'director') THEN 'president_director'
        ELSE am.max_final_level
    END,
    updated_at = now()
FROM approval_categories ac
JOIN letter_types lt ON lt.code = 'PRS'
WHERE am.approval_category_id = ac.id
  AND am.letter_type_id = lt.id
  AND am.resolution_mode = 'user_selected';

COMMIT;
