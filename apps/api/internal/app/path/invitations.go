package path

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

var (
	errInvalidInvitationDependencies = errors.New("path invitation service dependencies are invalid")
	ErrInvitationWarningRequired     = errors.New("path invitation visibility warning is required")
)

type InvitationWarningRequiredError struct {
	Context InvitationWarningContext
}

func (err *InvitationWarningRequiredError) Error() string {
	return ErrInvitationWarningRequired.Error()
}

func (err *InvitationWarningRequiredError) Unwrap() error {
	return ErrInvitationWarningRequired
}

type InvitationDependencies struct {
	Auth                    ports.Authenticator
	Paths                   Repository
	Directory               InvitationDirectory
	Authorizer              ports.Authorizer
	Invitations             InvitationRepository
	InvitationManagement    InvitationManagementRepository
	WarningPolicy           InvitationWarningPolicy
	Audits                  ports.Audits
	AuditRateLimiter        ports.AuditRateLimiter
	Clock                   ports.Clock
	NewID                   func() string
	AuthorizationOutbox     ports.AuthorizationOutbox
	AuthorizationStatus     AuthorizationChangeStatusReader
	AuthorizationSerializer ports.AuthorizationSerializer
	AuthorizationWorker     string
	AuthorizationLease      time.Duration
	CursorSigningKey        []byte
}

type InvitationService struct{ InvitationDependencies }

func NewInvitationService(dependencies InvitationDependencies) *InvitationService {
	return &InvitationService{InvitationDependencies: dependencies}
}

func (service *InvitationService) authenticate(ctx context.Context, authorization string) (ports.Principal, error) {
	if service.Auth == nil {
		return ports.Principal{}, errInvalidInvitationDependencies
	}
	principal, err := service.Auth.Authenticate(ctx, authorization)
	if err != nil {
		return ports.Principal{}, err
	}
	if principal.UserID == "" || len(principal.Scopes) != 1 || principal.Scopes[0] != "api:user" {
		return ports.Principal{}, platformapp.ErrUnauthenticated
	}
	return principal, nil
}

func (service *InvitationService) ReviewRecipient(
	ctx context.Context,
	authorization string,
	pathID domain.ID,
	exactUsername string,
) (InvitationRecipientReview, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return InvitationRecipientReview{}, err
	}
	if pathID == "" || exactUsername == "" || strings.TrimSpace(exactUsername) != exactUsername {
		return InvitationRecipientReview{}, ports.ErrInvalidArgument
	}
	if service.Paths == nil || service.Directory == nil || service.Authorizer == nil ||
		service.Audits == nil || service.AuditRateLimiter == nil || service.Clock == nil {
		return InvitationRecipientReview{}, errInvalidInvitationDependencies
	}
	now := service.Clock.Now().UTC()
	if now.IsZero() {
		return InvitationRecipientReview{}, errInvalidInvitationDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return InvitationRecipientReview{}, platformapp.ErrRateLimited
	}
	allowed, err := service.Authorizer.Check(ctx, "path", string(pathID), "manage_members", principal.UserID)
	if err != nil {
		return InvitationRecipientReview{}, err
	}
	if !allowed {
		event := shared.NewAuditEvent(
			ctx, service.Clock, principal.UserID, principal.UserID,
			audit.ResourceAccessDenied, "path", string(pathID), audit.Denied,
		)
		event.OccurredAt = now
		if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
			return InvitationRecipientReview{}, err
		}
		return InvitationRecipientReview{}, platformapp.ErrForbidden
	}
	path, err := service.Paths.Get(ctx, principal.UserID, pathID)
	if err != nil {
		return InvitationRecipientReview{}, err
	}
	if path.ID != pathID || !validCreatedPath(path, path.OwnerUserID) {
		return InvitationRecipientReview{}, errInvalidInvitationDependencies
	}
	if path.Archived() {
		return InvitationRecipientReview{}, ports.ErrConflict
	}
	recipient, err := service.Directory.ActiveByExactUsername(ctx, exactUsername)
	if err != nil {
		return InvitationRecipientReview{}, err
	}
	if recipient.UserID == "" || !strings.EqualFold(recipient.Username, exactUsername) ||
		strings.TrimSpace(recipient.DisplayName) == "" ||
		identity.ValidateProfileVisibility(recipient.ProfileVisibility) != nil {
		return InvitationRecipientReview{}, errInvalidInvitationDependencies
	}
	event := shared.NewAuditEvent(
		ctx, service.Clock, recipient.UserID, principal.UserID,
		audit.UserViewed, "user", recipient.UserID, audit.Succeeded,
	)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return InvitationRecipientReview{}, err
	}
	return InvitationRecipientReview{
		UserID: recipient.UserID, Username: recipient.Username, DisplayName: recipient.DisplayName,
	}, nil
}

