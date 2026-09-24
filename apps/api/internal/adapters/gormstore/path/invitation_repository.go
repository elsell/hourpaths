package pathstore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errInvalidPersistedInvitation = errors.New("persisted Path invitation is invalid")

var (
	_ application.InvitationDirectory             = (*Repository)(nil)
	_ application.InvitationRepository            = (*Repository)(nil)
	_ application.AuthorizationChangeStatusReader = (*Repository)(nil)
)

func (r *Repository) ActiveByExactUsername(ctx context.Context, exactUsername string) (application.InvitationRecipient, error) {
	if r == nil || r.DB == nil || exactUsername == "" || strings.TrimSpace(exactUsername) != exactUsername {
		return application.InvitationRecipient{}, ports.ErrInvalidArgument
	}
	var row invitationUserModel
	err := r.DB.WithContext(ctx).
		Where("status = ? AND translate(username, ?, ?) = translate(?, ?, ?)",
			identity.StatusActive, asciiUppercase, asciiLowercase, exactUsername, asciiUppercase, asciiLowercase).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return application.InvitationRecipient{}, ports.ErrNotFound
	}
	if err != nil {
		return application.InvitationRecipient{}, err
	}
	if row.Username == nil || strings.TrimSpace(row.DisplayName) == "" || row.ProfileVisibility == nil ||
		!strings.EqualFold(*row.Username, exactUsername) ||
		identity.ValidateProfileVisibility(*row.ProfileVisibility) != nil {
		return application.InvitationRecipient{}, errInvalidPersistedInvitation
	}
	return application.InvitationRecipient{
		UserID: row.ID, Username: *row.Username, DisplayName: row.DisplayName,
		ProfileVisibility: *row.ProfileVisibility,
	}, nil
}

const (
	asciiUppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	asciiLowercase = "abcdefghijklmnopqrstuvwxyz"
)

func (r *Repository) Send(ctx context.Context, command application.SendInvitationCommand) (application.SendInvitationResult, error) {
	if !validSendInvitationCommand(r, command) {
		return application.SendInvitationResult{}, ports.ErrInvalidArgument
	}
	var result application.SendInvitationResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reservation := idempotencyModel{
			PrincipalID: command.Idempotency.PrincipalID,
			Operation:   command.Idempotency.Operation, Key: command.Idempotency.Key,
			RequestHash: append([]byte(nil), command.Idempotency.RequestHash...),
			ResourceID:  string(command.Invitation.ID), CreatedAt: command.Invitation.CreatedAt,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			existing, err := invitationReservation(tx, command.Idempotency)
			if err != nil {
				return err
			}
			if existing.ResourceID == "" || !bytes.Equal(existing.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			var row invitationModel
			if err := tx.Where("id = ? AND inviter_user_id = ?", existing.ResourceID, command.Invitation.InviterUserID).
				First(&row).Error; err != nil {
				return err
			}
			invitation, err := invitationFromModel(row)
			if err != nil {
				return err
			}
			result = application.SendInvitationResult{Invitation: invitation, Replayed: true}
			return nil
		}

		var pathRow model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", command.Invitation.PathID).First(&pathRow).Error; err != nil {
			return err
		}
		if pathRow.OwnerUserID != command.Audit.OwnerUserID || pathRow.ArchivedAt != nil {
			return ports.ErrConflict
		}
		if command.Invitation.InviterUserID == command.Invitation.RecipientUserID {
			return ports.ErrConflict
		}
		var recipient invitationUserModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(
				"id = ? AND status = ? AND translate(username, ?, ?) = translate(?, ?, ?)",
				command.ExpectedRecipientUserID, identity.StatusActive,
				asciiUppercase, asciiLowercase, command.ExpectedRecipientUsername,
				asciiUppercase, asciiLowercase,
			).
			First(&recipient).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrNotFound
			}
			return err
		}
		if recipient.Username == nil ||
			recipient.ID != command.Invitation.RecipientUserID ||
			!strings.EqualFold(*recipient.Username, command.ExpectedRecipientUsername) {
			return ports.ErrNotFound
		}
		var memberships int64
		if err := tx.Model(&membershipModel{}).
			Where("path_id = ? AND user_id = ?", command.Invitation.PathID, command.Invitation.RecipientUserID).
			Count(&memberships).Error; err != nil {
			return err
		}
		if memberships != 0 {
			return ports.ErrConflict
		}
		inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(invitationToModel(command.Invitation))
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected != 1 {
			return ports.ErrConflict
		}
		if err := createInvitationNotification(tx, command.Notification, "path_invitation_received", "actionable"); err != nil {
			return err
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		result = application.SendInvitationResult{Invitation: command.Invitation}
		return nil
	})
	return result, classifyInvitationPersistenceError(err)
}

