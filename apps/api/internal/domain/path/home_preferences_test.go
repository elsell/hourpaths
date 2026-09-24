package path

import (
	"testing"
	"time"
)

func TestHomePreferencesValidateOrderAndUniqueMembershipSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	valid := HomePreferences{OrderMethod: HomeOrderManual, Revision: 1, PinnedPathIDs: []ID{"path-2"}, ManualPathIDs: []ID{"path-1", "path-2"}, UpdatedAt: now}
	if !valid.Valid() {
		t.Fatal("valid Home preferences were rejected")
	}
	for name, mutate := range map[string]func(*HomePreferences){
		"unknown order":    func(value *HomePreferences) { value.OrderMethod = "popular" },
		"duplicate manual": func(value *HomePreferences) { value.ManualPathIDs = []ID{"path-1", "path-1"} },
		"duplicate pin":    func(value *HomePreferences) { value.PinnedPathIDs = []ID{"path-2", "path-2"} },
		"foreign pin":      func(value *HomePreferences) { value.PinnedPathIDs = []ID{"path-3"} },
		"missing update":   func(value *HomePreferences) { value.UpdatedAt = time.Time{} },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if candidate.Valid() {
				t.Fatal("invalid Home preferences were accepted")
			}
		})
	}
	if defaults := (HomePreferences{OrderMethod: HomeOrderRecent}); !defaults.Valid() {
		t.Fatal("zero-revision default preferences were rejected")
	}
}