func (service *InvitationService) Send(
	ctx context.Context,
	authorization, idempotencyKey string,
	pathID domain.ID,
	exactUsername string,
	expectedRecipientUserID string,
	offeredRole domain.MembershipRole,
) (SendInvitationResult, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return SendInvitationResult{}, err
	}
	if pathID == "" || exactUsername == "" || strings.TrimSpace(exactUsername) != exactUsername ||
		expectedRecipientUserID == "" || strings.TrimSpace(expectedRecipientUserID) != expectedRecipientUserID ||
		!offeredRole.Valid() || !validIdempotencyKey(idempotencyKey) {
		return SendInvitationResult{}, ports.ErrInvalidArgument
	}
	if !service.validSendDependencies() {
		return SendInvitationResult{}, errInvalidInvitationDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return SendInvitationResult{}, errInvalidInvitationDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return SendInvitationResult{}, platformapp.ErrRateLimited
	}
	allowed, err := service.Authorizer.Check(ctx, "path", string(pathID), "manage_members", principal.UserID)
	if err != nil {
		return SendInvitationResult{}, err
	}
	if !allowed {
		event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(pathID), audit.Denied)
		event.OccurredAt = now
		if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
			return SendInvitationResult{}, err
		}
		return SendInvitationResult{}, platformapp.ErrForbidden
	}
	path, err := service.Paths.Get(ctx, principal.UserID, pathID)
	if err != nil {
		return SendInvitationResult{}, err
	}
	if path.ID != pathID || !validCreatedPath(path, path.OwnerUserID) {
		return SendInvitationResult{}, errInvalidInvitationDependencies
	}
	if path.Archived() {
		return SendInvitationResult{}, ports.ErrConflict
	}
	recipient, err := service.Directory.ActiveByExactUsername(ctx, exactUsername)
	if err != nil {
		return SendInvitationResult{}, err
	}
	if recipient.UserID == "" || !strings.EqualFold(recipient.Username, exactUsername) ||
		identity.ValidateProfileVisibility(recipient.ProfileVisibility) != nil {
		return SendInvitationResult{}, errInvalidInvitationDependencies
	}
	if recipient.UserID != expectedRecipientUserID {
		return SendInvitationResult{}, ports.ErrNotFound
	}

	invitation := domain.Invitation{
		ID: domain.InvitationID(service.NewID()), PathID: pathID, InviterUserID: principal.UserID,
		RecipientUserID: recipient.UserID, OfferedRole: offeredRole, CreatedAt: now,
	}
	if invitation.ID == "" {
		return SendInvitationResult{}, errInvalidInvitationDependencies
	}
	// Self, current-member, and duplicate-pending conflicts belong to the
	// transactional repository so concurrent requests cannot bypass them.
	notification := InvitationNotification{
		ID: service.NewID(), RecipientUserID: recipient.UserID, ActorUserID: principal.UserID,
		PathID: pathID, InvitationID: invitation.ID, OfferedRole: offeredRole,
		Actionable: true, CreatedAt: now,
	}
	if notification.ID == "" {
		return SendInvitationResult{}, errInvalidInvitationDependencies
	}
	digest := canonicalSendInvitationRequestHash(pathID, exactUsername, expectedRecipientUserID, offeredRole)
	event := shared.NewAuditEvent(ctx, service.Clock, path.OwnerUserID, principal.UserID, audit.PathInvitationCreated, "path_invitation", string(invitation.ID), audit.Succeeded)
	event.OccurredAt = now
	result, err := service.Invitations.Send(ctx, SendInvitationCommand{
		Invitation: invitation, ExpectedRecipientUserID: expectedRecipientUserID,
		ExpectedRecipientUsername: exactUsername, Notification: notification,
		Idempotency: ports.Idempotency{
			PrincipalID: principal.UserID, Operation: SendInvitationOperation,
			Key: idempotencyKey, RequestHash: digest[:],
		},
		Audit: event,
	})
	if err != nil {
		return SendInvitationResult{}, err
	}
	if !validSentInvitation(result.Invitation, pathID, principal.UserID, recipient.UserID, offeredRole) {
		return SendInvitationResult{}, errInvalidInvitationDependencies
	}
	return result, nil
}

func (service *InvitationService) validSendDependencies() bool {
	return service.Paths != nil && service.Directory != nil && service.Authorizer != nil &&
		service.Invitations != nil && service.Audits != nil && service.AuditRateLimiter != nil &&
		service.Clock != nil && service.NewID != nil
}

