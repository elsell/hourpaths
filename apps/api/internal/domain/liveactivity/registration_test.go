package liveactivity

import (
	"testing"
	"time"
)

func TestRegistrationSkipsPastBoundariesWithoutCarryingOldDeadline(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	registration := Registration{ID: "activity", OwnerUserID: "alice", AccountLease: "lease", Token: "abab", ExpiresAt: now.Add(2 * time.Hour), Boundaries: []time.Time{now.Add(-time.Minute), now.Add(time.Hour)}}
	if !registration.Valid(now) {
		t.Fatal("valid delayed registration rejected")
	}
	if got := registration.NextDeadline(now); !got.Equal(now.Add(time.Hour)) {
		t.Fatalf("deadline=%s", got)
	}
	if got := registration.NextDeadline(now.Add(time.Hour)); !got.Equal(registration.ExpiresAt) {
		t.Fatalf("deadline after boundary=%s", got)
	}
	for name, mutate := range map[string]func(*Registration){
		"expired":          func(r *Registration) { r.ExpiresAt = now },
		"unbounded":        func(r *Registration) { r.ExpiresAt = now.Add(9 * time.Hour) },
		"unordered":        func(r *Registration) { r.Boundaries = []time.Time{now.Add(time.Hour), now} },
		"duplicate":        func(r *Registration) { r.Boundaries = []time.Time{now, now} },
		"outside activity": func(r *Registration) { r.Boundaries = []time.Time{now.Add(3 * time.Hour)} },
		"too many":         func(r *Registration) { r.Boundaries = make([]time.Time, 65) },
		"invalid token":    func(r *Registration) { r.Token = "../another-device" },
		"missing owner":    func(r *Registration) { r.OwnerUserID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := registration
			mutate(&changed)
			if changed.Valid(now) {
				t.Fatal("invalid registration accepted")
			}
		})
	}
}
