package identity

import (
	"errors"
	"strings"
)

// DuplicateEmailRecoveryDecline suppresses one recovery hint for one
// provisional account and normalized provider email. Email correspondence is
// never an account identity or ownership proof.
type DuplicateEmailRecoveryDecline struct {
	ProvisionalUserID string
	NormalizedEmail   string
}

func NewDuplicateEmailRecoveryDecline(provisionalUserID, email string) (DuplicateEmailRecoveryDecline, error) {
	decline := DuplicateEmailRecoveryDecline{
		ProvisionalUserID: provisionalUserID,
		NormalizedEmail:   NormalizeEmail(email),
	}
	if decline.ProvisionalUserID == "" || decline.NormalizedEmail == "" {
		return DuplicateEmailRecoveryDecline{}, errors.New("duplicate email recovery decline requires owner and email")
	}
	return decline, nil
}

func (d DuplicateEmailRecoveryDecline) AppliesTo(provisionalUserID, email string) bool {
	return d.ProvisionalUserID == provisionalUserID && d.NormalizedEmail == NormalizeEmail(email)
}

func NormalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
