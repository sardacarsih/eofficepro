BEGIN;

DELETE FROM approval_actions
WHERE approval_step_id IN (
    SELECT id
    FROM approval_steps
    WHERE approval_cycle > 1
);

DELETE FROM approval_steps
WHERE approval_cycle > 1;

ALTER TABLE approval_steps
    DROP CONSTRAINT approval_steps_letter_cycle_order_key;

ALTER TABLE approval_steps
    DROP COLUMN approval_cycle;

ALTER TABLE approval_steps
    ADD CONSTRAINT approval_steps_letter_id_step_order_key
        UNIQUE (letter_id, step_order);

COMMIT;