func canonicalSendInvitationRequestHash(pathID domain.ID, username, expectedRecipientUserID string, role domain.MembershipRole) [sha256.Size]byte {
	digest := sha256.New()
	writeHashString(digest, string(pathID))
	writeHashString(digest, username)
	writeHashString(digest, expectedRecipientUserID)
	writeHashString(digest, string(role))
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func validSentInvitation(invitation domain.Invitation, pathID domain.ID, inviter, recipient string, role domain.MembershipRole) bool {
	if invitation.PathID != pathID || invitation.InviterUserID != inviter ||
		invitation.RecipientUserID != recipient || invitation.OfferedRole != role || !invitation.Pending() {
		return false
	}
	_, err := domain.NewInvitation(
		invitation.ID, invitation.PathID, invitation.InviterUserID, invitation.RecipientUserID,
		invitation.OfferedRole, invitation.CreatedAt,
	)
	return err == nil
}

func (service *InvitationService) ListPending(
	ctx context.Context,
	authorization string,
	cursor string,
	limit int,
) ([]PendingInvitation, string, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	if service.Invitations == nil || service.WarningPolicy == nil || service.Audits == nil ||
		service.AuditRateLimiter == nil || service.Clock == nil ||
		len(service.CursorSigningKey) < 32 {
		return nil, "", errInvalidInvitationDependencies
	}
	now := service.Clock.Now().UTC()
	if now.IsZero() {
		return nil, "", errInvalidInvitationDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return nil, "", platformapp.ErrRateLimited
	}
	request := InvitationPageRequest{Limit: limit, Snapshot: now}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || payload.Owner != principal.UserID ||
			payload.Domain != "path-invitation" || payload.Snapshot.After(now) {
			return nil, "", ports.ErrInvalidArgument
		}
		request.AfterID = domain.InvitationID(payload.AfterID)
		request.AfterCreated = payload.AfterCreated
		request.Snapshot = payload.Snapshot
	}
	page, err := service.Invitations.ListPending(ctx, principal.UserID, request)
	if err != nil {
		return nil, "", err
	}
	if !validPendingInvitationPage(page, principal.UserID, limit) {
		return nil, "", errInvalidInvitationDependencies
	}
	nextCursor := ""
	if page.HasMore {
		last := page.Items[len(page.Items)-1].PendingInvitation
		nextCursor, err = shared.EncodeCursor(service.CursorSigningKey, shared.CursorPayload{
			Version: 1, Owner: principal.UserID, Domain: "path-invitation",
			AfterID: string(last.Invitation.ID), AfterCreated: last.Invitation.CreatedAt, Snapshot: request.Snapshot,
		})
		if err != nil {
			return nil, "", err
		}
	}
	items := make([]PendingInvitation, len(page.Items))
	for index := range page.Items {
		candidate := page.Items[index]
		warning, warningErr := service.WarningPolicy.Evaluate(ctx, candidate.WarningInput)
		if warningErr != nil {
			return nil, "", warningErr
		}
		items[index] = candidate.PendingInvitation
		if warning.Required {
			warningContext := warning.Context
			items[index].Warning = &warningContext
		}
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.PathInvitationListed, "path_invitation", "pending", audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	return items, nextCursor, nil
}

func validPendingInvitationPage(page InvitationPage, recipient string, limit int) bool {
	if len(page.Items) > limit || (page.HasMore && len(page.Items) == 0) {
		return false
	}
	for _, candidate := range page.Items {
		pending := candidate.PendingInvitation
		invitation := pending.Invitation
		if invitation.RecipientUserID != recipient || !invitation.Pending() ||
			!validSentInvitation(invitation, invitation.PathID, invitation.InviterUserID, recipient, invitation.OfferedRole) ||
			strings.TrimSpace(pending.PathName) == "" ||
			pending.PathName != strings.TrimSpace(pending.PathName) ||
			pending.Inviter.UserID != invitation.InviterUserID ||
			strings.TrimSpace(pending.Inviter.UserID) == "" ||
			strings.TrimSpace(pending.Inviter.Username) == "" ||
			pending.Inviter.Username != strings.TrimSpace(pending.Inviter.Username) ||
			strings.TrimSpace(pending.Inviter.DisplayName) == "" ||
			pending.Inviter.DisplayName != strings.TrimSpace(pending.Inviter.DisplayName) ||
			pending.Warning != nil ||
			candidate.WarningInput.OfferedRole != invitation.OfferedRole ||
			identity.ValidateProfileVisibility(candidate.WarningInput.RecipientVisibility) != nil ||
			(candidate.WarningInput.PathVisibility != "private" &&
				candidate.WarningInput.PathVisibility != "followers" &&
				candidate.WarningInput.PathVisibility != "public") {
			return false
		}
	}
	return true
}

func (service *InvitationService) ListNotifications(
	ctx context.Context,
	authorization string,
	cursor string,
	limit int,
) ([]InvitationNotificationProjection, string, int64, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", 0, err
	}
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 {
		return nil, "", 0, ports.ErrInvalidArgument
	}
	if service.Invitations == nil || service.Audits == nil ||
		service.AuditRateLimiter == nil || service.Clock == nil ||
		len(service.CursorSigningKey) < 32 {
		return nil, "", 0, errInvalidInvitationDependencies
	}
	now := service.Clock.Now().UTC()
	if now.IsZero() {
		return nil, "", 0, errInvalidInvitationDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return nil, "", 0, platformapp.ErrRateLimited
	}
	request := NotificationPageRequest{Limit: limit, Snapshot: now}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || payload.Owner != principal.UserID ||
			payload.Domain != "path-notification" || payload.Snapshot.After(now) {
			return nil, "", 0, ports.ErrInvalidArgument
		}
		request.AfterID = payload.AfterID
		request.AfterCreated = payload.AfterCreated
		request.Snapshot = payload.Snapshot
	}
	page, err := service.Invitations.ListNotifications(ctx, principal.UserID, request)
	if err != nil {
		return nil, "", 0, err
	}
	if !validNotificationPage(page, limit, request.Snapshot) {
		return nil, "", 0, errInvalidInvitationDependencies
	}
	nextCursor := ""
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		nextCursor, err = shared.EncodeCursor(service.CursorSigningKey, shared.CursorPayload{
			Version: 1, Owner: principal.UserID, Domain: "path-notification",
			AfterID: last.ID, AfterCreated: last.CreatedAt, Snapshot: request.Snapshot,
		})
		if err != nil {
			return nil, "", 0, err
		}
	}
	event := shared.NewAuditEvent(
		ctx, service.Clock, principal.UserID, principal.UserID,
		audit.ResourceListed, "notification", "history", audit.Succeeded,
	)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", 0, err
	}
	return page.Items, nextCursor, page.UnreadCount, nil
}

