package social

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledNudges struct {
	preference                                              NudgeAudiencePreference
	eligibility                                             NudgeEligibility
	result                                                  NudgeMutationResult
	err, preferenceErr                                      error
	getActor, getPath                                       string
	eligibilityActor, eligibilityRecipient, eligibilityPath string
	eligibilityAt                                           time.Time
	preferenceCommand                                       *NudgePreferenceCommand
	nudgeCommand                                            *NudgeCommand
}

func (repository *controlledNudges) GetNudgeAudience(_ context.Context, actor, path string) (NudgeAudiencePreference, error) {
	repository.getActor, repository.getPath = actor, path
	return repository.preference, repository.preferenceErr
}
func (repository *controlledNudges) UpdateNudgeAudience(_ context.Context, command NudgePreferenceCommand) (NudgeAudiencePreference, error) {
	repository.preferenceCommand = &command
	return repository.preference, repository.preferenceErr
}
func (repository *controlledNudges) GetNudgeEligibility(_ context.Context, actor, recipient, path string, at time.Time) (NudgeEligibility, error) {
	repository.eligibilityActor, repository.eligibilityRecipient, repository.eligibilityPath, repository.eligibilityAt = actor, recipient, path, at
	return repository.eligibility, repository.err
}
func (repository *controlledNudges) SendNudge(_ context.Context, command NudgeCommand) (NudgeMutationResult, error) {
	repository.nudgeCommand = &command
	return repository.result, repository.err
}

func nudgeService(repository *controlledNudges) (*Service, *feedAuthorizer, *controlledAudits) {
	audits := &controlledAudits{}
	service := testService(&controlledProfiles{}, audits)
	authorizer := &feedAuthorizer{allowed: map[string]bool{"path": true}}
	service.Nudges = repository
	service.Authorizer = authorizer
	service.NudgeRateLimiter = &controlledRelationshipLimiter{allowed: true}
	service.NewID = func() string { return "nudge-1" }
	return service, authorizer, audits
}

func presetNudgeContent(preset domain.NudgePreset) domain.NudgeContent {
	return domain.NudgeContent{Kind: domain.NudgeContentPreset, Preset: preset}
}

func TestSendNudgeUsesPathViewAtomicCommandIdempotencyAndAudit(t *testing.T) {
	now := time.Date(2026, 8, 5, 13, 0, 0, 123456000, time.UTC)
	want := domain.Nudge{ID: "nudge-1", SenderID: "viewer", RecipientID: "recipient", PathID: "path", Content: domain.NudgeContent{Kind: domain.NudgeContentPreset, Preset: domain.NudgeLetsGo}, SentAt: now}
	repository := &controlledNudges{result: NudgeMutationResult{Nudge: want}}
	service, authorizer, _ := nudgeService(repository)
	service.Clock = fixedClock{now: now.Add(789 * time.Nanosecond)}
	got, err := service.SendNudge(context.Background(), "Bearer session", "path", "recipient", presetNudgeContent(domain.NudgeLetsGo), "0123456789abcdef")
	if err != nil || got != want {
		t.Fatalf("nudge=%+v err=%v", got, err)
	}
	if len(authorizer.checks) != 1 || authorizer.checks[0] != "path:path:view:viewer" {
		t.Fatalf("authorization=%v", authorizer.checks)
	}
	command := repository.nudgeCommand
	if command == nil || command.Nudge != want || command.Idempotency.PrincipalID != "viewer" || command.Idempotency.Operation != SendNudgeOperation || command.Idempotency.Key != "0123456789abcdef" || len(command.Idempotency.RequestHash) != 32 {
		t.Fatalf("command=%+v", command)
	}
	if command.Audit.Action != audit.ResourceCreated || command.Audit.TargetType != "nudge" || command.Audit.TargetID != "nudge-1" || command.Audit.ActorUserID != "viewer" || command.Audit.OccurredAt != now {
		t.Fatalf("audit=%+v", command.Audit)
	}
}

func TestSendNudgeReplayMayReturnOriginalStableReceipt(t *testing.T) {
	now := time.Date(2026, 8, 5, 13, 0, 0, 0, time.UTC)
	original := domain.Nudge{ID: "original", SenderID: "viewer", RecipientID: "recipient", PathID: "path", Content: domain.NudgeContent{Kind: domain.NudgeContentPreset, Preset: domain.NudgeKeepItGoing}, SentAt: now.Add(-time.Hour)}
	repository := &controlledNudges{result: NudgeMutationResult{Nudge: original, Replayed: true}}
	service, _, _ := nudgeService(repository)
	service.Clock = fixedClock{now: now}
	got, err := service.SendNudge(context.Background(), "Bearer session", "path", "recipient", presetNudgeContent(domain.NudgeKeepItGoing), "0123456789abcdef")
	if err != nil || got != original {
		t.Fatalf("replay=%+v err=%v", got, err)
	}
}

