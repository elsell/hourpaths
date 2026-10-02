package activity

import (
	"context"
	"strconv"
	"strings"
	"time"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const OfflineActivityOperation = "activity.offline.recorded"

type OfflineActivityInput struct {
	Kind               string
	ActivityID         string
	StartedAt          time.Time
	DurationSeconds    int64
	OccurrenceTimeZone string
	Note               string
	AuthoredAt         time.Time
	Counter            int64
}
type OfflineActivityCommand struct {
	Kind        string
	Activity    domain.RecordedActivity
	Order       domain.ActivityEditOrder
	Idempotency ports.Idempotency
	Audit       audit.Event
}
type OfflineActivityResult struct {
	Outcome  string
	Activity *domain.RecordedActivity
	Order    *domain.ActivityEditOrder
	Replayed bool
}
type OfflineActivityRepository interface {
	SynchronizeActivity(context.Context, OfflineActivityCommand) (OfflineActivityResult, error)
}

func (s *Service) SynchronizeActivity(ctx context.Context, authorization, pathID, key string, input OfflineActivityInput) (OfflineActivityResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return OfflineActivityResult{}, err
	}
	if !validPathID(pathID) || !validIdempotencyKey(key) || input.ActivityID == "" || len(input.ActivityID) > 128 || strings.TrimSpace(input.ActivityID) != input.ActivityID || (input.Kind != "create" && input.Kind != "edit") {
		return OfflineActivityResult{}, ports.ErrInvalidArgument
	}
	now, err := s.authorizeTracking(ctx, principal.UserID, pathID)
	if err != nil {
		return OfflineActivityResult{}, err
	}
	if s.Repository == nil {
		return OfflineActivityResult{}, errInvalidDependencies
	}
	order, err := domain.NewActivityEditOrder(input.AuthoredAt, input.Counter, key)
	if err != nil {
		return OfflineActivityResult{}, ports.ErrInvalidArgument
	}
	entry, err := domain.RecordManualActivity(domain.ManualActivity{ID: input.ActivityID, PathID: pathID, ParticipantID: principal.UserID, StartedAt: durableInstant(input.StartedAt), DurationSeconds: input.DurationSeconds, OccurrenceTimeZone: input.OccurrenceTimeZone, Note: input.Note}, now)
	if err != nil {
		return OfflineActivityResult{}, ports.ErrInvalidArgument
	}
	action := audit.ResourceCreated
	if input.Kind == "edit" {
		action = audit.ResourceUpdated
	}
	event := s.auditEvent(ctx, principal.UserID, action, entry.ID)
	event.TargetType = "activity"
	command := OfflineActivityCommand{Kind: input.Kind, Activity: entry, Order: order, Audit: event, Idempotency: ports.Idempotency{PrincipalID: principal.UserID, Operation: OfflineActivityOperation, Key: key, RequestHash: requestHash(OfflineActivityOperation, pathID, input.Kind, entry.ID, entry.StartedAt.Format(time.RFC3339Nano), strconv.FormatInt(input.DurationSeconds, 10), entry.OccurrenceTimeZone, entry.Note, order.AuthoredAt.Format(time.RFC3339Nano), strconv.FormatInt(order.Counter, 10))}}
	result, err := s.Repository.SynchronizeActivity(ctx, command)
	if err != nil {
		return OfflineActivityResult{}, err
	}
	switch result.Outcome {
	case "accepted":
		if result.Activity == nil || result.Order == nil || result.Activity.ID != entry.ID || result.Activity.PathID != pathID || result.Activity.ParticipantID != principal.UserID || result.Activity.ValidateAt(now) != nil {
			return OfflineActivityResult{}, errInvalidDependencies
		}
		if _, err := domain.NewActivityEditOrder(result.Order.AuthoredAt, result.Order.Counter, result.Order.OperationID); err != nil {
			return OfflineActivityResult{}, errInvalidDependencies
		}
	case "deleted", "archived":
		if result.Activity != nil || result.Order != nil {
			return OfflineActivityResult{}, errInvalidDependencies
		}
	default:
		return OfflineActivityResult{}, errInvalidDependencies
	}
	return result, nil
}
