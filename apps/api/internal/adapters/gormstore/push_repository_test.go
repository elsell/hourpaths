package gormstore

import (
	"bytes"
	"testing"

	"gorm.io/gorm"
)

func TestPushTokenProtectionUsesRandomCiphertextAndOwnerInstallationAAD(t *testing.T) {
	repository, err := NewPushRepository(&gorm.DB{}, bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	const token = "ExponentPushToken[secret-value]"
	first, firstNonce, firstHash, err := repository.protectToken("user-1", "installation-1", token)
	if err != nil {
		t.Fatal(err)
	}
	second, secondNonce, secondHash, err := repository.protectToken("user-1", "installation-1", token)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(first, []byte(token)) {
		t.Fatal("ciphertext contains the plaintext token")
	}
	if bytes.Equal(first, second) || bytes.Equal(firstNonce, secondNonce) {
		t.Fatal("token encryption must use a fresh nonce")
	}
	if !bytes.Equal(firstHash, secondHash) {
		t.Fatal("keyed lookup hash must be stable")
	}
	revealed, err := repository.revealToken("user-1", "installation-1", first, firstNonce)
	if err != nil || revealed != token {
		t.Fatalf("revealed token = %q, err = %v", revealed, err)
	}
	if _, err := repository.revealToken("user-2", "installation-1", first, firstNonce); err == nil {
		t.Fatal("ciphertext must not decrypt for another owner")
	}
	if _, err := repository.revealToken("user-1", "installation-2", first, firstNonce); err == nil {
		t.Fatal("ciphertext must not decrypt for another installation")
	}
}

func TestNewPushRepositoryRejectsMissingOrShortKey(t *testing.T) {
	for _, key := range [][]byte{nil, bytes.Repeat([]byte{1}, 31)} {
		if _, err := NewPushRepository(&gorm.DB{}, key); err == nil {
			t.Fatal("short push encryption key was accepted")
		}
	}
}