func TestNudgeEligibilityAndPreferenceReadsAreViewerScopedAuthorizedAndAudited(t *testing.T) {
	now := time.Date(2026, 8, 5, 13, 0, 0, 0, time.UTC)
	repository := &controlledNudges{
		preference:  NudgeAudiencePreference{PathID: "path", UserID: "viewer", Audience: domain.DefaultNudgeAudience},
		eligibility: NudgeEligibility{PathID: "path", RecipientUserID: "recipient", Eligible: false, Reason: NudgeGoalCompleteReason},
	}
	service, authorizer, audits := nudgeService(repository)
	service.Clock = fixedClock{now: now}
	preference, err := service.GetNudgeAudience(context.Background(), "Bearer session", "path")
	if err != nil || preference != repository.preference {
		t.Fatalf("preference=%+v err=%v", preference, err)
	}
	eligibility, err := service.GetNudgeEligibility(context.Background(), "Bearer session", "path", "recipient")
	if err != nil || eligibility != repository.eligibility {
		t.Fatalf("eligibility=%+v err=%v", eligibility, err)
	}
	if repository.getActor != "viewer" || repository.getPath != "path" || repository.eligibilityActor != "viewer" || repository.eligibilityRecipient != "recipient" || repository.eligibilityPath != "path" || repository.eligibilityAt != now {
		t.Fatalf("repository=%+v", repository)
	}
	if len(authorizer.checks) != 2 || len(audits.events) != 2 || audits.events[0].Action != audit.ResourceViewed || audits.events[0].TargetType != "nudge_preference" || audits.events[1].TargetType != "nudge_eligibility" {
		t.Fatalf("authorization=%v audits=%+v", authorizer.checks, audits.events)
	}
}

func TestUpdateNudgeAudienceCanOnlyWriteViewerPathPreferenceWithRevision(t *testing.T) {
	now := time.Date(2026, 8, 5, 13, 0, 0, 0, time.UTC)
	want := NudgeAudiencePreference{PathID: "path", UserID: "viewer", Audience: domain.NudgeAudienceFollowers, Revision: 3, UpdatedAt: now}
	repository := &controlledNudges{preference: want}
	service, _, _ := nudgeService(repository)
	service.Clock = fixedClock{now: now}
	got, err := service.UpdateNudgeAudience(context.Background(), "Bearer session", "path", "0123456789abcdef", 2, domain.NudgeAudienceFollowers)
	if err != nil || got != want {
		t.Fatalf("preference=%+v err=%v", got, err)
	}
	command := repository.preferenceCommand
	if command == nil || command.ActorUserID != "viewer" || command.PathID != "path" || command.ExpectedRevision != 2 || command.Audience != domain.NudgeAudienceFollowers || command.ChangedAt != now || command.Idempotency.Operation != UpdateNudgeAudienceOperation || len(command.Idempotency.RequestHash) != 32 || command.Audit.Action != audit.ResourceUpdated || command.Audit.TargetType != "nudge_preference" {
		t.Fatalf("command=%+v", command)
	}
}

func TestNudgeDeniedTargetsRemainOpaqueAndAuditFailureFailsClosed(t *testing.T) {
	for _, test := range []struct {
		name      string
		configure func(*controlledNudges, *feedAuthorizer)
		wantCall  bool
	}{
		{name: "repository hidden", configure: func(repository *controlledNudges, _ *feedAuthorizer) { repository.err = ports.ErrNotFound }, wantCall: true},
		{name: "audience hidden", configure: func(repository *controlledNudges, _ *feedAuthorizer) { repository.err = ErrNudgeAudienceDenied }, wantCall: true},
		{name: "policy hidden", configure: func(_ *controlledNudges, authorizer *feedAuthorizer) { authorizer.allowed["path"] = false }},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &controlledNudges{}
			service, authorizer, audits := nudgeService(repository)
			test.configure(repository, authorizer)
			_, err := service.SendNudge(context.Background(), "Bearer session", "path", "recipient", presetNudgeContent(domain.NudgeLetsGo), "0123456789abcdef")
			if !errors.Is(err, ports.ErrNotFound) || (repository.nudgeCommand != nil) != test.wantCall || len(audits.events) != 1 || audits.events[0].Action != audit.ResourceAccessDenied || audits.events[0].TargetID != "hidden" {
				t.Fatalf("err=%v command=%+v audits=%+v", err, repository.nudgeCommand, audits.events)
			}
			audits.err = errors.New("audit unavailable")
			repository.nudgeCommand = nil
			_, err = service.SendNudge(context.Background(), "Bearer session", "path", "recipient", presetNudgeContent(domain.NudgeLetsGo), "fedcba9876543210")
			if err == nil || errors.Is(err, ports.ErrNotFound) {
				t.Fatalf("audit failure error=%v", err)
			}
		})
	}
}

