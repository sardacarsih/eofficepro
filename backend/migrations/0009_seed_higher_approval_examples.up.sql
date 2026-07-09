UPDATE approval_matrices
SET final_level = 'vp_director',
    updated_at = now()
WHERE id = 'a8000000-0000-0000-0000-000000000003'::uuid;

UPDATE approval_matrices
SET final_level = 'president_director',
    updated_at = now()
WHERE id = 'a8000000-0000-0000-0000-000000000004'::uuid;
