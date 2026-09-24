package path

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
	"time"
)

// Dependencies is the complete platform capability set available to this
// domain service. Keep policy and product behavior in this package; adapters are
// injected through these ports by the generated composition registry.
type Dependencies struct {
	Auth                    ports.Authenticator
	Profiles                ProfileReader
	Authorizer              ports.Authorizer
	AuthorizationOutbox     ports.AuthorizationOutbox
	AuthorizationSerializer ports.AuthorizationSerializer
	Repository              Repository
	MemberRemoval           MemberRemovalRepository
	InvitationManagement    InvitationManagementRepository
	Audits                  ports.Audits
	AuditRateLimiter        ports.AuditRateLimiter
	Clock                   ports.Clock
	Probe                   ports.Probe
	NewID                   func() string
	AuthorizationWorker     string
	AuthorizationLease      time.Duration
	CursorSigningKey        []byte
}

type Service struct{ Dependencies }

var errInvalidPathDependencies = errors.New("path service dependencies are invalid")

func New(dependencies Dependencies) *Service {
	return &Service{Dependencies: dependencies}
}

// requireConfiguredPolicy authenticates before returning the fail-closed
// scaffold result. Replace calls to this helper with the domain's specified
// authorization policy and use cases; never remove authentication or permit a
// request merely because its persistence row exists.
func (s *Service) requireConfiguredPolicy(ctx context.Context, authorization string) error {
	if _, err := s.authenticate(ctx, authorization); err != nil {
		return err
	}
	return ports.ErrAuthorizationPolicyNotConfigured
}

func (s *Service) authenticate(ctx context.Context, authorization string) (ports.Principal, error) {
	if s.Auth == nil {
		return ports.Principal{}, errInvalidPathDependencies
	}
	principal, err := s.Auth.Authenticate(ctx, authorization)
	if err != nil {
		return ports.Principal{}, err
	}
	if principal.UserID == "" || len(principal.Scopes) != 1 || principal.Scopes[0] != "api:user" {
		return ports.Principal{}, platformapp.ErrUnauthenticated
	}
	return principal, nil
}

func (s *Service) Create(ctx context.Context, authorization, idempotencyKey string, attributes domain.Attributes) (domain.Entity, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return domain.Entity{}, err
	}
	if !validIdempotencyKey(idempotencyKey) {
		return domain.Entity{}, ports.ErrInvalidArgument
	}
	if s.Profiles == nil || s.Repository == nil || s.Authorizer == nil || s.AuthorizationOutbox == nil || s.AuthorizationSerializer == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil || s.NewID == nil || s.AuthorizationWorker == "" || s.AuthorizationLease <= 0 {
		return domain.Entity{}, errInvalidPathDependencies
	}
	now := s.Clock.Now().UTC()
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return domain.Entity{}, platformapp.ErrRateLimited
	}
	requestedVisibility := strings.TrimSpace(attributes.Visibility)
	if requestedVisibility != "" && requestedVisibility != "private" && requestedVisibility != "followers" && requestedVisibility != "public" {
		return domain.Entity{}, ports.ErrInvalidArgument
	}
	digest := canonicalCreateRequestHash(attributes)
	profile, err := s.Profiles.PathCreationProfile(ctx, principal.UserID)
	if err != nil {
		return domain.Entity{}, err
	}
	if identity.ValidateProfileVisibility(profile.ProfileVisibility) != nil || identity.ValidateFirstDayOfWeek(profile.FirstDayOfWeek) != nil {
		return domain.Entity{}, errInvalidPathDependencies
	}
	visibility, err := boundedVisibility(profile.ProfileVisibility, requestedVisibility)
	if err != nil {
		return domain.Entity{}, err
	}
	intervalGoal := defaultIntervalGoalAlignment(attributes.IntervalGoal, profile.FirstDayOfWeek)
	entity, err := domain.New(domain.ID(s.NewID()), principal.UserID, domain.Attributes{
		Name:          attributes.Name,
		Visibility:    visibility,
		IntervalGoal:  intervalGoal,
		OverallTarget: attributes.OverallTarget,
	})
	if err != nil {
		return domain.Entity{}, ports.ErrInvalidArgument
	}
	entity.CreatedAt = now
	entity.UpdatedAt = now
	change := ports.AuthorizationChange{
		ID: s.NewID(), ResourceType: "path", ResourceID: string(entity.ID), Relation: "creator",
		SubjectType: "user", SubjectID: principal.UserID, OwnerUserID: principal.UserID, ActorUserID: principal.UserID,
		Operation: ports.AuthorizationTouch, LockedBy: s.AuthorizationWorker, Lease: s.AuthorizationLease,
	}
	event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceCreated, "path", string(entity.ID), audit.Succeeded)
	created, replayed, err := s.Repository.Create(ctx, entity, change, ports.Idempotency{PrincipalID: principal.UserID, Operation: "path.create", Key: idempotencyKey, RequestHash: digest[:]}, event)
	if err != nil {
		return domain.Entity{}, err
	}
	if !validCreatedPath(created, principal.UserID) {
		return domain.Entity{}, errInvalidPathDependencies
	}
	if replayed {
		pending, claimErr := s.AuthorizationOutbox.ClaimAuthorizationChangeForResource(ctx, "path", string(created.ID), s.AuthorizationWorker, s.AuthorizationLease)
		if errors.Is(claimErr, ports.ErrNotFound) {
			return created, nil
		}
		if claimErr != nil {
			return domain.Entity{}, claimErr
		}
		if !validCreateRelationship(pending, created) {
			return domain.Entity{}, errInvalidPathDependencies
		}
		if err := s.reconcileRelationship(ctx, pending); err != nil {
			return domain.Entity{}, err
		}
		return created, nil
	}
	if err := s.reconcileRelationship(ctx, change); err != nil {
		return domain.Entity{}, err
	}
	return created, nil
}

