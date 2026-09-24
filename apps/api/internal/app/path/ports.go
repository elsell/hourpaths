package path

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"time"
)

type ProfileReader interface {
	PathCreationProfile(context.Context, string) (PathCreationProfile, error)
	TimeZone(context.Context, string) (string, error)
}

type PathCreationProfile struct {
	ProfileVisibility identity.ProfileVisibility
	FirstDayOfWeek    identity.FirstDayOfWeek
}

type PageRequest struct {
	AfterID      domain.ID
	AfterCreated time.Time
	Snapshot     time.Time
	Limit        int
	Archived     bool
}

type Page struct {
	Items   []domain.Entity
	HasMore bool
}

const UpdateGoalsOperation = "path.goals.update"
const RenameOperation = "path.rename"
const SetVisibilityOperation = "path.visibility.set"
const SetArchiveStateOperation = "path.archive-state.set"
const DeletePathOperation = "path.delete"
const LeavePathOperation = "path.membership.leave"
const RemoveMemberOperation = "path.membership.remove"
const ChangeMemberRoleOperation = "path.membership.role.change"
const SendInvitationOperation = "path.invitation.send"
const AcceptInvitationOperation = "path.invitation.accept"
const RejectInvitationOperation = "path.invitation.reject"
const CancelInvitationOperation = "path.invitation.cancel"
const InitiateOwnershipTransferOperation = "path.ownership-transfer.initiate"
const AcceptOwnershipTransferOperation = "path.ownership-transfer.accept"
const DeclineOwnershipTransferOperation = "path.ownership-transfer.decline"
const CancelOwnershipTransferOperation = "path.ownership-transfer.cancel"

type GoalIntervalProgress struct {
	TargetSeconds      int64
	AccumulatedSeconds int64
	StartedAt          time.Time
	EndedAt            time.Time
}

type UpdateGoalsCommand struct {
	ActorUserID         string
	ParticipantTimeZone string
	ExpectedGoals       domain.GoalConfiguration
	Path                domain.Entity
	ProjectedAt         time.Time
	Idempotency         ports.Idempotency
	Audit               audit.Event
}

type UpdateGoalsResult struct {
	Path               domain.Entity
	AccumulatedSeconds int64
	IntervalProgress   *GoalIntervalProgress
	Replayed           bool
}

type RenameCommand struct {
	ActorUserID  string
	ExpectedName string
	Path         domain.Entity
	Idempotency  ports.Idempotency
	Audit        audit.Event
}

type RenameResult struct {
	Path     domain.Entity
	Replayed bool
}

type VisibilityNotification struct {
	ID, RecipientUserID, ActorUserID string
	PathID                           domain.ID
	Visibility                       string
	CreatedAt                        time.Time
}

type SetVisibilityCommand struct {
	ActorUserID         string
	ExpectedVisibility  string
	Path                domain.Entity
	ChangedAt           time.Time
	Idempotency         ports.Idempotency
	Audit               audit.Event
	Notifications       []VisibilityNotification
	NewID               func() string
	AuthorizationWorker string
	AuthorizationLease  time.Duration
}

type SetVisibilityResult struct {
	Path                 domain.Entity
	AuthorizationChanges []ports.AuthorizationChange
	Replayed             bool
}

type SetArchiveStateCommand struct {
	ActorUserID      string
	ExpectedArchived bool
	Archived         bool
	Path             domain.Entity
	ChangedAt        time.Time
	Idempotency      ports.Idempotency
	Audit            audit.Event
	NewActivityID    func() string
}

type SetArchiveStateResult struct {
	Path     domain.Entity
	Replayed bool
}

type DeletePathCommand struct {
	ActorUserID         string
	PathID              domain.ID
	ExpectedName        string
	DeletedAt           time.Time
	Idempotency         ports.Idempotency
	Audit               audit.Event
	NewID               func() string
	AuthorizationWorker string
	AuthorizationLease  time.Duration
}

