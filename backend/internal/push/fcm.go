// Package push mengirim push notification melalui Firebase Cloud Messaging.
package push

import (
	"context"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"

	"github.com/kskgroup/eofficepro/internal/config"
)

type Message struct {
	Title          string
	Body           string
	LetterID       string
	Event          string
	TargetSection  string
	NotificationID string
}

type Client struct {
	enabled bool
	fcm     *messaging.Client
}

func New(ctx context.Context, cfg *config.Config) *Client {
	if !cfg.FirebaseCloudMessagingEnabled {
		log.Printf("FCM disabled: FIREBASE_CLOUD_MESSAGING_ENABLED tidak aktif")
		return &Client{}
	}

	opts := []option.ClientOption{}
	if cfg.FirebaseCredentialsJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(cfg.FirebaseCredentialsJSON)))
	} else if cfg.FirebaseCredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.FirebaseCredentialsFile))
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: cfg.FirebaseProjectID}, opts...)
	if err != nil {
		log.Printf("FCM disabled: initialize Firebase app failed: %v", err)
		return &Client{}
	}
	fcm, err := app.Messaging(ctx)
	if err != nil {
		log.Printf("FCM disabled: initialize messaging client failed: %v", err)
		return &Client{}
	}
	log.Printf("FCM enabled (project %s)", cfg.FirebaseProjectID)
	return &Client{enabled: true, fcm: fcm}
}

func (c *Client) Enabled() bool {
	return c != nil && c.enabled && c.fcm != nil
}

func (c *Client) SendToTokens(ctx context.Context, tokens []string, msg Message) []string {
	invalidTokens := []string{}
	if !c.Enabled() || len(tokens) == 0 {
		return invalidTokens
	}
	for _, token := range tokens {
		if token == "" {
			continue
		}
		data := map[string]string{
			"event_type":     msg.Event,
			"letter_id":      msg.LetterID,
			"target_section": msg.TargetSection,
		}
		if msg.NotificationID != "" {
			data["notification_id"] = msg.NotificationID
		}
		payload := &messaging.Message{
			Token: token,
			Notification: &messaging.Notification{
				Title: msg.Title,
				Body:  msg.Body,
			},
			Data: data,
			Android: &messaging.AndroidConfig{
				Priority: "high",
				Notification: &messaging.AndroidNotification{
					ChannelID: "eoffice_priority",
				},
			},
		}
		if _, err := c.fcm.Send(ctx, payload); err != nil {
			log.Printf("FCM send failed: %v", err)
			if messaging.IsRegistrationTokenNotRegistered(err) || messaging.IsInvalidArgument(err) {
				invalidTokens = append(invalidTokens, token)
			}
		}
	}
	return invalidTokens
}