func defaultIntervalGoalAlignment(goal domain.IntervalGoal, firstDayOfWeek identity.FirstDayOfWeek) domain.IntervalGoal {
	if !goal.Present || goal.Alignment != (domain.GoalAlignment{}) {
		return goal
	}
	switch goal.Recurrence {
	case domain.RecurrenceHourly:
		goal.Alignment.Minute = 0
	case domain.RecurrenceDaily:
		goal.Alignment.Hour = 0
	case domain.RecurrenceWeekly:
		goal.Alignment.ISOWeekday = int(firstDayOfWeek)
	case domain.RecurrenceMonthly:
		goal.Alignment.Day = 1
	case domain.RecurrenceYearly:
		goal.Alignment.Month = 1
		goal.Alignment.Day = 1
	}
	return goal
}

func canonicalCreateRequestHash(attributes domain.Attributes) [sha256.Size]byte {
	digest := sha256.New()
	writeHashString(digest, strings.TrimSpace(attributes.Name))
	writeHashString(digest, strings.TrimSpace(attributes.Visibility))
	writeHashBool(digest, attributes.IntervalGoal.Present)
	writeHashInt64(digest, attributes.IntervalGoal.TargetSeconds)
	writeHashString(digest, string(attributes.IntervalGoal.Recurrence))
	writeHashInt64(digest, int64(attributes.IntervalGoal.Alignment.Minute))
	writeHashInt64(digest, int64(attributes.IntervalGoal.Alignment.Hour))
	writeHashInt64(digest, int64(attributes.IntervalGoal.Alignment.ISOWeekday))
	writeHashInt64(digest, int64(attributes.IntervalGoal.Alignment.Month))
	writeHashInt64(digest, int64(attributes.IntervalGoal.Alignment.Day))
	writeHashBool(digest, attributes.OverallTarget.Present)
	writeHashInt64(digest, attributes.OverallTarget.TargetSeconds)
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}

type hashWriter interface {
	Write([]byte) (int, error)
}

func writeHashString(digest hashWriter, value string) {
	writeHashUint64(digest, uint64(len(value)))
	_, _ = digest.Write([]byte(value))
}

func writeHashBool(digest hashWriter, value bool) {
	encoded := byte(0)
	if value {
		encoded = 1
	}
	_, _ = digest.Write([]byte{encoded})
}

func writeHashInt64(digest hashWriter, value int64) {
	writeHashUint64(digest, uint64(value))
}

func writeHashUint64(digest hashWriter, value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	_, _ = digest.Write(encoded[:])
}

