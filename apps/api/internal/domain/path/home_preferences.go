package path

import "time"

type HomeOrderMethod string

const (
	HomeOrderRecent       HomeOrderMethod = "recent"
	HomeOrderAlphabetical HomeOrderMethod = "alphabetical"
	HomeOrderManual       HomeOrderMethod = "manual"
)

func (method HomeOrderMethod) Valid() bool {
	return method == HomeOrderRecent || method == HomeOrderAlphabetical || method == HomeOrderManual
}

type HomePreferences struct {
	OrderMethod   HomeOrderMethod
	Revision      int64
	PinnedPathIDs []ID
	ManualPathIDs []ID
	UpdatedAt     time.Time
}

func (preferences HomePreferences) Valid() bool {
	if !preferences.OrderMethod.Valid() || preferences.Revision < 0 {
		return false
	}
	manual := make(map[ID]struct{}, len(preferences.ManualPathIDs))
	for _, id := range preferences.ManualPathIDs {
		if id == "" {
			return false
		}
		if _, duplicate := manual[id]; duplicate {
			return false
		}
		manual[id] = struct{}{}
	}
	pinned := make(map[ID]struct{}, len(preferences.PinnedPathIDs))
	for _, id := range preferences.PinnedPathIDs {
		if _, exists := manual[id]; !exists {
			return false
		}
		if _, duplicate := pinned[id]; duplicate {
			return false
		}
		pinned[id] = struct{}{}
	}
	return preferences.Revision == 0 || !preferences.UpdatedAt.IsZero()
}
