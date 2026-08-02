package auth

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

const (
	minimumPasswordLength = 12
	maximumPasswordLength = 128
	maximumPasswordBytes  = 256
)

func normalizeEmail(value string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(value))
	if len(email) < 3 || len(email) > 320 || strings.ContainsAny(email, "\r\n\t ") {
		return "", false
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, "@") {
		return "", false
	}
	return email, true
}

func validPassword(value string) bool {
	characters := utf8.RuneCountInString(value)
	return utf8.ValidString(value) && characters >= minimumPasswordLength && characters <= maximumPasswordLength && len(value) <= maximumPasswordBytes
}
