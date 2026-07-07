// Package mail mengirim email transaksional (reset password, notifikasi E08).
// Tanpa SMTP_HOST, berjalan dalam mode dev: isi email dicetak ke log aplikasi
// sehingga alur tetap bisa diuji tanpa server email.
package mail

import (
	"fmt"
	"log"
	"net/smtp"

	"github.com/kskgroup/eofficepro/internal/config"
)

type Mailer struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Mailer {
	return &Mailer{cfg: cfg}
}

func (m *Mailer) Send(to, subject, body string) error {
	if m.cfg.SMTPHost == "" {
		log.Printf("MAIL (mode dev, tidak terkirim)\n  To: %s\n  Subject: %s\n  Body: %s", to, subject, body)
		return nil
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		m.cfg.SMTPFrom, to, subject, body)

	addr := fmt.Sprintf("%s:%d", m.cfg.SMTPHost, m.cfg.SMTPPort)
	var auth smtp.Auth
	if m.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPass, m.cfg.SMTPHost)
	}
	return smtp.SendMail(addr, auth, m.cfg.SMTPFrom, []string{to}, []byte(msg))
}
