package stats

import (
	"context"
	"errors"
	platform "github.com/elsell/hour-paths/apps/api/internal/app"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/stats"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
)

type pathControlled struct {
	controlled
	reads             int
	path, participant string
	readErr           error
}

func (f *pathControlled) ReadPath(_ context.Context, viewer, path, participant string) (Snapshot, error) {
	f.reads++
	f.actor = viewer
	f.path = path
	f.participant = participant
	return f.snapshot, f.readErr
}
func TestPathStatisticsAuthorizationAndParticipantScope(t *testing.T) {
	for _, name := range []string{"own", "other participant", "unauthenticated", "denied", "policy outage", "audit outage", "nonparticipant"} {
		t.Run(name, func(t *testing.T) {
			f := &pathControlled{}
			f.snapshot = Snapshot{TimeZone: "UTC", FirstDayOfWeek: 1, Records: []domain.Record{}}
			selected := ""
			switch name {
			case "other participant":
				selected = "participant"
			case "unauthenticated":
				f.authErr = platform.ErrUnauthenticated
			case "denied":
				f.denied = true
			case "policy outage":
				f.errorPolicy = errors.New("policy unavailable")
			case "audit outage":
				f.errorAudit = errors.New("audit unavailable")
			case "nonparticipant":
				f.readErr = ports.ErrNotFound
			}
			s := Service{Auth: f, PathRepository: f, Authorizer: f, Audits: f, AuditRateLimiter: f, Clock: f}
			got, err := s.GetPath(context.Background(), "session", "path", selected)
			if name == "own" || name == "other participant" {
				want := "viewer"
				if selected != "" {
					want = selected
				}
				if err != nil || f.actor != "viewer" || f.path != "path" || f.participant != want || len(got.Calendar) != 365 || len(f.events) != 1 {
					t.Fatalf("scope/result: %+v %+v %v", f, got, err)
				}
			} else {
				if name == "nonparticipant" && len(f.events) != 1 {
					t.Fatal("visibility denial was not audited")
				}
				if err == nil || len(got.Calendar) != 0 {
					t.Fatalf("failed open: %+v %v", got, err)
				}
				if (name == "unauthenticated" || name == "denied" || name == "policy outage") && f.reads != 0 {
					t.Fatal("read before authorization")
				}
			}
		})
	}
}
