BEGIN;

UPDATE approval_matrices am
SET max_final_level = defaults.max_level,
    updated_at = now()
FROM approval_categories ac,
     (VALUES
        ('staff', 'director'), ('assistant', 'director'),
        ('division_head', 'director'), ('sub_dept_head', 'director'),
        ('dept_head', 'vp_director'), ('gm', 'vp_director'),
        ('director', 'president_director'),
        ('vp_director', 'president_director'), ('secretary', 'director')
     ) AS defaults(originator_level, max_level)
WHERE am.approval_category_id = ac.id
  AND am.originator_level = defaults.originator_level
  AND am.resolution_mode = 'user_selected';

COMMIT;