func (r *Repository) ListPending(ctx context.Context, recipientUserID string, page application.InvitationPageRequest) (application.InvitationPage, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(recipientUserID) == "" ||
		page.Limit < 1 || page.Limit > 100 || page.Snapshot.IsZero() ||
		(page.AfterID == "") != page.AfterCreated.IsZero() ||
		(!page.AfterCreated.IsZero() && page.AfterCreated.After(page.Snapshot)) {
		return application.InvitationPage{}, ports.ErrInvalidArgument
	}
	query := r.DB.WithContext(ctx).
		Table("path_invitation_models").
		Select(`path_invitation_models.*,
			path_models.name AS path_name,
			path_models.visibility AS path_visibility,
			invitation_recipient.profile_visibility AS recipient_visibility,
			EXISTS (
				SELECT 1
				FROM recorded_activity_models AS retained_activity
				WHERE retained_activity.participant_id = path_invitation_models.recipient_user_id
				  AND retained_activity.path_id = path_invitation_models.path_id
			) AS has_retained_activity,
			invitation_inviter.id AS inviter_id,
			invitation_inviter.username AS inviter_username,
			invitation_inviter.display_name AS inviter_name`).
		Joins("JOIN path_models ON path_models.id = path_invitation_models.path_id").
		Joins("JOIN user_models AS invitation_recipient ON invitation_recipient.id = path_invitation_models.recipient_user_id AND invitation_recipient.status = ?", identity.StatusActive).
		Joins("JOIN user_models AS invitation_inviter ON invitation_inviter.id = path_invitation_models.inviter_user_id AND invitation_inviter.status = ?", identity.StatusActive).
		Where(`path_invitation_models.recipient_user_id = ? AND
			path_invitation_models.created_at <= ? AND
			path_invitation_models.accepted_at IS NULL AND
			path_invitation_models.rejected_at IS NULL AND
			path_invitation_models.canceled_at IS NULL`,
			recipientUserID, page.Snapshot)
	if page.AfterID != "" {
		query = query.Where("path_invitation_models.created_at > ? OR (path_invitation_models.created_at = ? AND path_invitation_models.id > ?)",
			page.AfterCreated, page.AfterCreated, page.AfterID)
	}
	var rows []pendingInvitationRow
	if err := query.Order("path_invitation_models.created_at ASC, path_invitation_models.id ASC").Limit(page.Limit + 1).Find(&rows).Error; err != nil {
		return application.InvitationPage{}, err
	}
	hasMore := len(rows) > page.Limit
	if hasMore {
		rows = rows[:page.Limit]
	}
	items := make([]application.PendingInvitationCandidate, len(rows))
	for index := range rows {
		pending, err := pendingInvitationFromRow(rows[index], recipientUserID)
		if err != nil {
			return application.InvitationPage{}, errInvalidPersistedInvitation
		}
		items[index] = pending
	}
	return application.InvitationPage{Items: items, HasMore: hasMore}, nil
}