func validNotificationPage(page NotificationPage, limit int, snapshot time.Time) bool {
	if page.UnreadCount < 0 || len(page.Items) > limit || (page.HasMore && len(page.Items) == 0) {
		return false
	}
	var previous InvitationNotificationProjection
	for index, item := range page.Items {
		noInteractionSubject := item.SocialFeedEventID == "" && item.Reaction == "" && item.CommentID == ""
		invitationSubject := ((item.Kind == NotificationPathInvitationReceived && item.Presentation == NotificationActionable) ||
			(item.Kind == NotificationPathInvitationAccepted && item.Presentation == NotificationInformational)) &&
			item.InvitationID != "" && item.OfferedRole.Valid() && item.OwnershipTransferID == "" && noInteractionSubject && item.InteractionDisabled == ""
		transferSubject := ((item.Kind == NotificationPathOwnershipTransferReceived && item.Presentation == NotificationActionable) ||
			(item.Kind == NotificationPathOwnershipTransferAccepted && item.Presentation == NotificationInformational) ||
			(item.Kind == NotificationPathOwnershipTransferDeclined && item.Presentation == NotificationInformational) ||
			(item.Kind == NotificationPathOwnershipTransferCanceled && item.Presentation == NotificationInformational)) &&
			item.InvitationID == "" && item.OfferedRole == "" && item.OwnershipTransferID != "" && noInteractionSubject && item.InteractionDisabled == ""
		memberAccessSubject := item.Presentation == NotificationInformational &&
			((item.Kind == NotificationPathMemberRemoved && item.OfferedRole.Valid()) ||
				(item.Kind == NotificationPathMemberRoleChanged && item.OfferedRole.ValidPathMemberRole())) &&
			item.InvitationID == "" && item.OwnershipTransferID == "" && item.FollowRequestID == "" && noInteractionSubject && item.InteractionDisabled == ""
		memberLeftSubject := item.Kind == NotificationPathMemberLeft && item.Presentation == NotificationInformational &&
			item.OfferedRole == "" && item.InvitationID == "" && item.OwnershipTransferID == "" && item.FollowRequestID == "" &&
			noInteractionSubject && item.InteractionDisabled == ""
		visibilitySubject := item.Kind == NotificationPathVisibilityChanged && item.Presentation == NotificationInformational &&
			validVisibility(item.PathVisibility) && item.OfferedRole == "" && item.InvitationID == "" && item.OwnershipTransferID == "" &&
			item.FollowRequestID == "" && noInteractionSubject && item.InteractionDisabled == ""
		visibilityPayloadValid := visibilitySubject || item.PathVisibility == ""
		socialSubject := ((item.Kind == NotificationNewFollower && item.Presentation == NotificationInformational && item.FollowRequestID == "") ||
			(item.Kind == NotificationFollowRequestReceived && item.Presentation == NotificationActionable && item.FollowRequestID != "") ||
			(item.Kind == NotificationFollowRequestAccepted && item.Presentation == NotificationInformational && item.FollowRequestID != "")) &&
			item.PathID == "" && item.PathName == "" && item.InvitationID == "" && item.OfferedRole == "" && item.OwnershipTransferID == "" && noInteractionSubject && item.InteractionDisabled == ""
		reactionSubject := item.Kind == NotificationPracticeReaction && item.Presentation == NotificationInformational &&
			item.PathID != "" && strings.TrimSpace(item.PathName) != "" && item.PathName == strings.TrimSpace(item.PathName) &&
			item.SocialFeedEventID != "" && strings.TrimSpace(item.SocialFeedEventID) == item.SocialFeedEventID && item.Reaction.Valid() &&
			item.CommentID == "" && item.InvitationID == "" && item.OfferedRole == "" && item.OwnershipTransferID == "" && item.FollowRequestID == "" && item.InteractionDisabled == ""
		commentSubject := item.Kind == NotificationPracticeComment && item.Presentation == NotificationInformational &&
			item.PathID != "" && strings.TrimSpace(item.PathName) != "" && item.PathName == strings.TrimSpace(item.PathName) &&
			item.SocialFeedEventID != "" && strings.TrimSpace(item.SocialFeedEventID) == item.SocialFeedEventID &&
			item.CommentID != "" && strings.TrimSpace(item.CommentID) == item.CommentID && item.Reaction == "" &&
			item.InvitationID == "" && item.OfferedRole == "" && item.OwnershipTransferID == "" && item.FollowRequestID == "" && item.InteractionDisabled == ""
		heartSubject := item.Kind == NotificationCommentHeart && item.Presentation == NotificationInformational &&
			item.PathID != "" && strings.TrimSpace(item.PathName) != "" && item.PathName == strings.TrimSpace(item.PathName) &&
			item.SocialFeedEventID != "" && strings.TrimSpace(item.SocialFeedEventID) == item.SocialFeedEventID &&
			item.CommentID != "" && strings.TrimSpace(item.CommentID) == item.CommentID && item.Reaction == "" &&
			item.InvitationID == "" && item.OfferedRole == "" && item.OwnershipTransferID == "" && item.FollowRequestID == "" && item.InteractionDisabled == ""
		nudgeSubject := item.Kind == NotificationNudgeReceived && item.Presentation == NotificationInformational &&
			item.PathID != "" && strings.TrimSpace(item.PathName) != "" && item.PathName == strings.TrimSpace(item.PathName) &&
			item.NudgeContent.Valid() && item.InvitationID == "" && item.OfferedRole == "" && item.OwnershipTransferID == "" &&
			item.FollowRequestID == "" && noInteractionSubject && item.InteractionDisabled == ""
		noNudgeSubject := item.NudgeContent == (socialdomain.NudgeContent{})
		disabledInteractionSubject := item.Presentation == NotificationInformational &&
			((item.InteractionDisabled == InteractionDisabledComments &&
				(item.Kind == NotificationPracticeComment || item.Kind == NotificationCommentHeart)) ||
				(item.InteractionDisabled == InteractionDisabledReactions && item.Kind == NotificationPracticeReaction)) &&
			item.PathID != "" && strings.TrimSpace(item.PathName) != "" && item.PathName == strings.TrimSpace(item.PathName) &&
			item.SocialFeedEventID != "" && strings.TrimSpace(item.SocialFeedEventID) == item.SocialFeedEventID &&
			item.Reaction == "" && item.CommentID == "" && item.InvitationID == "" && item.OfferedRole == "" &&
			item.OwnershipTransferID == "" && item.FollowRequestID == ""
		pathSubject := (invitationSubject || transferSubject || memberAccessSubject || memberLeftSubject || visibilitySubject) && item.PathID != "" && strings.TrimSpace(item.PathName) != "" &&
			item.PathName == strings.TrimSpace(item.PathName) && item.FollowRequestID == ""
		deletionSubject := item.Kind == NotificationPathDeleted && item.Presentation == NotificationInformational &&
			item.PathID == "" && strings.TrimSpace(item.PathName) != "" && item.PathName == strings.TrimSpace(item.PathName) &&
			item.InvitationID == "" && item.OfferedRole == "" && item.OwnershipTransferID == "" && item.FollowRequestID == "" && noInteractionSubject && item.InteractionDisabled == ""
		if strings.TrimSpace(item.ID) == "" ||
			item.CreatedAt.IsZero() || item.CreatedAt.After(snapshot) ||
			strings.TrimSpace(item.Actor.UserID) == "" ||
			strings.TrimSpace(item.Actor.Username) == "" ||
			item.Actor.Username != strings.TrimSpace(item.Actor.Username) ||
			strings.TrimSpace(item.Actor.DisplayName) == "" ||
			item.Actor.DisplayName != strings.TrimSpace(item.Actor.DisplayName) ||
			!visibilityPayloadValid ||
			(!nudgeSubject && (!noNudgeSubject || (!pathSubject && !socialSubject && !deletionSubject && !reactionSubject && !commentSubject && !heartSubject && !disabledInteractionSubject))) {
			return false
		}
		if index > 0 && (item.CreatedAt.After(previous.CreatedAt) ||
			(item.CreatedAt.Equal(previous.CreatedAt) && item.ID >= previous.ID)) {
			return false
		}
		previous = item
	}
	return true
}

