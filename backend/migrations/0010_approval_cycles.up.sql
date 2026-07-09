BEGIN;

ALTER TABLE approval_steps
    ADD COLUMN approval_cycle int NOT NULL DEFAULT 1
        CHECK (approval_cycle > 0);

ALTER TABLE approval_steps
    DROP CONSTRAINT approval_steps_letter_id_step_order_key;

ALTER TABLE approval_steps
    ADD CONSTRAINT approval_steps_letter_cycle_order_key
        UNIQUE (letter_id, approval_cycle, step_order);

COMMIT;
