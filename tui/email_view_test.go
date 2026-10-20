package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/floatpane/matcha/fetcher"
)

func TestEmailViewUpdate(t *testing.T) {
	emailWithAttachments := fetcher.Email{
		From:    "test@example.com",
		Subject: "Test Email with Attachments",
		Body:    "This is the body.",
		Date:    time.Now(),
		Attachments: []fetcher.Attachment{
			{Filename: "attachment1.txt", Data: []byte("attachment1")},
			{Filename: "attachment2.txt", Data: []byte("attachment2")},
		},
	}

	emailWithoutAttachments := fetcher.Email{
		From:    "test@example.com",
		Subject: "Test Email without Attachments",
		Body:    "This is the body.",
		Date:    time.Now(),
	}

	t.Run("Focus on attachments", func(t *testing.T) {
		emailView := NewEmailView(emailWithAttachments, 0, 80, 24, MailboxInbox, false)
		if emailView.focusOnAttachments {
			t.Error("focusOnAttachments should be initially false")
		}

		// Tab to focus on attachments
		model, _ := emailView.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		emailView = model.(*EmailView)

		if !emailView.focusOnAttachments {
			t.Error("focusOnAttachments should be true after tabbing")
		}

		// Tab back to body
		model, _ = emailView.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		emailView = model.(*EmailView)
		if emailView.focusOnAttachments {
			t.Error("focusOnAttachments should be false after tabbing again")
		}
	})

	t.Run("No focus on attachments when there are none", func(t *testing.T) {
		emailView := NewEmailView(emailWithoutAttachments, 0, 80, 24, MailboxInbox, false)
		if emailView.focusOnAttachments {
			t.Error("focusOnAttachments should be initially false")
		}
		// Tab
		model, _ := emailView.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		emailView = model.(*EmailView)
		if emailView.focusOnAttachments {
			t.Error("focusOnAttachments should remain false when there are no attachments")
		}
	})

	t.Run("Navigate attachments", func(t *testing.T) {
		emailView := NewEmailView(emailWithAttachments, 0, 80, 24, MailboxInbox, false)
		// Focus on attachments
		model, _ := emailView.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		emailView = model.(*EmailView)

		if emailView.attachmentCursor != 0 {
			t.Errorf("Initial attachmentCursor should be 0, got %d", emailView.attachmentCursor)
		}

		// Move down
		model, _ = emailView.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		emailView = model.(*EmailView)
		if emailView.attachmentCursor != 1 {
			t.Errorf("After one down arrow, attachmentCursor should be 1, got %d", emailView.attachmentCursor)
		}

		// Move down again (should wrap to the first attachment)
		model, _ = emailView.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		emailView = model.(*EmailView)
		if emailView.attachmentCursor != 0 {
			t.Errorf("attachmentCursor should wrap to the start of the list, got %d", emailView.attachmentCursor)
		}

		// Move up (should wrap to the last attachment)
		model, _ = emailView.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		emailView = model.(*EmailView)
		if emailView.attachmentCursor != 1 {
			t.Errorf("After one up arrow from the first item, attachmentCursor should be 1, got %d", emailView.attachmentCursor)
		}
	})

	t.Run("Attachment navigation does not scroll body", func(t *testing.T) {
		emailView := NewEmailView(emailWithAttachments, 0, 80, 24, MailboxInbox, false)
		emailView.viewport.SetHeight(2)
		emailView.viewport.SetContent("line 1\nline 2\nline 3\nline 4\nline 5\n")
		emailView.viewport.SetYOffset(1)

		model, _ := emailView.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		emailView = model.(*EmailView)
		if !emailView.focusOnAttachments {
			t.Fatal("focusOnAttachments should be true after tabbing")
		}

		before := emailView.viewport.YOffset()
		model, _ = emailView.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		emailView = model.(*EmailView)
		if got := emailView.viewport.YOffset(); got != before {
			t.Fatalf("attachment navigation should not scroll the email body, got offset %d want %d", got, before)
		}
	})

	t.Run("Download attachment", func(t *testing.T) {
		emailView := NewEmailView(emailWithAttachments, 0, 80, 24, MailboxInbox, false)
		// Focus on attachments
		model, _ := emailView.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		emailView = model.(*EmailView)

		// Move to the second attachment
		model, _ = emailView.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		emailView = model.(*EmailView)

		// Press enter
		_, cmd := emailView.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd == nil {
			t.Fatal("Expected a command, but got nil")
		}

		msg := cmd()
		downloadMsg, ok := msg.(DownloadAttachmentMsg)
		if !ok {
			t.Fatalf("Expected a DownloadAttachmentMsg, but got %T", msg)
		}
		if downloadMsg.Filename != "attachment2.txt" {
			t.Errorf("Expected to download 'attachment2.txt', but got '%s'", downloadMsg.Filename)
		}
		if downloadMsg.Mailbox != MailboxInbox {
			t.Errorf("Expected mailbox to be MailboxInbox, got %s", downloadMsg.Mailbox)
		}
	})

	t.Run("Reply to email", func(t *testing.T) {
		emailView := NewEmailView(emailWithAttachments, 0, 80, 24, MailboxInbox, false)

		_, cmd := emailView.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
		if cmd == nil {
			t.Fatal("Expected a command, but got nil")
		}

		msg := cmd()
		replyMsg, ok := msg.(ReplyToEmailMsg)
		if !ok {
			t.Fatalf("Expected a ReplyToEmailMsg, but got %T", msg)
		}
		if replyMsg.Email.Subject != emailWithAttachments.Subject {
			t.Errorf("Expected reply to have subject '%s', but got '%s'", emailWithAttachments.Subject, replyMsg.Email.Subject)
		}
	})
}