func TestSendNudgeMapsSemanticRaceFailuresToStablePublicErrors(t *testing.T) {
	for _, test := range []struct {
		name        string
		cause, want error
	}{
		{name: "goal completed", cause: ErrNudgeGoalComplete, want: ports.ErrConflict},
		{name: "window consumed", cause: ErrNudgeAlreadySent, want: platformapp.ErrRateLimited},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &controlledNudges{err: test.cause}
			service, _, audits := nudgeService(repository)
			_, err := service.SendNudge(context.Background(), "Bearer session", "path", "recipient", presetNudgeContent(domain.NudgeLetsGo), "0123456789abcdef")
			if !errors.Is(err, test.want) || len(audits.events) != 0 {
				t.Fatalf("error=%v audits=%+v", err, audits.events)
			}
		})
	}
}

func TestNudgeRejectsBadInputsRateLimitAndMalformedAdapterOutputs(t *testing.T) {
	repository := &controlledNudges{}
	service, _, _ := nudgeService(repository)
	for _, call := range []func() error{
		func() error {
			_, err := service.SendNudge(context.Background(), "Bearer session", "path", "viewer", presetNudgeContent(domain.NudgeLetsGo), "0123456789abcdef")
			return err
		},
		func() error {
			_, err := service.SendNudge(context.Background(), "Bearer session", "path", "recipient", domain.NudgeContent{Kind: "custom", Preset: domain.NudgeLetsGo}, "0123456789abcdef")
			return err
		},
		func() error {
			_, err := service.UpdateNudgeAudience(context.Background(), "Bearer session", "path", "short", 0, domain.NudgeAudienceEveryone)
			return err
		},
		func() error {
			_, err := service.UpdateNudgeAudience(context.Background(), "Bearer session", "path", "0123456789abcdef", -1, domain.NudgeAudienceEveryone)
			return err
		},
	} {
		if err := call(); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("invalid input error=%v", err)
		}
	}
	service.NudgeRateLimiter = &controlledRelationshipLimiter{}
	if _, err := service.GetNudgeEligibility(context.Background(), "Bearer session", "path", "recipient"); !errors.Is(err, platformapp.ErrRateLimited) {
		t.Fatalf("rate-limit error=%v", err)
	}
	if repository.eligibilityActor != "" || repository.nudgeCommand != nil || repository.preferenceCommand != nil {
		t.Fatalf("invalid request reached repository: %+v", repository)
	}

	service.NudgeRateLimiter = &controlledRelationshipLimiter{allowed: true}
	repository.eligibility = NudgeEligibility{PathID: "path", RecipientUserID: "recipient", Eligible: true, Reason: NudgeGoalCompleteReason}
	if _, err := service.GetNudgeEligibility(context.Background(), "Bearer session", "path", "recipient"); !errors.Is(err, errInvalidDependencies) {
		t.Fatalf("malformed eligibility error=%v", err)
	}
	repository.result = NudgeMutationResult{Nudge: domain.Nudge{ID: "other", SenderID: "viewer", RecipientID: "wrong", PathID: "path", Content: domain.NudgeContent{Kind: domain.NudgeContentPreset, Preset: domain.NudgeLetsGo}, SentAt: time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)}}
	if _, err := service.SendNudge(context.Background(), "Bearer session", "path", "recipient", presetNudgeContent(domain.NudgeLetsGo), "0123456789abcdef"); !errors.Is(err, errInvalidDependencies) {
		t.Fatalf("malformed send error=%v", err)
	}
}

func TestNudgeIdempotencyHashesBindEverySemanticField(t *testing.T) {
	base := nudgeIdempotency("viewer", "key", "path", "recipient", presetNudgeContent(domain.NudgeLetsGo))
	variants := []ports.Idempotency{
		nudgeIdempotency("viewer", "key", "other", "recipient", presetNudgeContent(domain.NudgeLetsGo)),
		nudgeIdempotency("viewer", "key", "path", "other", presetNudgeContent(domain.NudgeLetsGo)),
		nudgeIdempotency("viewer", "key", "path", "recipient", presetNudgeContent(domain.NudgeTimeToWork)),
		nudgeIdempotency("viewer", "key", "path", "recipient", domain.NudgeContent{Kind: "future", Preset: domain.NudgeLetsGo}),
	}
	for _, variant := range variants {
		if reflect.DeepEqual(base.RequestHash, variant.RequestHash) {
			t.Fatalf("unbound nudge idempotency: base=%x variant=%x", base.RequestHash, variant.RequestHash)
		}
	}
	preference := nudgePreferenceIdempotency("viewer", "key", "path", 1, domain.NudgeAudienceFollowers)
	for _, variant := range []ports.Idempotency{
		nudgePreferenceIdempotency("viewer", "key", "other", 1, domain.NudgeAudienceFollowers),
		nudgePreferenceIdempotency("viewer", "key", "path", 2, domain.NudgeAudienceFollowers),
		nudgePreferenceIdempotency("viewer", "key", "path", 1, domain.NudgeAudienceEveryone),
	} {
		if reflect.DeepEqual(preference.RequestHash, variant.RequestHash) {
			t.Fatalf("unbound preference idempotency: base=%x variant=%x", preference.RequestHash, variant.RequestHash)
		}
	}
}
