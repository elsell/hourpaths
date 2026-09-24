package social

import (
	"strings"
	"time"
)

// BlockTarget is the minimum identity retained for a block control. It never
// carries profile visibility, description, email, or provider identity.
type BlockTarget struct {
	UserID, Username, DisplayName string
}

func (target BlockTarget) Valid() bool {
	return strings.TrimSpace(target.UserID) == target.UserID && target.UserID != "" &&
		strings.TrimSpace(target.Username) == target.Username && target.Username != "" &&
		strings.TrimSpace(target.DisplayName) == target.DisplayName && target.DisplayName != ""
}

type SharedPath struct{ ID, Name string }

func (path SharedPath) Valid() bool {
	return strings.TrimSpace(path.ID) == path.ID && path.ID != "" &&
		strings.TrimSpace(path.Name) == path.Name && path.Name != ""
}

type BlockedAccount struct {
	Target    BlockTarget
	BlockedAt time.Time
}

func (account BlockedAccount) Valid() bool {
	return account.Target.Valid() && !account.BlockedAt.IsZero() && account.BlockedAt.Location() == time.UTC
}
