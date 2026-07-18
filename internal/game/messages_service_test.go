package game

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// fakeMailer records sends and can be told to fail or report disabled.
type fakeMailer struct {
	enabled  bool
	err      error
	subjects []string
	bodies   []string
}

func (m *fakeMailer) Enabled() bool { return m.enabled }
func (m *fakeMailer) Send(subject, body string) error {
	if m.err != nil {
		return m.err
	}
	m.subjects = append(m.subjects, subject)
	m.bodies = append(m.bodies, body)
	return nil
}

func testMessageService(m Mailer) (*Service, *fakeStore) {
	svc, store := testService()
	svc.mailer = m
	return svc, store
}

func TestSendMessageEmailsAndSaves(t *testing.T) {
	mail := &fakeMailer{enabled: true}
	svc, store := testMessageService(mail)

	msg, err := svc.SendMessage(context.Background(), "bug", "  the numpad broke!  ",
		&MessageContext{Version: "v1.0", Route: "#/play", UserAgent: "iPad"})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if msg.Body != "the numpad broke!" {
		t.Errorf("body not trimmed: %q", msg.Body)
	}
	if !msg.Emailed || msg.EmailError != "" {
		t.Errorf("want emailed=true with no error, got emailed=%v error=%q", msg.Emailed, msg.EmailError)
	}
	if len(mail.subjects) != 1 || !strings.Contains(mail.subjects[0], "Bug") {
		t.Errorf("unexpected subjects: %v", mail.subjects)
	}
	if !strings.Contains(mail.bodies[0], "the numpad broke!") ||
		!strings.Contains(mail.bodies[0], "Screen: #/play") ||
		!strings.Contains(mail.bodies[0], "App version: v1.0") {
		t.Errorf("email body missing text or context footer:\n%s", mail.bodies[0])
	}
	saved, _ := store.ListMessages(context.Background())
	if len(saved) != 1 || !saved[0].Emailed {
		t.Errorf("stored row should exist with emailed=true, got %+v", saved)
	}
}

func TestSendMessageMailerFailureStillSaves(t *testing.T) {
	mail := &fakeMailer{enabled: true, err: errors.New("smtp: connection refused")}
	svc, store := testMessageService(mail)

	msg, err := svc.SendMessage(context.Background(), "idea", "add a snorlax game", nil)
	if err != nil {
		t.Fatalf("SendMessage must not fail on a mailer error, got: %v", err)
	}
	if msg.Emailed {
		t.Error("emailed should be false after a send failure")
	}
	if !strings.Contains(msg.EmailError, "connection refused") {
		t.Errorf("email_error should record the failure, got %q", msg.EmailError)
	}
	saved, _ := store.ListMessages(context.Background())
	if len(saved) != 1 || saved[0].Emailed || !strings.Contains(saved[0].EmailError, "connection refused") {
		t.Errorf("stored row should keep the error, got %+v", saved)
	}
}

func TestSendMessageDisabledMailerSavesUnsent(t *testing.T) {
	svc, store := testMessageService(&fakeMailer{enabled: false})

	msg, err := svc.SendMessage(context.Background(), "", "hi uncle jim", nil)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if msg.Kind != MessageKindMessage {
		t.Errorf("empty kind should default to message, got %q", msg.Kind)
	}
	if msg.Emailed || msg.EmailError != "" {
		t.Errorf("disabled mailer: want emailed=false with no error, got emailed=%v error=%q", msg.Emailed, msg.EmailError)
	}
	saved, _ := store.ListMessages(context.Background())
	if len(saved) != 1 {
		t.Fatalf("message should be saved even with mail off, got %d rows", len(saved))
	}
}

func TestSendMessageRateLimit(t *testing.T) {
	svc, store := testMessageService(&fakeMailer{enabled: false})
	ctx := context.Background()

	for i := 0; i < messageRateLimit; i++ {
		if _, err := svc.SendMessage(ctx, "message", fmt.Sprintf("note %d", i), nil); err != nil {
			t.Fatalf("message %d: %v", i, err)
		}
	}
	_, err := svc.SendMessage(ctx, "message", "one too many", nil)
	if !errors.Is(err, ErrMessageRateLimited) {
		t.Fatalf("11th message in an hour: want ErrMessageRateLimited, got %v", err)
	}
	saved, _ := store.ListMessages(ctx)
	if len(saved) != messageRateLimit {
		t.Errorf("over-cap message must not be saved: want %d rows, got %d", messageRateLimit, len(saved))
	}
}

func TestSendMessageValidation(t *testing.T) {
	svc, _ := testMessageService(&fakeMailer{enabled: false})
	ctx := context.Background()

	cases := []struct {
		name, kind, body string
	}{
		{"bad kind", "shout", "hello"},
		{"empty body", "bug", "   "},
		{"too long", "bug", strings.Repeat("a", maxMessageChars+1)},
	}
	for _, tc := range cases {
		_, err := svc.SendMessage(ctx, tc.kind, tc.body, nil)
		if err == nil || !strings.HasPrefix(err.Error(), "invalid:") {
			t.Errorf("%s: want invalid: error, got %v", tc.name, err)
		}
	}
	// Exactly at the cap is fine.
	if _, err := svc.SendMessage(ctx, "bug", strings.Repeat("a", maxMessageChars), nil); err != nil {
		t.Errorf("body at exactly %d chars should pass, got %v", maxMessageChars, err)
	}
}

func TestMarkMessageReadAndUnreadCount(t *testing.T) {
	svc, _ := testMessageService(&fakeMailer{enabled: false})
	ctx := context.Background()

	m1, _ := svc.SendMessage(ctx, "message", "first", nil)
	svc.SendMessage(ctx, "message", "second", nil)

	if n, _ := svc.CountUnreadMessages(ctx); n != 2 {
		t.Errorf("want 2 unread, got %d", n)
	}
	if err := svc.MarkMessageRead(ctx, m1.ID); err != nil {
		t.Fatalf("MarkMessageRead: %v", err)
	}
	if n, _ := svc.CountUnreadMessages(ctx); n != 1 {
		t.Errorf("want 1 unread after marking, got %d", n)
	}
	if err := svc.MarkMessageRead(ctx, 999); err == nil || !strings.HasPrefix(err.Error(), "not found:") {
		t.Errorf("unknown id: want not found: error, got %v", err)
	}
}
