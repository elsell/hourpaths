package audit

import "time"

type Action string

const (
	UserProvisioned                 Action = "user.provisioned"
	UserDeactivated                 Action = "user.deactivated"
	UserProfileSynchronized         Action = "user.profile_synchronized"
	UserViewed                      Action = "user.viewed"
	UserOnboardingCompleted         Action = "user.onboarding_completed"
	SessionCreated                  Action = "session.created"
	SessionRevoked                  Action = "session.revoked"
	ResourceListed                  Action = "resource.listed"
	ResourceViewed                  Action = "resource.viewed"
	ResourceCreated                 Action = "resource.created"
	ResourceUpdated                 Action = "resource.updated"
	ResourceDeleted                 Action = "resource.deleted"
	ResourceAccessDenied            Action = "resource.access_denied"
	AuthorizationApplied            Action = "authorization.relationship_applied"
	AuthorizationFailed             Action = "authorization.relationship_failed"
	AuthorizationDeadLettersListed  Action = "authorization.dead_letters_listed"
	AuthorizationDeadLetterRequeued Action = "authorization.dead_letter_requeued"
	InvitationCreated               Action = "invitation.created"
	InvitationListed                Action = "invitation.listed"
	InvitationRevoked               Action = "invitation.revoked"
	InvitationConsumed              Action = "invitation.consumed"
	DuplicateEmailRecoveryDeclined  Action = "duplicate_email_recovery.declined"
	ActivityTimerStarted            Action = "activity.timer_started"
	ActivityTimerStopped            Action = "activity.timer_stopped"
	PathInvitationCreated           Action = "path_invitation.created"
	PathInvitationListed            Action = "path_invitation.listed"
	PathInvitationAccepted          Action = "path_invitation.accepted"
	PathInvitationRejected          Action = "path_invitation.rejected"
	PathInvitationCanceled          Action = "path_invitation.canceled"
	PathVisibilityChanged           Action = "path.visibility_changed"
	PathOwnershipTransferCreated    Action = "path_ownership_transfer.created"
	PathOwnershipTransferListed     Action = "path_ownership_transfer.listed"
	PathOwnershipTransferAccepted   Action = "path_ownership_transfer.accepted"
	PathOwnershipTransferDeclined   Action = "path_ownership_transfer.declined"
	PathOwnershipTransferCanceled   Action = "path_ownership_transfer.canceled"
)

type Outcome string

const (
	Succeeded Outcome = "succeeded"
	Denied    Outcome = "denied"
	Failed    Outcome = "failed"
)

type Event struct {
	ID, OwnerUserID, ActorUserID string
	Action                       Action
	TargetType, TargetID         string
	Outcome                      Outcome
	CorrelationID                string
	OccurredAt                   time.Time
}

func (e Event) Valid() bool {
	knownAction := e.Action == UserProvisioned || e.Action == UserDeactivated || e.Action == UserProfileSynchronized || e.Action == UserViewed || e.Action == UserOnboardingCompleted || e.Action == SessionCreated || e.Action == SessionRevoked || e.Action == ResourceListed || e.Action == ResourceViewed || e.Action == ResourceCreated || e.Action == ResourceUpdated || e.Action == ResourceDeleted || e.Action == ResourceAccessDenied || e.Action == AuthorizationApplied || e.Action == AuthorizationFailed || e.Action == AuthorizationDeadLettersListed || e.Action == AuthorizationDeadLetterRequeued || e.Action == InvitationCreated || e.Action == InvitationListed || e.Action == InvitationRevoked || e.Action == InvitationConsumed || e.Action == DuplicateEmailRecoveryDeclined || e.Action == ActivityTimerStarted || e.Action == ActivityTimerStopped || e.Action == PathInvitationCreated || e.Action == PathInvitationListed || e.Action == PathInvitationAccepted || e.Action == PathInvitationRejected || e.Action == PathInvitationCanceled || e.Action == PathVisibilityChanged || e.Action == PathOwnershipTransferCreated || e.Action == PathOwnershipTransferListed || e.Action == PathOwnershipTransferAccepted || e.Action == PathOwnershipTransferDeclined || e.Action == PathOwnershipTransferCanceled
	knownOutcome := e.Outcome == Succeeded || e.Outcome == Denied || e.Outcome == Failed
	return e.ID != "" && e.OwnerUserID != "" && e.ActorUserID != "" && knownAction &&
		e.TargetType != "" && e.TargetID != "" && knownOutcome && e.CorrelationID != "" && !e.OccurredAt.IsZero()
}

type Page struct {
	Events  []Event
	HasMore bool
}
