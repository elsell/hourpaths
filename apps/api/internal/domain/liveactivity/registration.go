package liveactivity

import (
	"encoding/hex"
	"time"
)

const MaximumLifetime = 8 * time.Hour
const MaximumBoundaries = 64

type Registration struct {
	ID, OwnerUserID, AccountLease, Token string
	ExpiresAt                            time.Time
	Boundaries                           []time.Time
}

func ValidID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func (r Registration) Valid(now time.Time) bool {
	if now.IsZero() || !ValidID(r.ID) || !ValidID(r.OwnerUserID) || !ValidID(r.AccountLease) || len(r.Token) < 2 || len(r.Token) > 2048 || !r.ExpiresAt.After(now) || r.ExpiresAt.After(now.Add(MaximumLifetime)) || len(r.Boundaries) > MaximumBoundaries {
		return false
	}
	if _, err := hex.DecodeString(r.Token); err != nil {
		return false
	}
	var previous time.Time
	for _, boundary := range r.Boundaries {
		if boundary.IsZero() || boundary.After(r.ExpiresAt) || boundary.Before(now.Add(-MaximumLifetime)) || !previous.IsZero() && !boundary.After(previous) {
			return false
		}
		previous = boundary
	}
	return true
}

// Missing a push never reuses a prior period. A wake always sets the next future
// deadline, and the native cache chooses progress from the device's own ledger.
func (r Registration) NextDeadline(now time.Time) time.Time {
	for _, boundary := range r.Boundaries {
		if boundary.After(now) {
			return boundary
		}
	}
	return r.ExpiresAt
}