type DeletePathResult struct {
	PathID               domain.ID
	Deleted              bool
	AuthorizationChanges []ports.AuthorizationChange
	Replayed             bool
}

type LeavePathCommand struct {
	ActorUserID         string
	PathID              domain.ID
	LeftAt              time.Time
	RetainActivity      bool
	Idempotency         ports.Idempotency
	Audit               audit.Event
	NewID               func() string
	AuthorizationWorker string
	AuthorizationLease  time.Duration
}

type LeavePathResult struct {
	PathID               domain.ID
	Left                 bool
	ActivityRetained     bool
	AuthorizationChanges []ports.AuthorizationChange
	Replayed             bool
}

type MemberRemovalReview struct {
	UserID, Username, DisplayName string
	Role                          domain.MembershipRole
	SessionCount                  int64
	TotalTrackedSeconds           int64
	RunningTimer                  bool
}

type MemberRemovalQuery struct {
	ActorUserID, TargetUserID string
	PathID                    domain.ID
}

type RemoveMemberCommand struct {
	ActorUserID, TargetUserID string
	PathID                    domain.ID
	ExpectedRole              domain.MembershipRole
	RemovedAt                 time.Time
	Idempotency               ports.Idempotency
	Audit                     audit.Event
	NewID                     func() string
	AuthorizationWorker       string
	AuthorizationLease        time.Duration
	Notification              MemberAccessNotification
}

type RemoveMemberResult struct {
	PathID               domain.ID
	UserID               string
	Removed              bool
	ActivityDeleted      bool
	AuthorizationChanges []ports.AuthorizationChange
	Replayed             bool
}

type MemberAccessNotification struct {
	ID, RecipientUserID, ActorUserID string
	PathID                           domain.ID
	Kind                             InvitationNotificationKind
	Role                             domain.MembershipRole
	CreatedAt                        time.Time
}

type ChangeMemberRoleCommand struct {
	ActorUserID, TargetUserID string
	PathID                    domain.ID
	ExpectedRole, Role        domain.MembershipRole
	ChangedAt                 time.Time
	Idempotency               ports.Idempotency
	Audit                     audit.Event
	Notification              MemberAccessNotification
	NewID                     func() string
	AuthorizationWorker       string
	AuthorizationLease        time.Duration
}

type ChangeMemberRoleResult struct {
	PathID               domain.ID
	UserID               string
	Role                 domain.MembershipRole
	ActivityDeleted      bool
	AuthorizationChanges []ports.AuthorizationChange
	Replayed             bool
}

type MemberRemovalRepository interface {
	ListMembers(context.Context, MemberListQuery, MemberPageRequest) (MemberPage, error)
	ReviewMemberRemoval(context.Context, MemberRemovalQuery) (MemberRemovalReview, error)
	RemoveMemberReplay(context.Context, string, domain.ID, string, ports.Idempotency) (bool, error)
	RemoveMember(context.Context, RemoveMemberCommand) (RemoveMemberResult, error)
	ChangeMemberRoleReplay(context.Context, string, domain.ID, string, ports.Idempotency) (bool, error)
	ChangeMemberRole(context.Context, ChangeMemberRoleCommand) (ChangeMemberRoleResult, error)
}

type Member struct {
	UserID, Username, DisplayName, Role                 string
	SessionCount, TotalTrackedSeconds                   int64
	IntervalProgress, OverallProgress                   *GoalProgress
	BlockedByViewer, CanRemove, CanChangeRole, CanLeave bool
	CanGrantAdministrator, CanRevokeAdministrator       bool
	CanStepDownAdministrator                            bool
	CreatedAt                                           time.Time
}

type GoalProgress struct {
	AccumulatedSeconds int64
	TargetSeconds      int64
}
type MemberListQuery struct {
	ActorUserID string
	PathID      domain.ID
}
type MemberPageRequest struct {
	AfterUserID                   string
	ExpectedProjectionFingerprint string
	AfterCreated, Snapshot        time.Time
	Limit                         int
}
type MemberPage struct {
	Items                 []Member
	ProjectionFingerprint string
	HasMore               bool
}

