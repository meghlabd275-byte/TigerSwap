package services

import (
	"errors"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
)

// Mailer delivers transactional mail (password reset, email verification).
//
// TigerSwap does not bake in a third-party mail API. The operator configures an
// SMTP relay through the environment; when none is configured the mailer is
// disabled and the recovery endpoints answer with an explicit error rather than
// silently pretending a message was sent. This mirrors the rest of the platform:
// an action that cannot do real work says so.
type Mailer struct {
	host     string
	port     string
	username string
	password string
	from     string
	// devLog, when set, logs the message body instead of sending it. It exists
	// only for a local run without an SMTP relay and is never enabled in
	// production because it would put reset links in the log.
	devLog bool
}

// NewMailer reads SMTP configuration from the environment. It returns a disabled
// mailer (Enabled() == false) when no relay is configured, which the callers
// treat as "cannot send".
func NewMailer() *Mailer {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if port == "" {
		port = "587"
	}
	mailer := &Mailer{
		host:     host,
		port:     port,
		username: os.Getenv("SMTP_USERNAME"),
		password: os.Getenv("SMTP_PASSWORD"),
		from:     from,
	}
	if host == "" || from == "" {
		mailer.devLog = strings.EqualFold(os.Getenv("MAIL_DEV_LOG"), "true")
	}
	return mailer
}

// Enabled reports whether the mailer can actually deliver a message.
func (m *Mailer) Enabled() bool {
	if m == nil {
		return false
	}
	return m.host != "" && m.from != ""
}

// Send delivers a plain-text message. When the mailer is disabled it returns an
// explicit error so the caller can surface "mail is not configured" instead of
// reporting a delivery that never happened.
func (m *Mailer) Send(to, subject, body string) error {
	if m == nil {
		return errors.New("mail is not configured")
	}
	if !m.Enabled() {
		if m.devLog {
			// Local-only aid. Never set MAIL_DEV_LOG in production: the body
			// contains a single-use recovery link.
			log.Printf("mail (dev-log) to=%s subject=%q\n%s", to, subject, body)
			return nil
		}
		return errors.New("mail is not configured")
	}

	message := buildMessage(m.from, to, subject, body)
	address := m.host + ":" + m.port

	var auth smtp.Auth
	if m.username != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}
	if err := smtp.SendMail(address, auth, m.from, []string{to}, []byte(message)); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}

// buildMessage assembles an RFC 5322 message. The headers are fixed by the
// server, so a user-supplied value cannot inject a header.
func buildMessage(from, to, subject, body string) string {
	var b strings.Builder
	b.WriteString("From: " + sanitizeHeader(from) + "\r\n")
	b.WriteString("To: " + sanitizeHeader(to) + "\r\n")
	b.WriteString("Subject: " + sanitizeHeader(subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return b.String()
}

// sanitizeHeader strips CR/LF so a value cannot inject an extra header.
func sanitizeHeader(value string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(value)
}
