package support

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalid = errors.New("invalid support input")

const Lifetime = 30 * 24 * time.Hour
const AdminResource = "hourpaths-platform-admin"

type Message struct {
	ID, Body  string
	Operator  bool
	CreatedAt time.Time
}
type Conversation struct {
	ID, Subject, KeyHash string
	CreatedAt, ExpiresAt time.Time
	Messages             []Message
}
type Notice struct {
	ID, RequestID string
	CreatedAt     time.Time
	Read          bool
}

func KeyDigest(secret string) (string, error) {
	if len(secret) != 64 {
		return "", ErrInvalid
	}
	b, e := hex.DecodeString(secret)
	if e != nil || len(b) != 32 {
		return "", ErrInvalid
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
func MessageText(value string) (string, error) { return text(value, 5000) }
func SubjectText(value string) (string, error) { return text(value, 160) }
func text(value string, maximum int) (string, error) {
	if !utf8.ValidString(value) {
		return "", ErrInvalid
	}
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > maximum || strings.ContainsRune(value, 0) {
		return "", ErrInvalid
	}
	return value, nil
}

// ValidTicketID accepts the canonical uppercase ULID representation, including
// the 128-bit upper bound. The identifier conveys no authorization by itself.
func ValidTicketID(id string) bool {
	if len(id) != 26 || id[0] < '0' || id[0] > '7' {
		return false
	}
	for _, c := range id {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", c) {
			return false
		}
	}
	return true
}
