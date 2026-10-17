package support

import (
	"strings"
	"testing"
)

func TestPrivateRecoveryKeyAndBoundedPlainText(t *testing.T) {
	if _, err := KeyDigest("short"); err == nil {
		t.Fatal("weak key accepted")
	}
	a, err := KeyDigest(strings.Repeat("a", 64))
	if err != nil || a == strings.Repeat("a", 64) {
		t.Fatal("recovery secret not hashed")
	}
	for _, s := range []string{"", "  ", strings.Repeat("x", 5001), string([]byte{0xff})} {
		if _, err := MessageText(s); err == nil {
			t.Fatal("invalid message accepted")
		}
	}
	if got, err := MessageText(" <script>text</script> "); err != nil || got != "<script>text</script>" {
		t.Fatal("plain text not retained")
	}
}

func TestTicketIDsRequireCanonicalULID(t *testing.T) {
	for _, id := range []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV", "00000000000000000000000000", "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"} {
		if !ValidTicketID(id) {
			t.Fatalf("valid ULID rejected: %s", id)
		}
	}
	for _, id := range []string{"1", "123456", "6de631a7-a5be-438e-9200-2650a4e8891d", "8ZZZZZZZZZZZZZZZZZZZZZZZZZ", "01ARZ3NDEKTSV4RRFFQ69G5FAI", "01arz3ndektsv4rrffq69g5fav"} {
		if ValidTicketID(id) {
			t.Fatalf("invalid ticket ID accepted: %s", id)
		}
	}
}