func validCreatedPath(entity domain.Entity, ownerUserID string) bool {
	if entity.ID == "" || entity.OwnerUserID != ownerUserID || entity.CreatedAt.IsZero() || entity.UpdatedAt.IsZero() {
		return false
	}
	validated, err := domain.New(entity.ID, entity.OwnerUserID, entity.Attributes)
	return err == nil && validated.Attributes == entity.Attributes
}

func validCreateRelationship(change ports.AuthorizationChange, entity domain.Entity) bool {
	validRelation := change.Relation == "creator" ||
		(entity.Visibility == "followers" && change.Relation == "followers_owner") ||
		(entity.Visibility == "public" && change.Relation == "public_viewer")
	validSubject := change.SubjectID == entity.OwnerUserID ||
		(change.Relation == "public_viewer" && change.SubjectID == "*")
	return change.ID != "" && change.ResourceType == "path" && change.ResourceID == string(entity.ID) &&
		validRelation && change.SubjectType == "user" && validSubject &&
		change.OwnerUserID == entity.OwnerUserID && change.ActorUserID == entity.OwnerUserID && change.Operation == ports.AuthorizationTouch
}

func validIdempotencyKey(value string) bool {
	if len(value) < 16 || len(value) > 128 {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func boundedVisibility(profile identity.ProfileVisibility, requested string) (string, error) {
	switch profile {
	case identity.ProfileVisibilityPublic:
		if requested == "" {
			return "public", nil
		}
		return requested, nil
	case identity.ProfileVisibilityPrivate:
		if requested == "" {
			return "followers", nil
		}
		if requested == "public" {
			return "", ports.ErrInvalidArgument
		}
		return requested, nil
	default:
		return "", errInvalidPathDependencies
	}
}

func (s *Service) reconcileRelationship(ctx context.Context, change ports.AuthorizationChange) error {
	var relationshipErr error
	err := s.AuthorizationSerializer.WithinResource(ctx, change.ResourceType, change.ResourceID, func(locked context.Context) error {
		if err := s.AuthorizationOutbox.RenewAuthorizationChange(locked, change.ID, s.AuthorizationWorker, s.AuthorizationLease); err != nil {
			return err
		}
		switch change.Operation {
		case ports.AuthorizationTouch:
			relationshipErr = s.Authorizer.WriteRelationship(locked, change.ResourceType, change.ResourceID, change.Relation, change.SubjectType, change.SubjectID)
		case ports.AuthorizationDelete:
			relationshipErr = s.Authorizer.DeleteRelationship(locked, change.ResourceType, change.ResourceID, change.Relation, change.SubjectType, change.SubjectID)
		default:
			return errInvalidPathDependencies
		}
		if relationshipErr != nil {
			auditErr := s.Audits.AppendAuditEvent(locked, shared.NewAuditEvent(locked, s.Clock, change.OwnerUserID, change.ActorUserID, audit.AuthorizationFailed, change.ResourceType, change.ResourceID, audit.Failed))
			return errors.Join(relationshipErr, auditErr)
		}
		event := shared.NewAuditEvent(locked, s.Clock, change.OwnerUserID, change.ActorUserID, audit.AuthorizationApplied, change.ResourceType, change.ResourceID, audit.Succeeded)
		return s.AuthorizationOutbox.CompleteAuthorizationChangeWithAudit(locked, change.ID, s.AuthorizationWorker, event)
	})
	if relationshipErr != nil {
		_, failErr := s.AuthorizationOutbox.FailAuthorizationChange(ctx, change.ID, s.AuthorizationWorker, 5, "dependency_failure")
		return errors.Join(err, failErr)
	}
	return err
}

func (s *Service) List(ctx context.Context, authorization, cursor string, limit int) ([]domain.Entity, string, error) {
	return s.list(ctx, authorization, cursor, limit, false)
}

func (s *Service) ListArchived(ctx context.Context, authorization, cursor string, limit int) ([]domain.Entity, string, error) {
	return s.list(ctx, authorization, cursor, limit, true)
}

func (s *Service) list(ctx context.Context, authorization, cursor string, limit int, archived bool) ([]domain.Entity, string, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	if s.Authorizer == nil || s.Repository == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil || len(s.CursorSigningKey) < 32 {
		return nil, "", errInvalidPathDependencies
	}
	now := s.Clock.Now().UTC()
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return nil, "", platformapp.ErrRateLimited
	}
	cursorDomain := "path"
	if archived {
		cursorDomain = "path-archived"
	}
	request := PageRequest{Limit: limit, Snapshot: now, Archived: archived}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(s.CursorSigningKey, cursor)
		if decodeErr != nil || payload.Owner != principal.UserID || payload.Domain != cursorDomain {
			return nil, "", ports.ErrInvalidArgument
		}
		request.AfterID = domain.ID(payload.AfterID)
		request.AfterCreated = payload.AfterCreated
		request.Snapshot = payload.Snapshot
	}
	page, err := s.Repository.List(ctx, principal.UserID, request)
	if err != nil {
		return nil, "", err
	}
	if len(page.Items) > limit || (page.HasMore && len(page.Items) == 0) {
		return nil, "", errInvalidPathDependencies
	}
	for _, entity := range page.Items {
		if entity.ID == "" || entity.Archived() != archived {
			return nil, "", errInvalidPathDependencies
		}
		allowed, checkErr := s.Authorizer.Check(ctx, "path", string(entity.ID), "view", principal.UserID)
		if checkErr != nil {
			return nil, "", checkErr
		}
		if !allowed {
			event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", "list", audit.Denied)
			if auditErr := s.Audits.AppendAuditEvent(ctx, event); auditErr != nil {
				return nil, "", auditErr
			}
			return nil, "", platformapp.ErrForbidden
		}
	}
	nextCursor := ""
	if page.HasMore && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		if last.ID == "" || last.CreatedAt.IsZero() {
			return nil, "", errInvalidPathDependencies
		}
		nextCursor, err = shared.EncodeCursor(s.CursorSigningKey, shared.CursorPayload{Version: 1, Owner: principal.UserID, Domain: cursorDomain, AfterID: string(last.ID), AfterCreated: last.CreatedAt, Snapshot: request.Snapshot})
		if err != nil {
			return nil, "", err
		}
	}
	event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceListed, "path", "path", audit.Succeeded)
	if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	return page.Items, nextCursor, nil
}