func (service *InvitationService) Accept(
	ctx context.Context,
	authorization, idempotencyKey string,
	invitationID domain.InvitationID,
) (AcceptInvitationResult, error) {
	return service.AcceptConfirmed(
		ctx, authorization, idempotencyKey, invitationID, InvitationWarningAcknowledgement{},
	)
}

func (service *InvitationService) AcceptConfirmed(
	ctx context.Context,
	authorization, idempotencyKey string,
	invitationID domain.InvitationID,
	acknowledgement InvitationWarningAcknowledgement,
) (AcceptInvitationResult, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return AcceptInvitationResult{}, err
	}
	if invitationID == "" || !validIdempotencyKey(idempotencyKey) {
		return AcceptInvitationResult{}, ports.ErrInvalidArgument
	}
	if acknowledgement.PathVisibility != "" &&
		acknowledgement.PathVisibility != "private" &&
		acknowledgement.PathVisibility != "followers" &&
		acknowledgement.PathVisibility != "public" {
		return AcceptInvitationResult{}, ports.ErrInvalidArgument
	}
	if !service.validAcceptDependencies() {
		return AcceptInvitationResult{}, errInvalidInvitationDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return AcceptInvitationResult{}, errInvalidInvitationDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return AcceptInvitationResult{}, platformapp.ErrRateLimited
	}
	digest := canonicalAcceptInvitationRequestHash(invitationID, acknowledgement)
	idempotency := ports.Idempotency{
		PrincipalID: principal.UserID, Operation: AcceptInvitationOperation,
		Key: idempotencyKey, RequestHash: digest[:],
	}
	decision, err := service.Invitations.AcceptanceDecision(ctx, principal.UserID, invitationID, idempotency)
	if err != nil {
		if errors.Is(err, domain.ErrInvitationUnavailable) {
			return AcceptInvitationResult{}, service.denyUnavailableInvitation(ctx, principal.UserID, invitationID, now)
		}
		return AcceptInvitationResult{}, err
	}
	if decision.Replay != nil {
		result := *decision.Replay
		if !result.Replayed || !validAcceptedInvitationReplay(result, principal.UserID, invitationID) {
			return AcceptInvitationResult{}, errInvalidInvitationDependencies
		}
		if err := service.reconcileReplayedInvitation(ctx, result); err != nil {
			return AcceptInvitationResult{}, err
		}
		return result, nil
	}
	if !validInvitationDecision(decision, principal.UserID, invitationID) || decision.Path.Archived() {
		return AcceptInvitationResult{}, service.denyUnavailableInvitation(ctx, principal.UserID, invitationID, now)
	}
	warning, err := service.WarningPolicy.Evaluate(ctx, InvitationWarningInput{
		RecipientVisibility: decision.RecipientVisibility,
		PathVisibility:      decision.Path.Visibility,
		OfferedRole:         decision.Invitation.OfferedRole,
		HasRetainedActivity: decision.HasRetainedActivity,
	})
	if err != nil {
		return AcceptInvitationResult{}, err
	}
	if warning.Required && acknowledgement.PathVisibility != warning.Context.PathVisibility {
		return AcceptInvitationResult{}, &InvitationWarningRequiredError{Context: warning.Context}
	}
	accepted, err := decision.Invitation.Accept(principal.UserID, now)
	if err != nil {
		return AcceptInvitationResult{}, service.denyUnavailableInvitation(ctx, principal.UserID, invitationID, now)
	}
	change := ports.AuthorizationChange{
		ID: service.NewID(), ResourceType: "path", ResourceID: string(accepted.PathID),
		Relation: string(accepted.OfferedRole), SubjectType: "user", SubjectID: principal.UserID,
		OwnerUserID: decision.Path.OwnerUserID, ActorUserID: principal.UserID,
		Operation: ports.AuthorizationTouch, LockedBy: service.AuthorizationWorker, Lease: service.AuthorizationLease,
	}
	notification := InvitationNotification{
		ID: service.NewID(), RecipientUserID: accepted.InviterUserID, ActorUserID: principal.UserID,
		PathID: accepted.PathID, InvitationID: accepted.ID, OfferedRole: accepted.OfferedRole,
		Actionable: false, CreatedAt: now,
	}
	if change.ID == "" || notification.ID == "" {
		return AcceptInvitationResult{}, errInvalidInvitationDependencies
	}
	event := shared.NewAuditEvent(ctx, service.Clock, decision.Path.OwnerUserID, principal.UserID, audit.PathInvitationAccepted, "path_invitation", string(invitationID), audit.Succeeded)
	event.OccurredAt = now
	result, err := service.Invitations.Accept(ctx, AcceptInvitationCommand{
		Invitation: accepted, Acknowledgement: acknowledgement,
		AuthorizationChange: change, Notification: notification,
		Idempotency: idempotency,
		Audit:       event,
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvitationUnavailable) {
			return AcceptInvitationResult{}, service.denyUnavailableInvitation(ctx, principal.UserID, invitationID, now)
		}
		return AcceptInvitationResult{}, err
	}
	if !validAcceptedInvitationResult(result, decision, principal.UserID, change) {
		return AcceptInvitationResult{}, errInvalidInvitationDependencies
	}
	reconcileChange := result.AuthorizationChange
	if result.Replayed {
		if err := service.reconcileReplayedInvitation(ctx, result); err != nil {
			return AcceptInvitationResult{}, err
		}
		return result, nil
	}
	if err := service.reconcileInvitationRelationship(ctx, reconcileChange); err != nil {
		return AcceptInvitationResult{}, err
	}
	return result, nil
}

