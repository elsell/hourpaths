package sociallock

import (
	"strconv"
	"strings"
)

const interactionOwnerNamespace = "social-interaction-owner"
const pathAudienceNamespace = "social-path-audience"

// Key preserves unambiguous advisory-lock identity across social writers and
// Path membership lifecycle operations.
func Key(namespace string, parts ...string) string {
	var key strings.Builder
	for _, part := range append([]string{namespace}, parts...) {
		key.WriteString(strconv.Itoa(len(part)))
		key.WriteByte(':')
		key.WriteString(part)
	}
	return key.String()
}

func InteractionOwnerKey(owner string) string { return Key(interactionOwnerNamespace, owner) }

func PathAudienceKey(pathID string) string { return Key(pathAudienceNamespace, pathID) }

// ProfilePathsKey serializes privacy with changes to an owner's Path audience.
// Acquire before individual Path audience locks and row locks.
func ProfilePathsKey(owner string) string { return Key("profile-paths", owner) }