func (r *Repository) ListNotifications(
	ctx context.Context,
	recipientUserID string,
	page application.NotificationPageRequest,
) (application.NotificationPage, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(recipientUserID) == "" ||
		page.Limit < 1 || page.Limit > 100 || page.Snapshot.IsZero() ||
		(page.AfterID == "") != page.AfterCreated.IsZero() ||
		(!page.AfterCreated.IsZero() && page.AfterCreated.After(page.Snapshot)) {
		return application.NotificationPage{}, ports.ErrInvalidArgument
	}
	var result application.NotificationPage
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		count, err := visibleUnreadNotificationCount(tx, recipientUserID, page.Snapshot)
		if err != nil {
			return err
		}
		query := notificationProjectionQuery(tx).
			Where("notification_models.recipient_user_id = ? AND notification_models.created_at <= ? AND "+visibleNotificationPredicate,
				recipientUserID, page.Snapshot)
		if page.AfterID != "" {
			query = query.Where(
				"notification_models.created_at < ? OR (notification_models.created_at = ? AND notification_models.id < ?)",
				page.AfterCreated, page.AfterCreated, page.AfterID,
			)
		}
		var rows []invitationNotificationRow
		if err := query.
			Order("notification_models.created_at DESC, notification_models.id DESC").
			Limit(page.Limit + 1).
			Find(&rows).Error; err != nil {
			return err
		}
		hasMore := len(rows) > page.Limit
		if hasMore {
			rows = rows[:page.Limit]
		}
		items := make([]application.InvitationNotificationProjection, len(rows))
		for index := range rows {
			item, err := invitationNotificationFromRow(rows[index], recipientUserID)
			if err != nil {
				return errInvalidPersistedInvitation
			}
			items[index] = item
		}
		result = application.NotificationPage{
			Items: items, HasMore: hasMore, UnreadCount: count,
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

func invitationNotificationFromRow(
	row invitationNotificationRow,
	recipientUserID string,
) (application.InvitationNotificationProjection, error) {
	kind := application.InvitationNotificationKind(row.Kind)
	presentation := application.NotificationPresentation(row.PresentationClass)
	role := domain.MembershipRole(row.OfferedRole)
	invitationSemantics := row.InvitationCreatedAt != nil &&
		((kind == application.NotificationPathInvitationReceived &&
			presentation == application.NotificationActionable &&
			row.RecipientUserID == row.InvitationRecipientID &&
			row.ActorUserID == row.InvitationInviterID) ||
			(kind == application.NotificationPathInvitationAccepted &&
				presentation == application.NotificationInformational &&
				row.RecipientUserID == row.InvitationInviterID &&
				row.ActorUserID == row.InvitationRecipientID &&
				row.InvitationAcceptedAt != nil &&
				!row.InvitationAcceptedAt.After(row.CreatedAt)))
	transferSemantics := row.TransferCreatedAt != nil && row.TransferExpiresAt != nil &&
		((kind == application.NotificationPathOwnershipTransferReceived &&
			presentation == application.NotificationActionable &&
			row.RecipientUserID == row.TransferRecipientID && row.ActorUserID == row.TransferInitiatorID) ||
			(kind == application.NotificationPathOwnershipTransferAccepted &&
				presentation == application.NotificationInformational &&
				row.RecipientUserID == row.TransferInitiatorID && row.ActorUserID == row.TransferRecipientID &&
				row.TransferAcceptedAt != nil && !row.TransferAcceptedAt.After(row.CreatedAt)) ||
			(kind == application.NotificationPathOwnershipTransferDeclined &&
				presentation == application.NotificationInformational &&
				row.RecipientUserID == row.TransferInitiatorID && row.ActorUserID == row.TransferRecipientID &&
				row.TransferDeclinedAt != nil && !row.TransferDeclinedAt.After(row.CreatedAt)) ||
			(kind == application.NotificationPathOwnershipTransferCanceled &&
				presentation == application.NotificationInformational &&
				row.RecipientUserID == row.TransferRecipientID && row.ActorUserID == row.TransferInitiatorID &&
				row.TransferCanceledAt != nil && !row.TransferCanceledAt.After(row.CreatedAt)))
	deletionSemantics := kind == application.NotificationPathDeleted && presentation == application.NotificationInformational &&
		row.PathID == "" && row.PathInvitationID == "" && row.PathOwnershipTransferID == "" && row.OfferedRole == "" &&
		row.PathNameSnapshot != nil && row.ActorUsernameSnapshot != nil && row.ActorDisplayNameSnapshot != nil
	leaveSemantics := kind == application.NotificationPathMemberLeft && presentation == application.NotificationInformational &&
		row.PathID != "" && row.RecipientUserID != row.ActorUserID && row.PathInvitationID == "" &&
		row.PathOwnershipTransferID == "" && row.FollowRequestID == "" && row.FollowSubjectUserID == "" &&
		row.SocialFeedEventID == "" && row.ReactionType == "" && row.CommentID == "" && row.OfferedRole == ""
	memberRemovalSemantics := kind == application.NotificationPathMemberRemoved && row.RecipientUserID != row.ActorUserID && role.Valid()
	memberRoleChangeSemantics := kind == application.NotificationPathMemberRoleChanged && role.ValidPathMemberRole() &&
		(row.RecipientUserID != row.ActorUserID || role == domain.RoleParticipant)
	memberAccessSemantics := (memberRemovalSemantics || memberRoleChangeSemantics) && presentation == application.NotificationInformational &&
		row.PathID != "" && row.PathInvitationID == "" &&
		row.PathOwnershipTransferID == "" && row.FollowRequestID == "" && row.FollowSubjectUserID == "" &&
		row.SocialFeedEventID == "" && row.ReactionType == "" && row.CommentID == ""
	visibilitySemantics := validVisibilityNotificationRow(row, kind, presentation)
	visibilityPayloadValid := visibilitySemantics || row.PathVisibility == ""
	newFollowerSemantics := kind == application.NotificationNewFollower && presentation == application.NotificationInformational &&
		row.FollowRequestID == "" && row.FollowSubjectUserID == row.ActorUserID
	followRequestSemantics := row.FollowRequestCreatedAt != nil && row.FollowRequestID != "" && row.FollowSubjectUserID == row.ActorUserID &&
		((kind == application.NotificationFollowRequestReceived && presentation == application.NotificationActionable &&
			row.RecipientUserID == row.FollowTargetID && row.ActorUserID == row.FollowRequesterID) ||
			(kind == application.NotificationFollowRequestAccepted && presentation == application.NotificationInformational &&
				row.RecipientUserID == row.FollowRequesterID && row.ActorUserID == row.FollowTargetID &&
				row.FollowRequestAcceptedAt != nil && !row.FollowRequestAcceptedAt.After(row.CreatedAt))) &&
		!row.CreatedAt.Before(*row.FollowRequestCreatedAt)
	socialSemantics := (newFollowerSemantics || followRequestSemantics) && row.PathID == "" &&
		row.PathInvitationID == "" && row.PathOwnershipTransferID == "" && row.OfferedRole == "" && row.Channel == "following"
	reaction := socialdomain.Reaction(row.ReactionType)
	reactionSemantics := kind == application.NotificationPracticeReaction && presentation == application.NotificationInformational &&
		row.Channel == "reactions" && row.SocialFeedEventID != "" && reaction.Valid() && row.PathID != "" &&
		row.RecipientUserID != row.ActorUserID && row.PathInvitationID == "" && row.PathOwnershipTransferID == "" &&
		row.FollowRequestID == "" && row.FollowSubjectUserID == "" && row.OfferedRole == "" && row.CommentID == ""
	commentSemantics := kind == application.NotificationPracticeComment && presentation == application.NotificationInformational &&
		row.Channel == "comments" && row.SocialFeedEventID != "" && row.CommentID != "" && row.PathID != "" &&
		row.CommentEventID == row.SocialFeedEventID && row.CommentAuthorID == row.ActorUserID &&
		row.RecipientUserID != row.ActorUserID && row.ReactionType == "" && row.PathInvitationID == "" &&
		row.PathOwnershipTransferID == "" && row.FollowRequestID == "" && row.FollowSubjectUserID == "" && row.OfferedRole == ""
	heartSemantics := kind == application.NotificationCommentHeart && presentation == application.NotificationInformational &&
		row.Channel == "comment_hearts" && row.SocialFeedEventID != "" && row.CommentID != "" && row.PathID != "" &&
		row.CommentEventID == row.SocialFeedEventID && row.CommentAuthorID == row.RecipientUserID &&
		row.RecipientUserID != row.ActorUserID && row.ReactionType == "" && row.PathInvitationID == "" &&
		row.PathOwnershipTransferID == "" && row.FollowRequestID == "" && row.FollowSubjectUserID == "" && row.OfferedRole == ""
	nudgeContent := socialdomain.NudgeContent{Kind: socialdomain.NudgeContentKind(row.NudgeContentKind), Preset: socialdomain.NudgePreset(row.NudgePreset)}
	nudgeSemantics := kind == application.NotificationNudgeReceived && presentation == application.NotificationInformational &&
		row.Channel == "nudges" && row.NudgeID != "" && row.NudgeID == row.NudgeRecordID && nudgeContent.Valid() &&
		row.RecipientUserID == row.NudgeRecipientID && row.ActorUserID == row.NudgeSenderID && row.PathID == row.NudgePathID &&
		row.RecipientUserID != row.ActorUserID && row.NudgeSentAt != nil && row.NudgeSentAt.Equal(row.CreatedAt) &&
		row.PathID != "" && row.PathInvitationID == "" && row.PathOwnershipTransferID == "" && row.FollowRequestID == "" &&
		row.FollowSubjectUserID == "" && row.SocialFeedEventID == "" && row.ReactionType == "" && row.CommentID == "" &&
		row.OfferedRole == "" && row.InteractionDisabledReason == nil
	disabled := application.InteractionDisabledReason("")
	if row.InteractionDisabledReason != nil {
		disabled = application.InteractionDisabledReason(*row.InteractionDisabledReason)
	}
	disabledSemantics := disabled == "" || (disabled.Valid() && row.DeletedAt != nil &&
		((disabled == application.InteractionDisabledReactions && reactionSemantics) ||
			(disabled == application.InteractionDisabledComments && (commentSemantics || heartSemantics))) &&
		row.EventOwnerID != "" && row.EventOwnerUsername != nil && strings.TrimSpace(*row.EventOwnerUsername) != "" &&
		strings.TrimSpace(*row.EventOwnerUsername) == *row.EventOwnerUsername && strings.TrimSpace(row.EventOwnerName) != "" && strings.TrimSpace(row.EventOwnerName) == row.EventOwnerName)
	ordinarySubjectValid := (invitationSemantics && row.PathID == row.InvitationPathID && row.PathInvitationID != "" &&
		row.PathOwnershipTransferID == "" && row.OfferedRole == row.InvitationOfferedRole && role.Valid() &&
		!row.CreatedAt.Before(*row.InvitationCreatedAt)) ||
		(transferSemantics && row.PathID == row.TransferPathID && row.PathInvitationID == "" &&
			row.PathOwnershipTransferID != "" && row.OfferedRole == "" && !row.CreatedAt.Before(*row.TransferCreatedAt)) || deletionSemantics || socialSemantics || reactionSemantics || commentSemantics || heartSemantics
	subjectValid := ((ordinarySubjectValid || leaveSemantics || memberAccessSemantics || visibilitySemantics) && row.NudgeID == "") || nudgeSemantics
	pathSemantics := (!socialSemantics && row.Channel == "path_access" || reactionSemantics || commentSemantics || heartSemantics || nudgeSemantics) && strings.TrimSpace(row.PathName) != "" &&
		strings.TrimSpace(row.PathName) == row.PathName
	actorID, actorUsername, actorName := row.ActorID, row.ActorUsername, row.ActorName
	if deletionSemantics {
		actorUsername, actorName = row.ActorUsernameSnapshot, *row.ActorDisplayNameSnapshot
	} else if disabled != "" {
		actorID, actorUsername, actorName = row.EventOwnerID, row.EventOwnerUsername, row.EventOwnerName
	}
	if strings.TrimSpace(row.ID) == "" ||
		row.RecipientUserID != recipientUserID ||
		(row.DeletedAt != nil && disabled == "") ||
		!disabledSemantics ||
		!visibilityPayloadValid ||
		!subjectValid ||
		row.CreatedAt.IsZero() ||
		(row.ReadAt != nil && row.ReadAt.Before(row.CreatedAt)) ||
		(disabled == "" && row.ActorID != row.ActorUserID) ||
		actorUsername == nil ||
		strings.TrimSpace(*actorUsername) == "" ||
		strings.TrimSpace(*actorUsername) != *actorUsername ||
		strings.TrimSpace(actorName) == "" ||
		strings.TrimSpace(actorName) != actorName ||
		(!pathSemantics && !socialSemantics) {
		return application.InvitationNotificationProjection{}, errInvalidPersistedInvitation
	}
	result := application.InvitationNotificationProjection{
		ID:           row.ID,
		Kind:         kind,
		Presentation: presentation,
		Read:         row.ReadAt != nil,
		CreatedAt:    row.CreatedAt,
		Actor: application.InvitationPublicIdentity{
			UserID: actorID, Username: *actorUsername, DisplayName: actorName,
		},
		PathID:              domain.ID(row.PathID),
		PathName:            row.PathName,
		PathVisibility:      row.PathVisibility,
		FollowRequestID:     row.FollowRequestID,
		SocialFeedEventID:   row.SocialFeedEventID,
		Reaction:            reaction,
		CommentID:           row.CommentID,
		NudgeContent:        nudgeContent,
		InteractionDisabled: disabled,
	}
	if disabled != "" {
		result.Reaction = ""
		result.CommentID = ""
	}
	if invitationSemantics {
		result.InvitationID = domain.InvitationID(row.PathInvitationID)
		result.OfferedRole = role
	} else if transferSemantics {
		result.OwnershipTransferID = domain.OwnershipTransferID(row.PathOwnershipTransferID)
	} else if memberAccessSemantics {
		result.OfferedRole = role
	}
	return result, nil
}

func (r *Repository) AcceptanceDecision(
	ctx context.Context,
	recipientUserID string,
	invitationID domain.InvitationID,
	idempotency ports.Idempotency,
) (application.InvitationDecision, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(recipientUserID) == "" || invitationID == "" ||
		!validInvitationIdempotency(idempotency, recipientUserID, application.AcceptInvitationOperation) {
		return application.InvitationDecision{}, ports.ErrInvalidArgument
	}
	tx := r.DB.WithContext(ctx)
	var reservation idempotencyModel
	err := tx.Where("principal_id = ? AND operation = ? AND key = ?",
		idempotency.PrincipalID, idempotency.Operation, idempotency.Key).First(&reservation).Error
	switch {
	case err == nil:
		if reservation.ResourceID != string(invitationID) || !bytes.Equal(reservation.RequestHash, idempotency.RequestHash) {
			return application.InvitationDecision{}, ports.ErrIdempotencyConflict
		}
		replay, err := acceptedInvitationReplay(tx, recipientUserID, invitationID)
		if err != nil {
			return application.InvitationDecision{}, classifyUnavailableInvitation(err)
		}
		return application.InvitationDecision{Replay: &replay}, nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return application.InvitationDecision{}, err
	}

	var invitationRow invitationModel
	if err := tx.Where(
		"id = ? AND recipient_user_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL",
		invitationID, recipientUserID,
	).First(&invitationRow).Error; err != nil {
		return application.InvitationDecision{}, classifyUnavailableInvitation(err)
	}
	invitation, err := invitationFromModel(invitationRow)
	if err != nil {
		return application.InvitationDecision{}, err
	}
	var pathRow model
	if err := tx.Where("id = ?", invitation.PathID).First(&pathRow).Error; err != nil {
		return application.InvitationDecision{}, classifyUnavailableInvitation(err)
	}
	path, err := toEntity(pathRow)
	if err != nil {
		return application.InvitationDecision{}, err
	}
	var recipient invitationUserModel
	if err := tx.Where("id = ? AND status = ?", recipientUserID, identity.StatusActive).First(&recipient).Error; err != nil {
		return application.InvitationDecision{}, classifyUnavailableInvitation(err)
	}
	if recipient.ProfileVisibility == nil ||
		identity.ValidateProfileVisibility(*recipient.ProfileVisibility) != nil {
		return application.InvitationDecision{}, errInvalidPersistedInvitation
	}
	hasRetainedActivity, err := retainedActivityExists(ctx, tx, recipientUserID, invitation.PathID)
	if err != nil {
		return application.InvitationDecision{}, err
	}
	return application.InvitationDecision{
		Invitation: invitation, Path: path, RecipientVisibility: *recipient.ProfileVisibility,
		HasRetainedActivity: hasRetainedActivity,
	}, nil
}

func (r *Repository) Accept(ctx context.Context, command application.AcceptInvitationCommand) (application.AcceptInvitationResult, error) {
	if !validAcceptInvitationCommand(r, command) {
		return application.AcceptInvitationResult{}, ports.ErrInvalidArgument
	}
	var result application.AcceptInvitationResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reservation := idempotencyModel{
			PrincipalID: command.Idempotency.PrincipalID,
			Operation:   command.Idempotency.Operation, Key: command.Idempotency.Key,
			RequestHash: append([]byte(nil), command.Idempotency.RequestHash...),
			ResourceID:  string(command.Invitation.ID), CreatedAt: command.Invitation.AcceptedAt,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			existing, err := invitationReservation(tx, command.Idempotency)
			if err != nil {
				return err
			}
			if existing.ResourceID != string(command.Invitation.ID) ||
				!bytes.Equal(existing.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			replay, err := acceptedInvitationReplay(tx, command.Invitation.RecipientUserID, command.Invitation.ID)
			if err != nil {
				return err
			}
			result = replay
			return nil
		}

		var row invitationModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"id = ? AND recipient_user_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL",
			command.Invitation.ID, command.Invitation.RecipientUserID,
		).First(&row).Error; err != nil {
			return classifyUnavailableInvitation(err)
		}
		pending, err := invitationFromModel(row)
		if err != nil || !samePendingInvitation(pending, command.Invitation) {
			return domain.ErrInvitationUnavailable
		}
		var pathRow model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", command.Invitation.PathID).First(&pathRow).Error; err != nil {
			return classifyUnavailableInvitation(err)
		}
		if pathRow.OwnerUserID != command.Audit.OwnerUserID || pathRow.ArchivedAt != nil {
			return domain.ErrInvitationUnavailable
		}
		var recipient invitationUserModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ?", command.Invitation.RecipientUserID, identity.StatusActive).
			First(&recipient).Error; err != nil {
			return classifyUnavailableInvitation(err)
		}
		if recipient.ProfileVisibility == nil ||
			identity.ValidateProfileVisibility(*recipient.ProfileVisibility) != nil {
			return errInvalidPersistedInvitation
		}
		hasRetainedActivity, err := retainedActivityExists(
			ctx,
			tx,
			command.Invitation.RecipientUserID,
			command.Invitation.PathID,
		)
		if err != nil {
			return err
		}
		warning, err := r.WarningPolicy.Evaluate(ctx, application.InvitationWarningInput{
			RecipientVisibility: *recipient.ProfileVisibility,
			PathVisibility:      pathRow.Visibility,
			OfferedRole:         pending.OfferedRole,
			HasRetainedActivity: hasRetainedActivity,
		})
		if err != nil {
			return err
		}
		if warning.Required &&
			command.Acknowledgement.PathVisibility != warning.Context.PathVisibility {
			return &application.InvitationWarningRequiredError{Context: warning.Context}
		}
		var memberships int64
		if err := tx.Model(&membershipModel{}).Where(
			"path_id = ? AND user_id = ?", command.Invitation.PathID, command.Invitation.RecipientUserID,
		).Count(&memberships).Error; err != nil {
			return err
		}
		if memberships != 0 {
			return domain.ErrInvitationUnavailable
		}
		outbox := authorizationOutboxModel{
			ID:           command.AuthorizationChange.ID,
			ResourceType: command.AuthorizationChange.ResourceType,
			ResourceID:   command.AuthorizationChange.ResourceID,
			Relation:     command.AuthorizationChange.Relation,
			SubjectType:  command.AuthorizationChange.SubjectType,
			SubjectID:    command.AuthorizationChange.SubjectID,
			OwnerUserID:  command.AuthorizationChange.OwnerUserID,
			ActorUserID:  command.AuthorizationChange.ActorUserID,
			Operation:    command.AuthorizationChange.Operation,
			LockedBy:     command.AuthorizationChange.LockedBy,
			CreatedAt:    command.Invitation.AcceptedAt,
		}
		if err := tx.Create(&outbox).Error; err != nil {
			return err
		}
		if err := tx.Model(&outbox).Update(
			"locked_until",
			gorm.Expr("CURRENT_TIMESTAMP + (? * INTERVAL '1 millisecond')", command.AuthorizationChange.Lease.Milliseconds()),
		).Error; err != nil {
			return err
		}
		updated := tx.Model(&invitationModel{}).Where(
			"id = ? AND recipient_user_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL",
			command.Invitation.ID, command.Invitation.RecipientUserID,
		).Updates(map[string]any{
			"accepted_at":             command.Invitation.AcceptedAt,
			"authorization_change_id": command.AuthorizationChange.ID,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return domain.ErrInvitationUnavailable
		}
		if err := tx.Create(&membershipModel{
			PathID: string(command.Invitation.PathID), UserID: command.Invitation.RecipientUserID,
			Role: string(command.Invitation.OfferedRole),
		}).Error; err != nil {
			return err
		}
		if err := createInvitationNotification(tx, command.Notification, "path_invitation_accepted", "informational"); err != nil {
			return err
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		result = application.AcceptInvitationResult{
			Invitation: command.Invitation, AuthorizationChange: command.AuthorizationChange,
		}
		return nil
	})
	return result, classifyInvitationPersistenceError(err)
}

func (r *Repository) AuthorizationChangeState(ctx context.Context, id string) (application.AuthorizationChangeState, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(id) == "" {
		return "", ports.ErrInvalidArgument
	}
	var row authorizationChangeStateModel
	if err := r.DB.WithContext(ctx).Table("authorization_outbox_models").
		Select("id, locked_by, locked_until, completed_at, dead_lettered_at, (locked_until > CURRENT_TIMESTAMP) AS lock_active").
		Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ports.ErrNotFound
		}
		return "", err
	}
	switch {
	case row.DeadLetteredAt != nil:
		return application.AuthorizationChangeDeadLettered, nil
	case row.CompletedAt != nil:
		return application.AuthorizationChangeCompleted, nil
	case row.LockedBy != "" && row.LockedUntil != nil && row.LockActive:
		return application.AuthorizationChangeLocked, nil
	default:
		return application.AuthorizationChangePending, nil
	}
}

