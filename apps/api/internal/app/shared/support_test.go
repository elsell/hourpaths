package shared

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

func TestAuditFactoryPreservesInjectedTimeAndCorrelation(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	event := NewAuditEvent(WithCorrelationID(context.Background(), "request"), fakeClock{now}, "owner", "actor", audit.ResourceCreated, "habit", "id", audit.Succeeded)
	if !event.Valid() || event.CorrelationID != "request" || !event.OccurredAt.Equal(now) {
		t.Fatalf("invalid shared audit event: %+v", event)
	}
}