type InvitationRecipient struct {
	UserID            string
	Username          string
	DisplayName       string
	ProfileVisibility identity.ProfileVisibility
}

type InvitationRecipientReview struct {
	UserID      string
	Username    string
	DisplayName string
}

type InvitationDirectory interface {
	ActiveByExactUsername(context.Context, string) (InvitationRecipient, error)
}

type InvitationNotification struct {
	ID, RecipientUserID, ActorUserID string
	PathID                           domain.ID
	InvitationID                     domain.InvitationID
	OfferedRole                      domain.MembershipRole
	Actionable                       bool
	CreatedAt                        time.Time
}

type InvitationNotificationKind string

const (
	NotificationPathInvitationReceived        InvitationNotificationKind = "path_invitation_received"
	NotificationPathInvitationAccepted        InvitationNotificationKind = "path_invitation_accepted"
	NotificationPathOwnershipTransferReceived InvitationNotificationKind = "path_ownership_transfer_received"
	NotificationPathOwnershipTransferAccepted InvitationNotificationKind = "path_ownership_transfer_accepted"
	NotificationPathOwnershipTransferDeclined InvitationNotificationKind = "path_ownership_transfer_declined"
	NotificationPathOwnershipTransferCanceled InvitationNotificationKind = "path_ownership_transfer_canceled"
	NotificationPathDeleted                   InvitationNotificationKind = "path_deleted"
	NotificationPathMemberLeft                InvitationNotificationKind = "path_member_left"
	NotificationPathMemberRemoved             InvitationNotificationKind = "path_member_removed"
	NotificationPathMemberRoleChanged         InvitationNotificationKind = "path_member_role_changed"
	NotificationPathVisibilityChanged         InvitationNotificationKind = "path_visibility_changed"
	NotificationNewFollower                   InvitationNotificationKind = "new_follower"
	NotificationFollowRequestReceived         InvitationNotificationKind = "follow_request_received"
	NotificationFollowRequestAccepted         InvitationNotificationKind = "follow_request_accepted"
	NotificationPracticeReaction              InvitationNotificationKind = "practice_reaction"
	NotificationPracticeComment               InvitationNotificationKind = "practice_comment"
	NotificationCommentHeart                  InvitationNotificationKind = "comment_heart"
	NotificationNudgeReceived                 InvitationNotificationKind = "nudge_received"
)

type NotificationPresentation string

const (
	NotificationActionable    NotificationPresentation = "actionable"
	NotificationInformational NotificationPresentation = "informational"
)

type InteractionDisabledReason string

const (
	InteractionDisabledComments  InteractionDisabledReason = "comments"
	InteractionDisabledReactions InteractionDisabledReason = "reactions"
)

func (reason InteractionDisabledReason) Valid() bool {
	return reason == InteractionDisabledComments || reason == InteractionDisabledReactions
}

type InvitationNotificationProjection struct {
	ID                  string
	Kind                InvitationNotificationKind
	Presentation        NotificationPresentation
	Read                bool
	CreatedAt           time.Time
	Actor               InvitationPublicIdentity
	PathID              domain.ID
	PathName            string
	PathVisibility      string
	InvitationID        domain.InvitationID
	OfferedRole         domain.MembershipRole
	OwnershipTransferID domain.OwnershipTransferID
	FollowRequestID     string
	SocialFeedEventID   string
	Reaction            socialdomain.Reaction
	CommentID           string
	NudgeContent        socialdomain.NudgeContent
	InteractionDisabled InteractionDisabledReason
}

type NotificationPageRequest struct {
	AfterID      string
	AfterCreated time.Time
	Snapshot     time.Time
	Limit        int
}