func validSendInvitationCommand(r *Repository, command application.SendInvitationCommand) bool {
	invitation := command.Invitation
	validated, err := domain.NewInvitation(
		invitation.ID, invitation.PathID, invitation.InviterUserID,
		invitation.RecipientUserID, invitation.OfferedRole, invitation.CreatedAt,
	)
	return r != nil && r.DB != nil && err == nil && validated == invitation &&
		command.ExpectedRecipientUserID == invitation.RecipientUserID &&
		strings.TrimSpace(command.ExpectedRecipientUsername) == command.ExpectedRecipientUsername &&
		command.ExpectedRecipientUsername != "" &&
		validInvitationIdempotency(command.Idempotency, invitation.InviterUserID, application.SendInvitationOperation) &&
		validInvitationNotification(command.Notification, invitation, invitation.RecipientUserID, invitation.InviterUserID, true, invitation.CreatedAt) &&
		validInvitationAudit(command.Audit, audit.PathInvitationCreated, invitation, invitation.InviterUserID, invitation.CreatedAt)
}

func validAcceptInvitationCommand(r *Repository, command application.AcceptInvitationCommand) bool {
	invitation := command.Invitation
	change := command.AuthorizationChange
	validAcknowledgement := command.Acknowledgement.PathVisibility == "" ||
		command.Acknowledgement.PathVisibility == "followers" ||
		command.Acknowledgement.PathVisibility == "public"
	return r != nil && r.DB != nil && r.WarningPolicy != nil && validAcknowledgement &&
		!invitation.AcceptedAt.IsZero() &&
		!invitation.AcceptedAt.Before(invitation.CreatedAt) &&
		validInvitationIdempotency(command.Idempotency, invitation.RecipientUserID, application.AcceptInvitationOperation) &&
		change.ID != "" && change.ResourceType == "path" && change.ResourceID == string(invitation.PathID) &&
		change.Relation == string(invitation.OfferedRole) && change.SubjectType == "user" &&
		change.SubjectID == invitation.RecipientUserID && change.OwnerUserID == command.Audit.OwnerUserID &&
		change.ActorUserID == invitation.RecipientUserID && change.Operation == ports.AuthorizationTouch &&
		change.LockedBy != "" && change.Lease > 0 &&
		validInvitationNotification(command.Notification, invitation, invitation.InviterUserID, invitation.RecipientUserID, false, invitation.AcceptedAt) &&
		validInvitationAudit(command.Audit, audit.PathInvitationAccepted, invitation, invitation.RecipientUserID, invitation.AcceptedAt)
}

