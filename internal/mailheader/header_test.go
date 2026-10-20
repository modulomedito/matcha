package mailheader

import (
	"mime"
	"net/mail"
	"strings"
	"testing"
)

func TestEncodeValueASCIIUnchanged(t *testing.T) {
	const value = "Hello, World!"
	if got := EncodeValue(value); got != value {
		t.Fatalf("EncodeValue(%q) = %q, want unchanged", value, got)
	}
}

func TestEncodeValueChinese(t *testing.T) {
	const value = "会议通知：项目进度"
	encoded := EncodeValue(value)
	if !IsASCII(encoded) {
		t.Fatalf("encoded value should be ASCII, got %q", encoded)
	}
	decoded, err := new(mime.WordDecoder).DecodeHeader(encoded)
	if err != nil {
		t.Fatalf("DecodeHeader() error: %v", err)
	}
	if decoded != value {
		t.Fatalf("decoded value = %q, want %q", decoded, value)
	}
}

func TestEncodeValueFoldsLongValue(t *testing.T) {
	value := strings.Repeat("这是一封很长的中文邮件主题。", 10)
	encoded := EncodeValue(value)

	for _, line := range strings.Split(encoded, "\r\n") {
		if len(line) > 998 {
			t.Fatalf("header line too long: %d bytes", len(line))
		}
		for _, word := range strings.Fields(line) {
			if len(word) > 75 {
				t.Fatalf("encoded-word too long (%d bytes): %q", len(word), word)
			}
		}
	}

	decoded, err := new(mime.WordDecoder).DecodeHeader(encoded)
	if err != nil {
		t.Fatalf("DecodeHeader() error: %v", err)
	}
	if decoded != value {
		t.Fatalf("decoded value = %q, want %q", decoded, value)
	}
}

func TestEncodeValueMixedTextRoundTrip(t *testing.T) {
	value := "Re: 你好 " + strings.Repeat("abcdefghij", 20) + " 再见"
	encoded := EncodeValue(value)

	decoded, err := new(mime.WordDecoder).DecodeHeader(encoded)
	if err != nil {
		t.Fatalf("DecodeHeader() error: %v", err)
	}
	if decoded != value {
		t.Fatalf("decoded value = %q, want %q", decoded, value)
	}
}

func TestFormatAddressListChinese(t *testing.T) {
	got := FormatAddressList([]string{"张三 <zhangsan@example.com>", "lisi@example.com"})
	if !IsASCII(got) {
		t.Fatalf("formatted address list contains non-ASCII: %q", got)
	}
	parsed, err := mail.ParseAddressList(got)
	if err != nil {
		t.Fatalf("ParseAddressList() error: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("got %d addresses, want 2", len(parsed))
	}
	if parsed[0].Name != "张三" {
		t.Fatalf("decoded display name = %q, want 张三", parsed[0].Name)
	}
	if parsed[1].Address != "lisi@example.com" {
		t.Fatalf("decoded address = %q, want lisi@example.com", parsed[1].Address)
	}
}

func TestContentDispositionChineseFilename(t *testing.T) {
	got := ContentDisposition("attachment", "报告.pdf")
	if !strings.Contains(got, "filename*=utf-8''") {
		t.Fatalf("expected RFC 2231 filename, got %q", got)
	}
	_, params, err := mime.ParseMediaType(got)
	if err != nil {
		t.Fatalf("ParseMediaType() error: %v", err)
	}
	if params["filename"] != "报告.pdf" {
		t.Fatalf("decoded filename = %q, want 报告.pdf", params["filename"])
	}
}
