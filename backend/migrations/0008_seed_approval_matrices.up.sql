WITH matrix_seed(id, letter_type_code) AS (
    VALUES
        ('a8000000-0000-0000-0000-000000000001'::uuid, 'ND'),
        ('a8000000-0000-0000-0000-000000000002'::uuid, 'MI'),
        ('a8000000-0000-0000-0000-000000000003'::uuid, 'SE'),
        ('a8000000-0000-0000-0000-000000000004'::uuid, 'SK'),
        ('a8000000-0000-0000-0000-000000000005'::uuid, 'SPT'),
        ('a8000000-0000-0000-0000-000000000006'::uuid, 'UND'),
        ('a8000000-0000-0000-0000-000000000007'::uuid, 'BA'),
        ('a8000000-0000-0000-0000-000000000008'::uuid, 'SP')
)
INSERT INTO approval_matrices (
    id,
    letter_type_id,
    originator_level,
    final_level,
    flow_mode,
    extra_steps,
    is_active
)
SELECT
    matrix_seed.id,
    letter_types.id,
    NULL,
    'director',
    'serial',
    '[]'::jsonb,
    true
FROM matrix_seed
JOIN letter_types ON letter_types.code = matrix_seed.letter_type_code
WHERE NOT EXISTS (
    SELECT 1
    FROM approval_matrices existing
    WHERE existing.letter_type_id = letter_types.id
      AND existing.originator_level IS NULL
      AND existing.is_active
);