func validInvitationIdempotency(idempotency ports.Idempotency, principal, operation string) bool {
	return idempotency.PrincipalID == principal && idempotency.Operation == operation &&
		strings.TrimSpace(idempotency.Key) != "" && len(idempotency.RequestHash) == 32
}

func validInvitationNotification(
	notification application.InvitationNotification,
	invitation domain.Invitation,
	recipient, actor string,
	actionable bool,
	createdAt time.Time,
) bool {
	return notification.ID != "" && notification.RecipientUserID == recipient &&
		notification.ActorUserID == actor && notification.PathID == invitation.PathID &&
		notification.InvitationID == invitation.ID && notification.OfferedRole == invitation.OfferedRole &&
		notification.Actionable == actionable && notification.CreatedAt.Equal(createdAt)
}

func validInvitationAudit(event audit.Event, action audit.Action, invitation domain.Invitation, actor string, occurredAt time.Time) bool {
	return event.Valid() && event.Action == action && event.Outcome == audit.Succeeded &&
		event.TargetType == "path_invitation" && event.TargetID == string(invitation.ID) &&
		event.ActorUserID == actor && event.OccurredAt.Equal(occurredAt)
}

func invitationReservation(tx *gorm.DB, idempotency ports.Idempotency) (idempotencyModel, error) {
	var existing idempotencyModel
	err := tx.Where("principal_id = ? AND operation = ? AND key = ?",
		idempotency.PrincipalID, idempotency.Operation, idempotency.Key).First(&existing).Error
	return existing, err
}