type NotificationPage struct {
	Items       []InvitationNotificationProjection
	HasMore     bool
	UnreadCount int64
}

type NotificationMutationCommand struct {
	RecipientUserID string
	NotificationID  string
	ChangedAt       time.Time
	Audit           audit.Event
}

type NotificationMutationResult struct {
	UnreadCount int64
}

type InvitationPageRequest struct {
	AfterID      domain.InvitationID
	AfterCreated time.Time
	Snapshot     time.Time
	Limit        int
}

type InvitationPage struct {
	Items   []PendingInvitationCandidate
	HasMore bool
}

type ManagedInvitation struct {
	Invitation domain.Invitation
	Inviter    InvitationPublicIdentity
	Recipient  InvitationPublicIdentity
}

type ManagedInvitationPage struct {
	Items   []ManagedInvitation
	HasMore bool
}

type InvitationPublicIdentity struct {
	UserID      string
	Username    string
	DisplayName string
}

type PendingInvitation struct {
	Invitation domain.Invitation
	PathName   string
	Inviter    InvitationPublicIdentity
	Warning    *InvitationWarningContext
}

type PendingInvitationCandidate struct {
	PendingInvitation PendingInvitation
	WarningInput      InvitationWarningInput
}

type SendInvitationCommand struct {
	Invitation                domain.Invitation
	ExpectedRecipientUserID   string
	ExpectedRecipientUsername string
	Notification              InvitationNotification
	Idempotency               ports.Idempotency
	Audit                     audit.Event
}

type SendInvitationResult struct {
	Invitation domain.Invitation
	Replayed   bool
}

type InvitationDecision struct {
	Invitation          domain.Invitation
	Path                domain.Entity
	RecipientVisibility identity.ProfileVisibility
	HasRetainedActivity bool
	Replay              *AcceptInvitationResult
}

type InvitationWarningInput struct {
	RecipientVisibility identity.ProfileVisibility
	PathVisibility      string
	OfferedRole         domain.MembershipRole
	HasRetainedActivity bool
}

type InvitationWarningContext struct {
	PathVisibility      string
	HasRetainedActivity bool
}

type InvitationWarningDecision struct {
	Required bool
	Context  InvitationWarningContext
}

type InvitationWarningAcknowledgement struct {
	PathVisibility string
}

type InvitationWarningPolicy interface {
	Evaluate(context.Context, InvitationWarningInput) (InvitationWarningDecision, error)
}

type AuthorizationChangeState string

const (
	AuthorizationChangePending      AuthorizationChangeState = "pending"
	AuthorizationChangeLocked       AuthorizationChangeState = "locked"
	AuthorizationChangeCompleted    AuthorizationChangeState = "completed"
	AuthorizationChangeDeadLettered AuthorizationChangeState = "dead_lettered"
)

type AuthorizationChangeStatusReader interface {
	AuthorizationChangeState(context.Context, string) (AuthorizationChangeState, error)
}

type AcceptInvitationCommand struct {
	Invitation          domain.Invitation
	Acknowledgement     InvitationWarningAcknowledgement
	AuthorizationChange ports.AuthorizationChange
	Notification        InvitationNotification
	Idempotency         ports.Idempotency
	Audit               audit.Event
}

type AcceptInvitationResult struct {
	Invitation          domain.Invitation
	AuthorizationChange ports.AuthorizationChange
	Replayed            bool
}

type RejectionDecision struct {
	Invitation  domain.Invitation
	OwnerUserID string
	Replay      *RejectInvitationResult
}

type RejectInvitationCommand struct {
	Invitation  domain.Invitation
	Idempotency ports.Idempotency
	Audit       audit.Event
}

type RejectInvitationResult struct {
	Invitation  domain.Invitation
	UnreadCount int64
	Replayed    bool
}

type CancellationDecision struct {
	Invitation domain.Invitation
	Path       domain.Entity
	Replay     *CancelInvitationResult
}

