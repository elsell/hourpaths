package audit

import (
	"testing"
	"time"
)

func TestEventValidationRejectsUnknownTaxonomy(t *testing.T) {
	valid := Event{ID: "event", OwnerUserID: "owner", ActorUserID: "actor", Action: ResourceViewed, TargetType: "habit", TargetID: "habit", Outcome: Succeeded, CorrelationID: "request", OccurredAt: time.Now()}
	if !valid.Valid() {
		t.Fatal("known complete event was rejected")
	}
	unknownAction := valid
	unknownAction.Action = "resource.maybe"
	if unknownAction.Valid() {
		t.Fatal("unknown audit action was accepted")
	}
	unknownOutcome := valid
	unknownOutcome.Outcome = "maybe"
	if unknownOutcome.Valid() {
		t.Fatal("unknown audit outcome was accepted")
	}
}

func TestOnboardingCompletionIsAValidRetainedAuditAction(t *testing.T) {
	event := Event{ID: "event", OwnerUserID: "owner", ActorUserID: "owner", Action: UserOnboardingCompleted, TargetType: "user", TargetID: "owner", Outcome: Succeeded, CorrelationID: "request", OccurredAt: time.Now()}
	if !event.Valid() {
		t.Fatal("onboarding completion audit action was rejected")
	}
}

func TestPathInvitationActionsAreValidRetainedAuditTaxonomy(t *testing.T) {
	for _, action := range []Action{
		PathInvitationCreated,
		PathInvitationListed,
		PathInvitationAccepted,
		PathInvitationRejected,
		PathInvitationCanceled,
	} {
		event := Event{
			ID: "event", OwnerUserID: "owner", ActorUserID: "actor",
			Action: action, TargetType: "path_invitation", TargetID: "invitation",
			Outcome: Succeeded, CorrelationID: "request", OccurredAt: time.Now(),
		}
		if !event.Valid() {
			t.Fatalf("Path invitation audit action %q was rejected", action)
		}
	}
}

func TestPathOwnershipTransferActionsAreValidRetainedAuditTaxonomy(t *testing.T) {
	for _, action := range []Action{
		PathOwnershipTransferCreated,
		PathOwnershipTransferListed,
		PathOwnershipTransferAccepted,
		PathOwnershipTransferDeclined,
		PathOwnershipTransferCanceled,
	} {
		event := Event{
			ID: "event", OwnerUserID: "creator", ActorUserID: "actor",
			Action: action, TargetType: "path_ownership_transfer", TargetID: "transfer",
			Outcome: Succeeded, CorrelationID: "request", OccurredAt: time.Now(),
		}
		if !event.Valid() {
			t.Fatalf("Path ownership transfer audit action %q was rejected", action)
		}
	}
}