func invitationFromModel(row invitationModel) (domain.Invitation, error) {
	terminalCount := 0
	for _, terminal := range []*time.Time{row.AcceptedAt, row.RejectedAt, row.CanceledAt} {
		if terminal != nil {
			terminalCount++
		}
	}
	if terminalCount > 1 ||
		((row.RejectedAt == nil) != (row.RejectionUnreadCount == nil)) ||
		(row.RejectionUnreadCount != nil && *row.RejectionUnreadCount < 0) {
		return domain.Invitation{}, errInvalidPersistedInvitation
	}
	invitation, err := domain.NewInvitation(
		domain.InvitationID(row.ID), domain.ID(row.PathID), row.InviterUserID,
		row.RecipientUserID, domain.MembershipRole(row.OfferedRole), row.CreatedAt.UTC(),
	)
	if err != nil {
		return domain.Invitation{}, errInvalidPersistedInvitation
	}
	if row.AcceptedAt != nil {
		invitation, err = invitation.Accept(row.RecipientUserID, row.AcceptedAt.UTC())
		if err != nil {
			return domain.Invitation{}, errInvalidPersistedInvitation
		}
	}
	if row.RejectedAt != nil {
		invitation, err = invitation.Reject(row.RecipientUserID, row.RejectedAt.UTC())
		if err != nil {
			return domain.Invitation{}, errInvalidPersistedInvitation
		}
	}
	if row.CanceledAt != nil {
		invitation, err = invitation.Cancel(row.CanceledAt.UTC())
		if err != nil {
			return domain.Invitation{}, errInvalidPersistedInvitation
		}
	}
	return invitation, nil
}

