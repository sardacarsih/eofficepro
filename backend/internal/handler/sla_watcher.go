package handler

// Pengawas SLA approval (E03-6): reminder saat 50% SLA berlalu dan eskalasi
// (approver + atasannya) saat SLA terlewati. Dedup lewat tabel notifications
// (satu sla_reminder / sla_escalation per pengguna per surat).

import (
	"context"
	"log"
	"time"
)

const slaCheckInterval = 5 * time.Minute

// RunSLAWatcher memeriksa SLA secara berkala sampai ctx selesai.
// Jalankan sebagai goroutine dari main.
func (h *Handler) RunSLAWatcher(ctx context.Context) {
	ticker := time.NewTicker(slaCheckInterval)
	defer ticker.Stop()
	for {
		h.checkSLA(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *Handler) checkSLA(ctx context.Context) {
	reminderEmails, err := h.insertSLAReminders(ctx)
	if err != nil {
		log.Printf("sla watcher: reminder gagal: %v", err)
	}
	escalationEmails, err := h.insertSLAEscalations(ctx)
	if err != nil {
		log.Printf("sla watcher: eskalasi gagal: %v", err)
	}
	if err := h.enqueueNotificationOutboxDirect(ctx, append(reminderEmails, escalationEmails...)); err != nil {
		log.Printf("sla watcher: antre notifikasi gagal: %v", err)
	}
}

func (h *Handler) insertSLAReminders(ctx context.Context) ([]notificationEmail, error) {
	rows, err := h.DB.Query(ctx, `
		WITH due AS (
			SELECT s.letter_id, s.approver_position_id, s.sla_deadline, l.subject
			FROM approval_steps s
			JOIN letters l ON l.id = s.letter_id
			JOIN letter_types lt ON lt.id = l.letter_type_id
			WHERE s.status = 'waiting'
			  AND l.status = 'in_approval'
			  AND s.sla_deadline IS NOT NULL
			  AND now() >= s.sla_deadline - make_interval(mins => GREATEST(lt.default_sla_hours, 1) * 30)
			  AND now() < s.sla_deadline
		),
		targets AS (
			SELECT DISTINCT user_id, letter_id, subject, sla_deadline
			FROM (
				SELECT up.user_id, d.letter_id, d.subject, d.sla_deadline
				FROM due d
				JOIN user_positions up ON up.position_id = d.approver_position_id
				JOIN users u ON u.id = up.user_id
				WHERE current_date >= up.valid_from
				  AND (up.valid_to IS NULL OR current_date < up.valid_to)
				  AND u.status = 'active'
				UNION
				-- Delegate aktif (E03-5) ikut diingatkan.
				SELECT dg.delegate_user_id, d.letter_id, d.subject, d.sla_deadline
				FROM due d
				JOIN delegations dg ON dg.delegator_position_id = d.approver_position_id
				JOIN users u ON u.id = dg.delegate_user_id
				WHERE now() >= dg.valid_from AND now() < dg.valid_to
				  AND dg.revoked_at IS NULL
				  AND u.status = 'active'
			) combined
		),
		inserted AS (
			INSERT INTO notifications (user_id, event_type, letter_id, title, body)
			SELECT t.user_id, 'sla_reminder', t.letter_id,
			       'SLA hampir habis: ' || t.subject,
			       'Approval surat ini menunggu Anda dan mendekati batas SLA ('
			         || to_char(t.sla_deadline AT TIME ZONE 'Asia/Jakarta', 'DD Mon YYYY HH24:MI') || ' WIB).'
			FROM targets t
			WHERE NOT EXISTS (
				SELECT 1 FROM notifications n
				WHERE n.user_id = t.user_id
				  AND n.letter_id = t.letter_id
				  AND n.event_type = 'sla_reminder'
			)
			RETURNING user_id, event_type, letter_id, title, body
		)
		SELECT u.email, i.event_type, i.letter_id::text, i.title, i.body
		FROM inserted i
		JOIN users u ON u.id = i.user_id`)
	if err != nil {
		return nil, err
	}
	return collectNotificationEmails(rows)
}

func (h *Handler) insertSLAEscalations(ctx context.Context) ([]notificationEmail, error) {
	rows, err := h.DB.Query(ctx, `
		WITH due AS (
			SELECT s.letter_id, s.approver_position_id, p.reports_to, p.title AS position_title, l.subject
			FROM approval_steps s
			JOIN letters l ON l.id = s.letter_id
			JOIN positions p ON p.id = s.approver_position_id
			WHERE s.status = 'waiting'
			  AND l.status = 'in_approval'
			  AND s.sla_deadline IS NOT NULL
			  AND now() >= s.sla_deadline
		),
		targets AS (
			SELECT DISTINCT ON (user_id, letter_id) user_id, letter_id, subject, position_title, is_superior
			FROM (
				SELECT up.user_id, d.letter_id, d.subject, d.position_title, false AS is_superior
				FROM due d
				JOIN user_positions up ON up.position_id = d.approver_position_id
				JOIN users u ON u.id = up.user_id
				WHERE current_date >= up.valid_from
				  AND (up.valid_to IS NULL OR current_date < up.valid_to)
				  AND u.status = 'active'
				UNION ALL
				-- Delegate aktif (E03-5) menerima eskalasi sebagai penindak lanjut.
				SELECT dg.delegate_user_id, d.letter_id, d.subject, d.position_title, false AS is_superior
				FROM due d
				JOIN delegations dg ON dg.delegator_position_id = d.approver_position_id
				JOIN users u ON u.id = dg.delegate_user_id
				WHERE now() >= dg.valid_from AND now() < dg.valid_to
				  AND dg.revoked_at IS NULL
				  AND u.status = 'active'
				UNION ALL
				SELECT up.user_id, d.letter_id, d.subject, d.position_title, true AS is_superior
				FROM due d
				JOIN user_positions up ON up.position_id = d.reports_to
				JOIN users u ON u.id = up.user_id
				WHERE d.reports_to IS NOT NULL
				  AND current_date >= up.valid_from
				  AND (up.valid_to IS NULL OR current_date < up.valid_to)
				  AND u.status = 'active'
			) combined
			ORDER BY user_id, letter_id, is_superior
		),
		inserted AS (
			INSERT INTO notifications (user_id, event_type, letter_id, title, body)
			SELECT t.user_id, 'sla_escalation', t.letter_id,
			       'SLA terlewati: ' || t.subject,
			       CASE WHEN t.is_superior
			            THEN 'Approval oleh bawahan Anda (' || t.position_title || ') telah melewati batas SLA. Mohon bantu tindak lanjut.'
			            ELSE 'Approval surat ini telah melewati batas SLA. Segera tindak lanjuti.'
			       END
			FROM targets t
			WHERE NOT EXISTS (
				SELECT 1 FROM notifications n
				WHERE n.user_id = t.user_id
				  AND n.letter_id = t.letter_id
				  AND n.event_type = 'sla_escalation'
			)
			RETURNING user_id, event_type, letter_id, title, body
		)
		SELECT u.email, i.event_type, i.letter_id::text, i.title, i.body
		FROM inserted i
		JOIN users u ON u.id = i.user_id`)
	if err != nil {
		return nil, err
	}
	return collectNotificationEmails(rows)
}
