package path

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

type ownershipTransferReservation struct {
	Version         int       `json:"version"`
	Domain          string    `json:"domain"`
	CreatorUserID   string    `json:"creatorUserId"`
	PathID          domain.ID `json:"pathId"`
	RecipientUserID string    `json:"recipientUserId"`
	ReviewedAt      time.Time `json:"reviewedAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

func encodeOwnershipTransferReservation(key []byte, value ownershipTransferReservation) (string, error) {
	if len(key) < 32 || !validOwnershipTransferReservation(value) {
		return "", errInvalidOwnershipTransferDependencies
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func decodeOwnershipTransferReservation(key []byte, token string) (ownershipTransferReservation, error) {
	parts := strings.Split(token, ".")
	if len(key) < 32 || len(parts) != 2 {
		return ownershipTransferReservation{}, errInvalidOwnershipTransferDependencies
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ownershipTransferReservation{}, errInvalidOwnershipTransferDependencies
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ownershipTransferReservation{}, errInvalidOwnershipTransferDependencies
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return ownershipTransferReservation{}, errInvalidOwnershipTransferDependencies
	}
	var value ownershipTransferReservation
	if err := json.Unmarshal(payload, &value); err != nil || !validOwnershipTransferReservation(value) {
		return ownershipTransferReservation{}, errInvalidOwnershipTransferDependencies
	}
	return value, nil
}

func validOwnershipTransferReservation(value ownershipTransferReservation) bool {
	return value.Version == 1 && value.Domain == "path-ownership-transfer-review" &&
		strings.TrimSpace(value.CreatorUserID) == value.CreatorUserID && value.CreatorUserID != "" &&
		value.PathID != "" && strings.TrimSpace(value.RecipientUserID) == value.RecipientUserID && value.RecipientUserID != "" &&
		value.CreatorUserID != value.RecipientUserID && !value.ReviewedAt.IsZero() && value.ExpiresAt.After(value.ReviewedAt)
}