type CancelInvitationCommand struct {
	ActorUserID string
	Invitation  domain.Invitation
	Idempotency ports.Idempotency
	Audit       audit.Event
}

type CancelInvitationResult struct {
	Invitation domain.Invitation
	Replayed   bool
}

type InvitationRepository interface {
	Send(context.Context, SendInvitationCommand) (SendInvitationResult, error)
	ListPending(context.Context, string, InvitationPageRequest) (InvitationPage, error)
	ListNotifications(context.Context, string, NotificationPageRequest) (NotificationPage, error)
	GetNotification(context.Context, string, string) (InvitationNotificationProjection, error)
	MarkNotificationRead(context.Context, NotificationMutationCommand) (NotificationMutationResult, error)
	DeleteNotification(context.Context, NotificationMutationCommand) (NotificationMutationResult, error)
	MarkAllNotificationsRead(context.Context, NotificationMutationCommand) (NotificationMutationResult, error)
	AcceptanceDecision(context.Context, string, domain.InvitationID, ports.Idempotency) (InvitationDecision, error)
	Accept(context.Context, AcceptInvitationCommand) (AcceptInvitationResult, error)
	RejectionDecision(context.Context, string, domain.InvitationID, ports.Idempotency) (RejectionDecision, error)
	Reject(context.Context, RejectInvitationCommand) (RejectInvitationResult, error)
}

type InvitationManagementRepository interface {
	ListManagedPending(context.Context, string, domain.ID, InvitationPageRequest) (ManagedInvitationPage, error)
	CancellationDecision(context.Context, string, domain.ID, domain.InvitationID, ports.Idempotency) (CancellationDecision, error)
	Cancel(context.Context, CancelInvitationCommand) (CancelInvitationResult, error)
}

// OwnershipTransferCandidate is an existing participant who may receive Path
// ownership. Repository implementations must exclude the creator and
// supporters; administrators remain eligible because they are participants.
type OwnershipTransferCandidate struct {
	UserID        string
	Username      string
	DisplayName   string
	Administrator bool
	CreatedAt     time.Time
}

type OwnershipTransferPublicIdentity struct {
	UserID      string
	Username    string
	DisplayName string
}

type OwnershipTransferReview struct {
	Recipient        OwnershipTransferPublicIdentity
	ReviewedAt       time.Time
	ExpiresAt        time.Time
	ReservationToken string
	ViewerTimeZone   string
}

// OwnershipTransferQuery is always scoped to the authenticated actor. For a
// pending-transfer read, repositories expose the request only to its current
// creator or selected recipient and otherwise return the opaque unavailable
// error.
type OwnershipTransferQuery struct {
	ActorUserID string
	PathID      domain.ID
	TransferID  domain.OwnershipTransferID
	Idempotency ports.Idempotency
}

// OwnershipTransferDecision is the transaction input used to revalidate a
// pending request. Path ownership, active state, and recipient participation
// are checked again by Accept in the same transaction that mutates roles.
type OwnershipTransferDecision struct {
	Transfer               domain.OwnershipTransfer
	Path                   domain.Entity
	RecipientIsParticipant bool
	Replay                 *OwnershipTransferResult
	Counterpart            OwnershipTransferPublicIdentity
	CounterpartRole        string
	ViewerTimeZone         string
}

type InitiateOwnershipTransferCommand struct {
	Transfer                    domain.OwnershipTransfer
	ExpectedCreatorUserID       string
	RequireActivePath           bool
	RequireRecipientParticipant bool
	RequireNoPendingForPath     bool
	Notification                OwnershipTransferNotification
	Idempotency                 ports.Idempotency
	Audit                       audit.Event
}

type AcceptOwnershipTransferCommand struct {
	Transfer                           domain.OwnershipTransfer
	ExpectedCreatorUserID              string
	RequireActivePath                  bool
	RequireRecipientParticipant        bool
	PromoteRecipientToCreator          bool
	RetainFormerCreatorAsAdministrator bool
	PreserveMembershipAndActivity      bool
	Notification                       OwnershipTransferNotification
	Idempotency                        ports.Idempotency
	Audit                              audit.Event
}