func validAcceptedInvitationReplay(result AcceptInvitationResult, recipient string, invitationID domain.InvitationID) bool {
	invitation := result.Invitation
	return invitation.ID == invitationID && invitation.RecipientUserID == recipient &&
		invitation.OfferedRole.Valid() && !invitation.AcceptedAt.IsZero() &&
		!invitation.AcceptedAt.Before(invitation.CreatedAt) &&
		validInvitationAuthorizationChange(result.AuthorizationChange, result.AuthorizationChange.OwnerUserID, recipient, invitation)
}

func (service *InvitationService) reconcileReplayedInvitation(ctx context.Context, result AcceptInvitationResult) error {
	claimed, err := service.AuthorizationOutbox.ClaimAuthorizationChange(
		ctx, result.AuthorizationChange.ID, service.AuthorizationWorker, service.AuthorizationLease,
	)
	if errors.Is(err, ports.ErrNotFound) {
		state, statusErr := service.AuthorizationStatus.AuthorizationChangeState(ctx, result.AuthorizationChange.ID)
		if statusErr != nil {
			return statusErr
		}
		switch state {
		case AuthorizationChangeCompleted:
			return nil
		case AuthorizationChangePending, AuthorizationChangeLocked:
			return ports.ErrAuthorizationPending
		case AuthorizationChangeDeadLettered:
			return ports.ErrAuthorizationDeadLettered
		default:
			return errInvalidInvitationDependencies
		}
	}
	if err != nil {
		return err
	}
	if !sameInvitationAuthorizationChange(claimed, result.AuthorizationChange) {
		return errInvalidInvitationDependencies
	}
	return service.reconcileInvitationRelationship(ctx, claimed)
}