func samePendingInvitation(pending, accepted domain.Invitation) bool {
	return pending.ID == accepted.ID && pending.PathID == accepted.PathID &&
		pending.InviterUserID == accepted.InviterUserID &&
		pending.RecipientUserID == accepted.RecipientUserID &&
		pending.OfferedRole == accepted.OfferedRole &&
		pending.CreatedAt.Equal(accepted.CreatedAt) && !accepted.AcceptedAt.IsZero()
}

func acceptedInvitationReplay(tx *gorm.DB, recipientUserID string, invitationID domain.InvitationID) (application.AcceptInvitationResult, error) {
	var row invitationModel
	if err := tx.Where(
		"id = ? AND recipient_user_id = ? AND accepted_at IS NOT NULL AND authorization_change_id IS NOT NULL",
		invitationID, recipientUserID,
	).First(&row).Error; err != nil {
		return application.AcceptInvitationResult{}, err
	}
	invitation, err := invitationFromModel(row)
	if err != nil || row.AuthorizationChangeID == nil {
		return application.AcceptInvitationResult{}, errInvalidPersistedInvitation
	}
	var outbox authorizationOutboxModel
	if err := tx.Where("id = ?", *row.AuthorizationChangeID).First(&outbox).Error; err != nil {
		return application.AcceptInvitationResult{}, err
	}
	return application.AcceptInvitationResult{
		Invitation: invitation,
		AuthorizationChange: ports.AuthorizationChange{
			ID: outbox.ID, ResourceType: outbox.ResourceType, ResourceID: outbox.ResourceID,
			Relation: outbox.Relation, SubjectType: outbox.SubjectType, SubjectID: outbox.SubjectID,
			OwnerUserID: outbox.OwnerUserID, ActorUserID: outbox.ActorUserID,
			Operation: outbox.Operation, LockedBy: outbox.LockedBy,
		},
		Replayed: true,
	}, nil
}