type DeclineOwnershipTransferCommand struct {
	Transfer     domain.OwnershipTransfer
	Notification OwnershipTransferNotification
	Idempotency  ports.Idempotency
	Audit        audit.Event
}

type CancelOwnershipTransferCommand struct {
	Transfer     domain.OwnershipTransfer
	Notification OwnershipTransferNotification
	Idempotency  ports.Idempotency
	Audit        audit.Event
}

// OwnershipTransferNotification is persisted in the same transaction as its
// transfer lifecycle event. PushRequested controls whether the durable push
// outbox is populated; decline and cancellation remain in-application only.
type OwnershipTransferNotification struct {
	ID, RecipientUserID, ActorUserID string
	PathID                           domain.ID
	OwnershipTransferID              domain.OwnershipTransferID
	Kind                             InvitationNotificationKind
	Presentation                     NotificationPresentation
	PushRequested                    bool
	CreatedAt                        time.Time
}

type OwnershipTransferResult struct {
	Transfer            domain.OwnershipTransfer
	Path                domain.Entity
	RelationshipUpdates []ports.RelationshipUpdate
	AuthorizationBatch  ports.AuthorizationBatch
	Replayed            bool
	Counterpart         OwnershipTransferPublicIdentity
	CounterpartRole     string
	ViewerTimeZone      string
}

// OwnershipTransferRepository owns the atomic single-pending invariant and
// all role changes. Accept must atomically revalidate the expected creator,
// active Path, and recipient participation before replacing the creator and
// retaining the former creator as an administrator.
type OwnershipTransferRepository interface {
	ListCandidates(context.Context, OwnershipTransferQuery, OwnershipTransferCandidatePageRequest) (OwnershipTransferCandidatePage, error)
	Candidate(context.Context, OwnershipTransferQuery, string) (OwnershipTransferCandidate, error)
	GetPending(context.Context, OwnershipTransferQuery) (OwnershipTransferDecision, error)
	Initiate(context.Context, InitiateOwnershipTransferCommand) (OwnershipTransferResult, error)
	Accept(context.Context, AcceptOwnershipTransferCommand) (OwnershipTransferResult, error)
	Decline(context.Context, DeclineOwnershipTransferCommand) (OwnershipTransferResult, error)
	Cancel(context.Context, CancelOwnershipTransferCommand) (OwnershipTransferResult, error)
}

type Repository interface {
	HomePreferencesRepository
	InvitationDirectory
	InvitationRepository
	AuthorizationChangeStatusReader
	Create(context.Context, domain.Entity, ports.AuthorizationChange, ports.Idempotency, audit.Event) (domain.Entity, bool, error)
	List(context.Context, string, PageRequest) (Page, error)
	Get(context.Context, string, domain.ID) (domain.Entity, error)
	Update(context.Context, string, domain.Entity, audit.Event) error
	Rename(context.Context, RenameCommand) (RenameResult, error)
	SetVisibility(context.Context, SetVisibilityCommand) (SetVisibilityResult, error)
	SetVisibilityReplay(context.Context, string, domain.ID, ports.Idempotency) (*SetVisibilityResult, error)
	UpdateGoals(context.Context, UpdateGoalsCommand) (UpdateGoalsResult, error)
	SetArchiveState(context.Context, SetArchiveStateCommand) (SetArchiveStateResult, error)
	DeletionReplay(context.Context, string, domain.ID, ports.Idempotency) (bool, error)
	DeletePath(context.Context, DeletePathCommand) (DeletePathResult, error)
	LeavePathReplay(context.Context, string, domain.ID, ports.Idempotency) (bool, error)
	LeavePath(context.Context, LeavePathCommand) (LeavePathResult, error)
}
