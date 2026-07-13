package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const notificationOutboxInterval = 5 * time.Second

// enqueueNotificationOutbox records delivery work in the caller's business
// transaction. A committed notification can therefore survive process restarts.
func enqueueNotificationOutbox(ctx context.Context, tx pgx.Tx, items []notificationEmail) error {
	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO notification_outbox
				(recipient_email, event_type, letter_id, title, body)
			VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5)`,
			item.Email, item.EventType, item.LetterID, item.Title, item.Body); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) enqueueNotificationOutboxDirect(ctx context.Context, items []notificationEmail) error {
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := enqueueNotificationOutbox(ctx, tx, items); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RunNotificationOutboxWorker delivers email and push work with retry rather
// than relying on a best-effort goroutine after the request has committed.
func (h *Handler) RunNotificationOutboxWorker(ctx context.Context) {
	ticker := time.NewTicker(notificationOutboxInterval)
	defer ticker.Stop()
	for {
		for h.processNextNotificationOutbox(ctx) {
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *Handler) processNextNotificationOutbox(ctx context.Context) bool {
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return false
	}
	defer tx.Rollback(ctx)
	var id string
	var item notificationEmail
	err = tx.QueryRow(ctx, `
		SELECT id::text, recipient_email, event_type, COALESCE(letter_id::text, ''), title, body
		FROM notification_outbox
		WHERE status IN ('pending', 'retry') AND available_at <= now()
		ORDER BY created_at
		FOR UPDATE SKIP LOCKED
		LIMIT 1`).Scan(&id, &item.Email, &item.EventType, &item.LetterID, &item.Title, &item.Body)
	if err != nil {
		return false
	}
	if _, err := tx.Exec(ctx, `
		UPDATE notification_outbox
		SET status = 'processing', attempts = attempts + 1, updated_at = now()
		WHERE id = $1`, id); err != nil {
		return false
	}
	if err := tx.Commit(ctx); err != nil {
		return false
	}

	if err := h.deliverNotification(ctx, item); err != nil {
		_, _ = h.DB.Exec(ctx, `
			UPDATE notification_outbox
			SET status = 'retry', available_at = now() + make_interval(secs => LEAST(3600, 30 * attempts)),
				last_error = $2, updated_at = now()
			WHERE id = $1`, id, err.Error())
		return true
	}
	_, _ = h.DB.Exec(ctx, `
		UPDATE notification_outbox
		SET status = 'delivered', delivered_at = now(), last_error = NULL, updated_at = now()
		WHERE id = $1`, id)
	return true
}

func (h *Handler) deliverNotification(ctx context.Context, item notificationEmail) error {
	link := h.notificationLink(item)
	if err := h.Mailer.Send(item.Email, item.Title, item.Body+"\n\nBuka di eOffice Pro:\n"+link); err != nil {
		return fmt.Errorf("mengirim email: %w", err)
	}
	if h.Push != nil && h.Push.Enabled() {
		tokens, err := h.pushTokensForEmail(ctx, item.Email)
		if err != nil {
			return fmt.Errorf("memuat token push: %w", err)
		}
		if len(tokens) > 0 {
			classification, err := h.pushNotificationClassification(ctx, item.LetterID)
			if err != nil {
				return fmt.Errorf("memuat klasifikasi push: %w", err)
			}
			invalid := h.Push.SendToTokens(ctx, tokens, buildPushMessage(item, classification))
			if len(invalid) > 0 {
				if err := h.deletePushTokens(ctx, invalid); err != nil {
					return fmt.Errorf("menghapus token push invalid: %w", err)
				}
			}
		}
	}
	return nil
}