func TestFormatEmailHeaderAlignsFields(t *testing.T) {
	email := fetcher.Email{
		To:      []string{"alice@example.com", "bob@example.com"},
		From:    "sender@example.com",
		Subject: "Hello",
	}

	got := formatEmailHeader(email, 80, "")
	want := "To:      alice@example.com, bob@example.com\n" +
		"From:    sender@example.com\n" +
		"Subject: Hello"
	if got != want {
		t.Fatalf("formatEmailHeader() =\n%q\nwant\n%q", got, want)
	}
}

func TestFormatEmailHeaderWrapsLongToWithAlignedContinuation(t *testing.T) {
	email := fetcher.Email{
		To: []string{
			"alpha@example.com", "bravo@example.com", "charlie@example.com",
			"delta@example.com", "echo@example.com", "foxtrot@example.com",
		},
		From:    "sender@example.com",
		Subject: "A long recipient list",
	}
	const width = 40

	got := formatEmailHeader(email, width, "")
	lines := strings.Split(got, "\n")
	if len(lines) <= 3 {
		t.Fatalf("expected the To field to wrap, got %d lines:\n%s", len(lines), got)
	}

	indent := strings.Repeat(" ", emailHeaderLabelWidth)
	sawContinuation := false
	for _, line := range lines {
		if lipgloss.Width(line) > width {
			t.Fatalf("header line exceeds width %d: %q", width, line)
		}
		if strings.HasPrefix(line, indent) {
			sawContinuation = true
		}
	}
	if !sawContinuation {
		t.Fatalf("expected wrapped continuation lines aligned to the value column:\n%s", got)
	}
}

func TestFormatEmailHeaderWrapsChineseSubject(t *testing.T) {
	email := fetcher.Email{
		To:      []string{"alice@example.com"},
		From:    "sender@example.com",
		Subject: strings.Repeat("中文主题", 20),
	}
	const width = 30

	got := formatEmailHeader(email, width, "")
	for _, line := range strings.Split(got, "\n") {
		if lipgloss.Width(line) > width {
			t.Fatalf("header line exceeds width %d: %q", width, line)
		}
	}
}

func TestRenderHeaderFitsViewportWidth(t *testing.T) {
	email := fetcher.Email{
		To:      []string{"alpha@example.com", "bravo@example.com", "charlie@example.com"},
		From:    "sender@example.com",
		Subject: strings.Repeat("中文主题", 10),
	}
	const width = 40

	header := NewEmailView(email, 0, width, 20, MailboxInbox, false).renderHeader()
	for _, line := range strings.Split(header, "\n") {
		if lipgloss.Width(line) > width {
			t.Fatalf("rendered header line exceeds viewport width %d: %q", width, line)
		}
	}
}