func (s *Service) Get(ctx context.Context, authorization string, id domain.ID) (domain.Entity, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return domain.Entity{}, err
	}
	if id == "" || s.Authorizer == nil || s.Repository == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil {
		return domain.Entity{}, errInvalidPathDependencies
	}
	now := s.Clock.Now().UTC()
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return domain.Entity{}, platformapp.ErrRateLimited
	}
	allowed, err := s.Authorizer.Check(ctx, "path", string(id), "view", principal.UserID)
	if err != nil {
		return domain.Entity{}, err
	}
	if !allowed {
		event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(id), audit.Denied)
		if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
			return domain.Entity{}, err
		}
		return domain.Entity{}, platformapp.ErrForbidden
	}
	entity, err := s.Repository.Get(ctx, principal.UserID, id)
	if err != nil {
		return domain.Entity{}, err
	}
	event := shared.NewAuditEvent(ctx, s.Clock, entity.OwnerUserID, principal.UserID, audit.ResourceViewed, "path", string(id), audit.Succeeded)
	if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return domain.Entity{}, err
	}
	return entity, nil
}

func (s *Service) Rename(ctx context.Context, authorization, idempotencyKey string, id domain.ID, expectedName, name string) (RenameResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return RenameResult{}, err
	}
	if id == "" || !validIdempotencyKey(idempotencyKey) ||
		strings.TrimSpace(expectedName) != expectedName || expectedName == "" {
		return RenameResult{}, ports.ErrInvalidArgument
	}
	validatedExpected, err := domain.New("rename-validation", "rename-validation", domain.Attributes{
		Name: expectedName, Visibility: "private",
	})
	if err != nil || validatedExpected.Name != expectedName {
		return RenameResult{}, ports.ErrInvalidArgument
	}
	validatedName, err := domain.New("rename-validation", "rename-validation", domain.Attributes{
		Name: name, Visibility: "private",
	})
	if err != nil {
		return RenameResult{}, ports.ErrInvalidArgument
	}
	if s.Authorizer == nil || s.Repository == nil || s.Audits == nil ||
		s.AuditRateLimiter == nil || s.Clock == nil {
		return RenameResult{}, errInvalidPathDependencies
	}
	now := s.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return RenameResult{}, errInvalidPathDependencies
	}
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return RenameResult{}, platformapp.ErrRateLimited
	}
	allowed, err := s.Authorizer.Check(ctx, "path", string(id), "rename", principal.UserID)
	if err != nil {
		return RenameResult{}, err
	}
	if !allowed {
		event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(id), audit.Denied)
		event.OccurredAt = now
		if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
			return RenameResult{}, err
		}
		return RenameResult{}, platformapp.ErrForbidden
	}
	existing, err := s.Repository.Get(ctx, principal.UserID, id)
	if err != nil {
		return RenameResult{}, err
	}
	if existing.ID != id || !validCreatedPath(existing, existing.OwnerUserID) {
		return RenameResult{}, errInvalidPathDependencies
	}
	if existing.Archived() {
		return RenameResult{}, ports.ErrConflict
	}
	renamed, err := existing.Rename(validatedName.Name, now)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidState) {
			return RenameResult{}, ports.ErrConflict
		}
		return RenameResult{}, ports.ErrInvalidArgument
	}
	digest := canonicalRenameRequestHash(id, validatedExpected.Name, renamed.Name)
	event := shared.NewAuditEvent(ctx, s.Clock, existing.OwnerUserID, principal.UserID, audit.ResourceUpdated, "path", string(id), audit.Succeeded)
	event.OccurredAt = now
	result, err := s.Repository.Rename(ctx, RenameCommand{
		ActorUserID: principal.UserID, ExpectedName: validatedExpected.Name, Path: renamed,
		Idempotency: ports.Idempotency{
			PrincipalID: principal.UserID, Operation: RenameOperation,
			Key: idempotencyKey, RequestHash: digest[:],
		},
		Audit: event,
	})
	if err != nil {
		return RenameResult{}, err
	}
	if !validRenameResult(result, existing, renamed) {
		return RenameResult{}, errInvalidPathDependencies
	}
	return result, nil
}

