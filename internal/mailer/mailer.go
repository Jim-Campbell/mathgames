// Package mailer sends plain-text email over SMTP with STARTTLS, using only
// the standard library (net/smtp negotiates STARTTLS on port 587 by itself).
// Built for Gmail with an app password, but any STARTTLS SMTP server works.
package mailer

import (
	"fmt"
	"mime"
	"net/smtp"
	"strings"
)

// SMTP implements game.Mailer. A zero-credential SMTP (no user/pass) is the
// disabled state: Enabled() is false and Send refuses with a clear error, so
// the service saves messages unsent instead of half-trying.
type SMTP struct {
	host, port, user, pass, to string
}

// New builds an SMTP mailer. to defaults to user (mail-to-self) when empty.
// The recipient comes only from server config, never from a request -- that's
// what keeps the messages endpoint from being a spam relay.
func New(host, port, user, pass, to string) *SMTP {
	if to == "" {
		to = user
	}
	return &SMTP{host: host, port: port, user: user, pass: pass, to: to}
}

// Enabled reports whether credentials are configured.
func (m *SMTP) Enabled() bool {
	return m.user != "" && m.pass != ""
}

// Send delivers one plain-text message. From is always the authenticated
// account -- Gmail rewrites or rejects anything else.
func (m *SMTP) Send(subject, body string) error {
	if !m.Enabled() {
		return fmt.Errorf("mailer not configured (set SMTP_USER and SMTP_PASS)")
	}
	msg := "From: " + m.user + "\r\n" +
		"To: " + m.to + "\r\n" +
		"Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: 8bit\r\n" +
		"\r\n" +
		toCRLF(body)
	auth := smtp.PlainAuth("", m.user, m.pass, m.host)
	if err := smtp.SendMail(m.host+":"+m.port, auth, m.user, []string{m.to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}

// toCRLF normalizes body line endings to the CRLF that RFC 5322 requires.
func toCRLF(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
}