func (service *InvitationService) validAcceptDependencies() bool {
	return service.Invitations != nil && service.WarningPolicy != nil && service.Audits != nil &&
		service.AuditRateLimiter != nil && service.Clock != nil && service.NewID != nil &&
		service.Authorizer != nil && service.AuthorizationOutbox != nil &&
		service.AuthorizationStatus != nil &&
		service.AuthorizationSerializer != nil && service.AuthorizationWorker != "" &&
		service.AuthorizationLease > 0
}

func (service *InvitationService) denyUnavailableInvitation(
	ctx context.Context,
	actorUserID string,
	invitationID domain.InvitationID,
	occurredAt time.Time,
) error {
	event := shared.NewAuditEvent(
		ctx, service.Clock, actorUserID, actorUserID, audit.ResourceAccessDenied,
		"path_invitation", string(invitationID), audit.Denied,
	)
	event.OccurredAt = occurredAt
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return err
	}
	return domain.ErrInvitationUnavailable
}

func validInvitationDecision(decision InvitationDecision, recipient string, id domain.InvitationID) bool {
	invitation := decision.Invitation
	if invitation.ID != id || invitation.RecipientUserID != recipient || !invitation.Pending() ||
		decision.Path.ID != invitation.PathID || !validCreatedPath(decision.Path, decision.Path.OwnerUserID) ||
		identity.ValidateProfileVisibility(decision.RecipientVisibility) != nil {
		return false
	}
	return validSentInvitation(invitation, invitation.PathID, invitation.InviterUserID, recipient, invitation.OfferedRole)
}