func canonicalRenameRequestHash(id domain.ID, expectedName, name string) [sha256.Size]byte {
	digest := sha256.New()
	writeHashString(digest, string(id))
	writeHashString(digest, expectedName)
	writeHashString(digest, name)
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func validRenameResult(result RenameResult, existing, requested domain.Entity) bool {
	if result.Path.ID != existing.ID || result.Path.OwnerUserID != existing.OwnerUserID ||
		result.Path.Name != requested.Name || result.Path.Visibility != existing.Visibility ||
		result.Path.IntervalGoal != existing.IntervalGoal || result.Path.OverallTarget != existing.OverallTarget ||
		result.Path.CreatedAt != existing.CreatedAt || result.Path.Archived() ||
		!validCreatedPath(result.Path, existing.OwnerUserID) {
		return false
	}
	return result.Replayed || result.Path == requested
}

func (s *Service) UpdateGoals(ctx context.Context, authorization, idempotencyKey string, id domain.ID, confirmed bool, expected domain.GoalConfiguration, intervalGoal domain.IntervalGoal, overallTarget domain.OverallTarget) (UpdateGoalsResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return UpdateGoalsResult{}, err
	}
	if !confirmed || id == "" || !validIdempotencyKey(idempotencyKey) || !expected.Valid() {
		return UpdateGoalsResult{}, ports.ErrInvalidArgument
	}
	if s.Authorizer == nil || s.Repository == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil {
		return UpdateGoalsResult{}, errInvalidPathDependencies
	}
	now := s.Clock.Now().UTC()
	if now.IsZero() {
		return UpdateGoalsResult{}, errInvalidPathDependencies
	}
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return UpdateGoalsResult{}, platformapp.ErrRateLimited
	}
	allowed, err := s.Authorizer.Check(ctx, "path", string(id), "manage_goals", principal.UserID)
	if err != nil {
		return UpdateGoalsResult{}, err
	}
	if !allowed {
		event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(id), audit.Denied)
		if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
			return UpdateGoalsResult{}, err
		}
		return UpdateGoalsResult{}, platformapp.ErrForbidden
	}
	existing, err := s.Repository.Get(ctx, principal.UserID, id)
	if err != nil {
		return UpdateGoalsResult{}, err
	}
	if existing.ID != id || !validCreatedPath(existing, existing.OwnerUserID) {
		return UpdateGoalsResult{}, errInvalidPathDependencies
	}
	if existing.Archived() {
		return UpdateGoalsResult{}, ports.ErrConflict
	}
	updated, err := existing.ReconfigureGoals(intervalGoal, overallTarget, now)
	if err != nil {
		return UpdateGoalsResult{}, ports.ErrInvalidArgument
	}
	participantTimeZone := ""
	if updated.IntervalGoal.Present {
		if s.Profiles == nil {
			return UpdateGoalsResult{}, errInvalidPathDependencies
		}
		participantTimeZone, err = s.Profiles.TimeZone(ctx, principal.UserID)
		if err != nil {
			return UpdateGoalsResult{}, err
		}
		if participantTimeZone == "" || participantTimeZone == "Local" || strings.TrimSpace(participantTimeZone) != participantTimeZone {
			return UpdateGoalsResult{}, errInvalidPathDependencies
		}
		if _, err := time.LoadLocation(participantTimeZone); err != nil {
			return UpdateGoalsResult{}, errInvalidPathDependencies
		}
	}
	digest := canonicalUpdateGoalsRequestHash(id, expected, intervalGoal, overallTarget)
	event := shared.NewAuditEvent(ctx, s.Clock, existing.OwnerUserID, principal.UserID, audit.ResourceUpdated, "path", string(id), audit.Succeeded)
	event.OccurredAt = now
	result, err := s.Repository.UpdateGoals(ctx, UpdateGoalsCommand{
		ActorUserID: principal.UserID, ParticipantTimeZone: participantTimeZone,
		ExpectedGoals: expected, Path: updated, ProjectedAt: now,
		Idempotency: ports.Idempotency{
			PrincipalID: principal.UserID, Operation: UpdateGoalsOperation,
			Key: idempotencyKey, RequestHash: digest[:],
		},
		Audit: event,
	})
	if err != nil {
		return UpdateGoalsResult{}, err
	}
	if !validUpdateGoalsResult(result, existing, updated) {
		return UpdateGoalsResult{}, errInvalidPathDependencies
	}
	return result, nil
}

