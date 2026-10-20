// Package mailheader provides helpers for encoding email header values so
// that non-ASCII text (e.g. Chinese) survives common mail clients.
package mailheader

import (
	"encoding/base64"
	"fmt"
	"mime"
	"net/mail"
	"strings"
)

// IsASCII reports whether s contains only ASCII bytes.
func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// EncodeValue encodes a non-ASCII header value using RFC 2047 encoded-words.
// Long values are split into multiple encoded-words, folded onto continuation
// lines so that no encoded-word exceeds the 75 character limit. Every chunk is
// encoded (even ASCII-only chunks) so that the whitespace inserted by folding
// is ignored when the value is decoded.
func EncodeValue(value string) string {
	if IsASCII(value) {
		return value
	}

	const (
		wordPrefix        = "=?UTF-8?B?"
		wordSuffix        = "?="
		maxEncodedWordLen = 75
	)

	var words []string
	var chunk []rune
	flush := func() {
		if len(chunk) == 0 {
			return
		}
		words = append(words, wordPrefix+base64.StdEncoding.EncodeToString([]byte(string(chunk)))+wordSuffix)
		chunk = chunk[:0]
	}

	for _, r := range value {
		chunk = append(chunk, r)
		encodedLen := len(wordPrefix) + base64.StdEncoding.EncodedLen(len(string(chunk))) + len(wordSuffix)
		if encodedLen > maxEncodedWordLen {
			chunk = chunk[:len(chunk)-1]
			flush()
			chunk = append(chunk, r)
		}
	}
	flush()

	return strings.Join(words, "\r\n ")
}

// FormatAddress formats a single address, RFC 2047-encoding the display name
// when it contains non-ASCII characters. Unparseable values are encoded as a
// whole so that they are not sent as raw 8-bit header bytes.
func FormatAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	parsed, err := mail.ParseAddress(addr)
	if err != nil {
		return EncodeValue(addr)
	}
	return parsed.String()
}

// FormatAddressList formats a list of addresses separated by commas, encoding
// any non-ASCII display names.
func FormatAddressList(addrs []string) string {
	formatted := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if formattedAddr := FormatAddress(addr); formattedAddr != "" {
			formatted = append(formatted, formattedAddr)
		}
	}
	return strings.Join(formatted, ", ")
}

// ContentDisposition builds a Content-Disposition value, using RFC 2231
// encoding for non-ASCII filenames.
func ContentDisposition(disposition, filename string) string {
	if value := mime.FormatMediaType(disposition, map[string]string{"filename": filename}); value != "" {
		return value
	}
	return fmt.Sprintf("%s; filename=%q", disposition, filename)
}
