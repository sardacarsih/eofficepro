BEGIN;

UPDATE approval_matrices am
SET max_final_level = CASE
        WHEN ac.code = 'SDM' AND am.originator_level IN ('division_head', 'sub_dept_head') THEN 'director'
        WHEN ac.code = 'KEUANGAN' AND am.originator_level IN ('dept_head', 'gm') THEN 'vp_director'
        ELSE am.max_final_level
    END,
    updated_at = now()
FROM approval_categories ac
JOIN letter_types lt ON lt.code = 'PRS'
WHERE am.approval_category_id = ac.id
  AND am.letter_type_id = lt.id
  AND am.resolution_mode = 'user_selected';

COMMIT;
