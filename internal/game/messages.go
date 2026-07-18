package game

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Message limits: body length and the rolling-hour rate cap (an 8-year-old
// with a send button can flood an inbox fast).
const (
	maxMessageChars   = 2000
	messageRateLimit  = 10
	messageRateWindow = time.Hour
)

// ErrMessageRateLimited is returned by SendMessage when the rolling-hour cap
// is hit; the handler maps it to 429. The message is NOT saved in that case.
var ErrMessageRateLimited = errors.New("message rate limit reached")

// Mailer is the email-delivery dependency SendMessage uses, implemented by
// internal/mailer for real SMTP and by a fake in tests. A disabled mailer
// (missing SMTP creds) reports Enabled() == false and the service skips the
// send entirely, leaving the row saved with emailed=false.
type Mailer interface {
	Enabled() bool
	Send(subject, body string) error
}

// messageSubjects maps a message kind to its email subject line.
var messageSubjects = map[string]string{
	MessageKindBug:     "[MathGames] 🐛 Bug from Skyler",
	MessageKindIdea:    "[MathGames] 💡 Idea from Skyler",
	MessageKindMessage: "[MathGames] 💬 Message from Skyler",
}

// SendMessage validates and saves one message from the kid, then attempts
// best-effort email delivery. Save always, email best-effort: a send failure
// records email_error on the row but never fails the request -- the kid sees
// "Sent!" either way, and Jim sees every message in the parents inbox.
func (s *Service) SendMessage(ctx context.Context, kind, body string, msgCtx *MessageContext) (*Message, error) {
	if kind == "" {
		kind = MessageKindMessage
	}
	switch kind {
	case MessageKindBug, MessageKindIdea, MessageKindMessage:
	default:
		return nil, fmt.Errorf("invalid: kind must be one of bug, idea, message")
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("invalid: body is required")
	}
	if utf8.RuneCountInString(body) > maxMessageChars {
		return nil, fmt.Errorf("invalid: body must be at most %d characters", maxMessageChars)
	}

	recent, err := s.store.CountMessagesSince(ctx, time.Now().Add(-messageRateWindow))
	if err != nil {
		return nil, fmt.Errorf("count recent messages: %w", err)
	}
	if recent >= messageRateLimit {
		return nil, ErrMessageRateLimited
	}

	m := &Message{Kind: kind, Body: body, Context: msgCtx}
	if err := s.store.InsertMessage(ctx, m); err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}

	if s.mailer != nil && s.mailer.Enabled() {
		if err := s.mailer.Send(messageSubjects[kind], messageEmailBody(m)); err != nil {
			m.EmailError = err.Error()
			s.log.Warn("message email failed", "message_id", m.ID, "error", err)
		} else {
			m.Emailed = true
		}
		if err := s.store.UpdateMessageEmailStatus(ctx, m.ID, m.Emailed, m.EmailError); err != nil {
			// The message itself is saved; losing the status update isn't
			// worth failing the kid's request over.
			s.log.Warn("update message email status failed", "message_id", m.ID, "error", err)
		}
	}

	return m, nil
}

// messageEmailBody is the message text plus a context footer so a bug report
// carries the screen it came from.
func messageEmailBody(m *Message) string {
	var b strings.Builder
	b.WriteString(m.Body)
	b.WriteString("\n\n—\nKind: ")
	b.WriteString(m.Kind)
	if m.Context != nil {
		if m.Context.Route != "" {
			b.WriteString("\nScreen: " + m.Context.Route)
		}
		if m.Context.Version != "" {
			b.WriteString("\nApp version: " + m.Context.Version)
		}
		if m.Context.UserAgent != "" {
			b.WriteString("\nUser agent: " + m.Context.UserAgent)
		}
	}
	b.WriteString("\nSent: " + m.CreatedAt.UTC().Format("2006-01-02 15:04:05 UTC"))
	return b.String()
}

// ListMessages returns every message, newest first, for the parents inbox.
func (s *Service) ListMessages(ctx context.Context) ([]Message, error) {
	msgs, err := s.store.ListMessages(ctx)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	return msgs, nil
}

// CountUnreadMessages backs the parents inbox badge.
func (s *Service) CountUnreadMessages(ctx context.Context) (int, error) {
	n, err := s.store.CountUnreadMessages(ctx)
	if err != nil {
		return 0, fmt.Errorf("count unread messages: %w", err)
	}
	return n, nil
}

// MarkMessageRead sets read_at on one message. The store's "not found:"
// sentinel passes through unwrapped so the handler routes it to 404.
func (s *Service) MarkMessageRead(ctx context.Context, id int64) error {
	return s.store.MarkMessageRead(ctx, id)
}