func canonicalAcceptInvitationRequestHash(
	invitationID domain.InvitationID,
	acknowledgement InvitationWarningAcknowledgement,
) [sha256.Size]byte {
	digest := sha256.New()
	writeHashString(digest, string(invitationID))
	writeHashString(digest, acknowledgement.PathVisibility)
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func validAcceptedInvitationResult(result AcceptInvitationResult, decision InvitationDecision, recipient string, requested ports.AuthorizationChange) bool {
	accepted := result.Invitation
	if accepted.ID != decision.Invitation.ID || accepted.PathID != decision.Invitation.PathID ||
		accepted.InviterUserID != decision.Invitation.InviterUserID ||
		accepted.RecipientUserID != recipient || accepted.OfferedRole != decision.Invitation.OfferedRole ||
		accepted.AcceptedAt.IsZero() || accepted.AcceptedAt.Before(accepted.CreatedAt) ||
		!validInvitationAuthorizationChange(result.AuthorizationChange, decision.Path.OwnerUserID, recipient, accepted) {
		return false
	}
	return result.Replayed || sameInvitationAuthorizationChange(result.AuthorizationChange, requested)
}

func validInvitationAuthorizationChange(change ports.AuthorizationChange, owner, recipient string, invitation domain.Invitation) bool {
	return change.ID != "" && change.ResourceType == "path" && change.ResourceID == string(invitation.PathID) &&
		change.Relation == string(invitation.OfferedRole) && change.SubjectType == "user" &&
		change.SubjectID == recipient && change.OwnerUserID == owner && change.ActorUserID == recipient &&
		change.Operation == ports.AuthorizationTouch
}

func sameInvitationAuthorizationChange(left, right ports.AuthorizationChange) bool {
	return left.ID == right.ID && left.ResourceType == right.ResourceType && left.ResourceID == right.ResourceID &&
		left.Relation == right.Relation && left.SubjectType == right.SubjectType &&
		left.SubjectID == right.SubjectID && left.OwnerUserID == right.OwnerUserID &&
		left.ActorUserID == right.ActorUserID && left.Operation == right.Operation
}

func (service *InvitationService) reconcileInvitationRelationship(ctx context.Context, change ports.AuthorizationChange) error {
	var relationshipErr error
	err := service.AuthorizationSerializer.WithinResource(ctx, change.ResourceType, change.ResourceID, func(locked context.Context) error {
		if err := service.AuthorizationOutbox.RenewAuthorizationChange(locked, change.ID, service.AuthorizationWorker, service.AuthorizationLease); err != nil {
			return err
		}
		relationshipErr = service.Authorizer.WriteRelationship(
			locked, change.ResourceType, change.ResourceID, change.Relation, change.SubjectType, change.SubjectID,
		)
		if relationshipErr != nil {
			auditErr := service.Audits.AppendAuditEvent(locked, shared.NewAuditEvent(
				locked, service.Clock, change.OwnerUserID, change.ActorUserID,
				audit.AuthorizationFailed, change.ResourceType, change.ResourceID, audit.Failed,
			))
			return errors.Join(relationshipErr, auditErr)
		}
		event := shared.NewAuditEvent(
			locked, service.Clock, change.OwnerUserID, change.ActorUserID,
			audit.AuthorizationApplied, change.ResourceType, change.ResourceID, audit.Succeeded,
		)
		return service.AuthorizationOutbox.CompleteAuthorizationChangeWithAudit(
			locked, change.ID, service.AuthorizationWorker, event,
		)
	})
	if relationshipErr != nil {
		_, failErr := service.AuthorizationOutbox.FailAuthorizationChange(
			ctx, change.ID, service.AuthorizationWorker, 5, "dependency_failure",
		)
		return errors.Join(err, failErr)
	}
	return err
}