func (s *Service) SetArchiveState(ctx context.Context, authorization, idempotencyKey string, id domain.ID, confirmed, expectedArchived, archived bool) (SetArchiveStateResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return SetArchiveStateResult{}, err
	}
	if !confirmed || id == "" || expectedArchived == archived || !validIdempotencyKey(idempotencyKey) {
		return SetArchiveStateResult{}, ports.ErrInvalidArgument
	}
	if s.Authorizer == nil || s.Repository == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil || s.NewID == nil {
		return SetArchiveStateResult{}, errInvalidPathDependencies
	}
	// PostgreSQL timestamptz persists microseconds. Canonicalize the lifecycle
	// instant before applying it so the first response and later reads agree.
	now := s.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return SetArchiveStateResult{}, errInvalidPathDependencies
	}
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return SetArchiveStateResult{}, platformapp.ErrRateLimited
	}
	allowed, err := s.Authorizer.Check(ctx, "path", string(id), "manage_lifecycle", principal.UserID)
	if err != nil {
		return SetArchiveStateResult{}, err
	}
	if !allowed {
		event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(id), audit.Denied)
		if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
			return SetArchiveStateResult{}, err
		}
		return SetArchiveStateResult{}, platformapp.ErrForbidden
	}
	existing, err := s.Repository.Get(ctx, principal.UserID, id)
	if err != nil {
		return SetArchiveStateResult{}, err
	}
	if existing.ID != id || !validCreatedPath(existing, existing.OwnerUserID) {
		return SetArchiveStateResult{}, errInvalidPathDependencies
	}
	requested := existing
	if existing.Archived() == expectedArchived {
		if archived {
			requested, err = existing.Archive(now)
		} else {
			requested, err = existing.Unarchive(now)
		}
		if err != nil {
			return SetArchiveStateResult{}, ports.ErrConflict
		}
	} else if existing.Archived() != archived {
		return SetArchiveStateResult{}, ports.ErrConflict
	}
	digest := canonicalSetArchiveStateRequestHash(id, expectedArchived, archived)
	event := shared.NewAuditEvent(ctx, s.Clock, existing.OwnerUserID, principal.UserID, audit.ResourceUpdated, "path", string(id), audit.Succeeded)
	event.OccurredAt = now
	result, err := s.Repository.SetArchiveState(ctx, SetArchiveStateCommand{
		ActorUserID: principal.UserID, ExpectedArchived: expectedArchived, Archived: archived,
		Path: requested, ChangedAt: now,
		Idempotency: ports.Idempotency{
			PrincipalID: principal.UserID, Operation: SetArchiveStateOperation,
			Key: idempotencyKey, RequestHash: digest[:],
		},
		Audit: event, NewActivityID: s.NewID,
	})
	if err != nil {
		return SetArchiveStateResult{}, err
	}
	if !validSetArchiveStateResult(result, existing, requested, archived) {
		return SetArchiveStateResult{}, errInvalidPathDependencies
	}
	return result, nil
}

