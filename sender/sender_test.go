package sender

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/mail"
	"strings"
	"testing"

	"github.com/floatpane/matcha/config"
	"github.com/floatpane/matcha/internal/mailheader"
)

type failingReader struct{}

func (failingReader) Read(p []byte) (int, error) {
	return 0, errors.New("simulated crypto/rand failure")
}

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("simulated write failure")
}

func TestWriteQuotedPrintablePropagatesFlushError(t *testing.T) {
	err := writeQuotedPrintable(failingWriter{}, "hello")
	if err == nil {
		t.Fatal("expected quoted-printable write error, got nil")
	}
	if !strings.Contains(err.Error(), "quoted-printable encoding failed") {
		t.Fatalf("expected quoted-printable context, got %v", err)
	}
}

// TestSMIMEOuterBoundary_RandFailure ensures that a crypto/rand failure surfaces
// as an error rather than silently producing a predictable, time-based
// boundary that an attacker could collide with (issue #1127).
func TestSMIMEOuterBoundary_RandFailure(t *testing.T) {
	orig := randReader
	t.Cleanup(func() { randReader = orig })
	randReader = failingReader{}

	got, err := smimeOuterBoundary()
	if err == nil {
		t.Fatalf("expected error when crypto/rand fails, got boundary %q", got)
	}
	if got != "" {
		t.Errorf("expected empty boundary on error, got %q", got)
	}
}

// TestSMIMEOuterBoundary_Success ensures the happy path returns a non-empty,
// random-looking boundary with the expected prefix.
func TestSMIMEOuterBoundary_Success(t *testing.T) {
	b1, err := smimeOuterBoundary()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(b1, "signed-") {
		t.Errorf("boundary should start with 'signed-', got %q", b1)
	}
	// 12 random bytes => 24 hex chars; total length 7 + 24 = 31.
	if len(b1) != len("signed-")+24 {
		t.Errorf("unexpected boundary length: got %d (%q)", len(b1), b1)
	}
	b2, err := smimeOuterBoundary()
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if b1 == b2 {
		t.Errorf("two consecutive boundaries should differ, both got %q", b1)
	}
}

// Ensure io is referenced even if a future refactor removes it indirectly.
var _ io.Reader = failingReader{}

func TestSMTPHelloHostname(t *testing.T) {
	orig := osHostname
	t.Cleanup(func() { osHostname = orig })

	osHostname = func() (string, error) { return "mail.example.com", nil }
	if got := smtpHelloHostname(); got != "mail.example.com" {
		t.Fatalf("expected hostname, got %q", got)
	}

	osHostname = func() (string, error) { return "", nil }
	if got := smtpHelloHostname(); got != "localhost" {
		t.Fatalf("expected localhost fallback for empty hostname, got %q", got)
	}

	osHostname = func() (string, error) { return "ignored", errors.New("hostname unavailable") }
	if got := smtpHelloHostname(); got != "localhost" {
		t.Fatalf("expected localhost fallback on error, got %q", got)
	}
}

// TestGenerateMessageID ensures the Message-ID has the correct format.
func TestGenerateMessageID(t *testing.T) {
	from := "test@example.com"
	msgID := generateMessageID(from)

	// Check if the message ID is enclosed in angle brackets.
	if !strings.HasPrefix(msgID, "<") || !strings.HasSuffix(msgID, ">") {
		t.Errorf("Message-ID should be enclosed in angle brackets, got %s", msgID)
	}

	// Check if the 'from' address is part of the message ID.
	if !strings.Contains(msgID, from) {
		t.Errorf("Message-ID should contain the from address, got %s", msgID)
	}

	// The original check was too simple and failed because the 'from' address itself contains an '@'.
	// A Message-ID is generally <unique-part@domain>. The current implementation uses the full 'from' address as the domain part.
	// This revised check validates that structure correctly.
	unwrappedID := strings.Trim(msgID, "<>")

	// Ensure there's at least one '@' symbol.
	if !strings.Contains(unwrappedID, "@") {
		t.Errorf("Message-ID should contain an '@' symbol, got %s", msgID)
	}

	// Check that the ID ends with the full 'from' address, preceded by an '@'.
	// This confirms the structure is <random_part>@<from_address>.
	expectedSuffix := "@" + from
	if !strings.HasSuffix(unwrappedID, expectedSuffix) {
		t.Errorf("Message-ID should end with '@' + from address. Got %s, expected suffix %s", unwrappedID, expectedSuffix)
	}

	// Check that the part before the suffix is not empty.
	randomPart := strings.TrimSuffix(unwrappedID, expectedSuffix)
	if randomPart == "" {
		t.Errorf("Message-ID has an empty random part, got %s", msgID)
	}
}

func TestBuildEmailEncodesChineseHeaders(t *testing.T) {
	account := &config.Account{Name: "张三", Email: "zhangsan@example.com"}
	subject := strings.Repeat("会议通知：项目进度。", 6)
	raw, err := BuildEmail(account, []string{"李四 <lisi@example.com>"}, nil, nil, subject, "你好，世界", "", nil, nil, "", nil, false, false, false, false)
	if err != nil {
		t.Fatalf("BuildEmail() error: %v", err)
	}

	headerEnd := bytes.Index(raw, []byte("\r\n\r\n"))
	if headerEnd < 0 {
		t.Fatal("message has no header/body separator")
	}
	if !mailheader.IsASCII(string(raw[:headerEnd])) {
		t.Fatalf("message headers contain raw non-ASCII bytes:\n%s", raw[:headerEnd])
	}

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadMessage() error: %v", err)
	}

	decodedSubject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil {
		t.Fatalf("DecodeHeader(Subject) error: %v", err)
	}
	if decodedSubject != subject {
		t.Fatalf("decoded subject = %q, want %q", decodedSubject, subject)
	}

	from, err := msg.Header.AddressList("From")
	if err != nil {
		t.Fatalf("AddressList(From) error: %v", err)
	}
	if len(from) != 1 || from[0].Name != "张三" || from[0].Address != "zhangsan@example.com" {
		t.Fatalf("unexpected From header: %+v", from)
	}

	to, err := msg.Header.AddressList("To")
	if err != nil {
		t.Fatalf("AddressList(To) error: %v", err)
	}
	if len(to) != 1 || to[0].Name != "李四" || to[0].Address != "lisi@example.com" {
		t.Fatalf("unexpected To header: %+v", to)
	}
}

func TestBuildEmailEncodesChineseAttachmentName(t *testing.T) {
	account := &config.Account{Email: "zhangsan@example.com"}
	raw, err := BuildEmail(account, []string{"lisi@example.com"}, nil, nil, "subject", "body", "",
		nil, map[string][]byte{"报告.pdf": []byte("pdf")}, "", nil, false, false, false, false)
	if err != nil {
		t.Fatalf("BuildEmail() error: %v", err)
	}
	if !bytes.Contains(raw, []byte("filename*=utf-8''")) {
		t.Fatalf("attachment filename was not RFC 2231 encoded:\n%s", raw)
	}
}

func TestExtractBareEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bare email",
			input:    "user@example.com",
			expected: "user@example.com",
		},
		{
			name:     "formatted with name",
			input:    "John Doe <user@example.com>",
			expected: "user@example.com",
		},
		{
			name:     "formatted with quoted name",
			input:    "\"John Doe\" <user@example.com>",
			expected: "user@example.com",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "invalid format returns as-is",
			input:    "not-an-email",
			expected: "not-an-email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBareEmail(tt.input)
			if got != tt.expected {
				t.Errorf("extractBareEmail(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
