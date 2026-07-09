UPDATE approval_matrices
SET final_level = 'director',
    updated_at = now()
WHERE id IN (
    'a8000000-0000-0000-0000-000000000003'::uuid,
    'a8000000-0000-0000-0000-000000000004'::uuid
);