func canonicalSetArchiveStateRequestHash(id domain.ID, expectedArchived, archived bool) [sha256.Size]byte {
	digest := sha256.New()
	writeHashString(digest, string(id))
	writeHashBool(digest, expectedArchived)
	writeHashBool(digest, archived)
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func validSetArchiveStateResult(result SetArchiveStateResult, existing, requested domain.Entity, archived bool) bool {
	if result.Path.ID != existing.ID || result.Path.OwnerUserID != existing.OwnerUserID || result.Path.Attributes != existing.Attributes ||
		result.Path.CreatedAt != existing.CreatedAt || result.Path.Archived() != archived || !validCreatedPath(result.Path, existing.OwnerUserID) {
		return false
	}
	return result.Replayed || result.Path == requested
}

func canonicalUpdateGoalsRequestHash(id domain.ID, expected domain.GoalConfiguration, intervalGoal domain.IntervalGoal, overallTarget domain.OverallTarget) [sha256.Size]byte {
	digest := sha256.New()
	writeHashString(digest, string(id))
	writeGoalConfigurationHash(digest, expected)
	writeGoalConfigurationHash(digest, domain.GoalConfiguration{IntervalGoal: intervalGoal, OverallTarget: overallTarget})
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func writeGoalConfigurationHash(digest hashWriter, configuration domain.GoalConfiguration) {
	intervalGoal, overallTarget := configuration.IntervalGoal, configuration.OverallTarget
	writeHashBool(digest, intervalGoal.Present)
	writeHashInt64(digest, intervalGoal.TargetSeconds)
	writeHashString(digest, string(intervalGoal.Recurrence))
	writeHashInt64(digest, int64(intervalGoal.Alignment.Minute))
	writeHashInt64(digest, int64(intervalGoal.Alignment.Hour))
	writeHashInt64(digest, int64(intervalGoal.Alignment.ISOWeekday))
	writeHashInt64(digest, int64(intervalGoal.Alignment.Month))
	writeHashInt64(digest, int64(intervalGoal.Alignment.Day))
	writeHashBool(digest, overallTarget.Present)
	writeHashInt64(digest, overallTarget.TargetSeconds)
}

func validUpdateGoalsResult(result UpdateGoalsResult, existing, requested domain.Entity) bool {
	if result.AccumulatedSeconds < 0 || result.Path.ID != existing.ID || result.Path.OwnerUserID != existing.OwnerUserID ||
		result.Path.Name != existing.Name || result.Path.Visibility != existing.Visibility || result.Path.CreatedAt != existing.CreatedAt ||
		!validCreatedPath(result.Path, existing.OwnerUserID) {
		return false
	}
	if !result.Replayed && result.Path != requested {
		return false
	}
	if !result.Path.IntervalGoal.Present {
		return result.IntervalProgress == nil
	}
	progress := result.IntervalProgress
	return progress != nil && progress.TargetSeconds == result.Path.IntervalGoal.TargetSeconds && progress.AccumulatedSeconds >= 0 &&
		!progress.StartedAt.IsZero() && !progress.EndedAt.IsZero() && progress.StartedAt.Location() == time.UTC &&
		progress.EndedAt.Location() == time.UTC && progress.EndedAt.After(progress.StartedAt)
}

func (s *Service) Update(ctx context.Context, authorization string, _ domain.ID, _ domain.Attributes) (domain.Entity, error) {
	return domain.Entity{}, s.requireConfiguredPolicy(ctx, authorization)
}
