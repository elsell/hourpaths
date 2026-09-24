package pushapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type PushNotificationResolver interface {
	GetNotification(context.Context, string, string) (pathapp.InvitationNotificationProjection, error)
}

type PushDeliveryWorker struct {
	Repository     ports.PushRepository
	Provider       ports.PushProvider
	Notifications  PushNotificationResolver
	Clock          ports.Clock
	WorkerID       string
	Lease          time.Duration
	ReceiptDelay   time.Duration
	BaseRetryDelay time.Duration
	MaxRetryDelay  time.Duration
	MaxAttempts    int
	BatchSize      int
}

var errPushDeliveryProcessing = errors.New("push delivery processing failed")

func (w PushDeliveryWorker) RunOnce(ctx context.Context) (int, error) {
	if !w.valid() {
		return 0, ports.ErrInvalidArgument
	}
	deliveries, err := w.Repository.ClaimPushDeliveries(ctx, w.WorkerID, w.Lease, w.BatchSize)
	if err != nil {
		return 0, errPushDeliveryProcessing
	}
	for index, delivery := range deliveries {
		if err := w.process(ctx, delivery); err != nil {
			return index, errPushDeliveryProcessing
		}
	}
	return len(deliveries), nil
}

func (w PushDeliveryWorker) valid() bool {
	return w.Repository != nil && w.Provider != nil && w.Notifications != nil && w.Clock != nil &&
		strings.TrimSpace(w.WorkerID) != "" &&
		w.Lease > 0 && w.ReceiptDelay > 0 && w.BaseRetryDelay > 0 &&
		w.MaxRetryDelay >= w.BaseRetryDelay && w.MaxAttempts > 0 &&
		w.BatchSize > 0 && w.BatchSize <= 100
}

func (w PushDeliveryWorker) process(ctx context.Context, delivery ports.PushDelivery) error {
	if delivery.NotificationID == "" || delivery.InstallationID == "" ||
		delivery.RecipientUserID == "" || delivery.Token == "" || delivery.Attempts < 1 {
		return ports.ErrInvalidArgument
	}
	eligible, err := w.Repository.PushDeliveryEligible(ctx, w.WorkerID, delivery.NotificationID, delivery.InstallationID)
	if err != nil {
		return err
	}
	if !eligible {
		return nil
	}
	if delivery.ProviderTicket != "" {
		return w.pollReceipt(ctx, delivery)
	}
	projection, err := w.Notifications.GetNotification(ctx, delivery.RecipientUserID, delivery.NotificationID)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return w.terminal(ctx, delivery, ports.PushDeliverySuppressed, "notification_ineligible")
		}
		return err
	}
	message, ok := localizedPushMessage(delivery, projection)
	if !ok {
		return w.terminal(ctx, delivery, ports.PushDeliverySuppressed, "notification_ineligible")
	}
	// The repository holds the same pair lock as blocking across the final
	// eligibility decision and provider handoff. Therefore either the block
	// commits first and send is never called, or handoff completes before the
	// block may commit.
	var providerErr error
	ticket, handedOff, err := w.Repository.HandoffPushDelivery(ctx, w.WorkerID, delivery.NotificationID, delivery.InstallationID, func() ports.PushTicket {
		var ticket ports.PushTicket
		ticket, providerErr = w.Provider.Send(ctx, message)
		return ticket
	})
	if err != nil {
		return err
	}
	if !handedOff {
		return nil
	}
	if providerErr != nil {
		return w.handleProviderError(ctx, delivery, providerErr)
	}
	return w.handleTicket(ctx, delivery, ticket)
}

func (w PushDeliveryWorker) handleTicket(ctx context.Context, delivery ports.PushDelivery, ticket ports.PushTicket) error {
	switch ticket.State {
	case ports.PushDelivered:
		return w.terminal(ctx, delivery, ports.PushDeliveryDelivered, "")
	case ports.PushAccepted, ports.PushPending:
		if ticket.ReceiptID == "" {
			return w.retryOrFail(ctx, delivery, "provider_ticket_missing")
		}
		at := w.now()
		return w.Repository.TransitionPushDelivery(ctx, w.WorkerID, ports.PushDeliveryTransition{
			NotificationID: delivery.NotificationID, InstallationID: delivery.InstallationID,
			Outcome: ports.PushDeliveryAwaitingReceipt, OccurredAt: at,
			AvailableAt: at.Add(w.ReceiptDelay), ProviderTicket: string(ticket.ReceiptID),
		}, audit.Event{})
	case ports.PushFailed:
		return w.handleFailure(ctx, delivery, ticket.Failure)
	default:
		return w.retryOrFail(ctx, delivery, "provider_ticket_invalid")
	}
}

func (w PushDeliveryWorker) pollReceipt(ctx context.Context, delivery ports.PushDelivery) error {
	receiptID := ports.PushReceiptID(delivery.ProviderTicket)
	receipts, err := w.Provider.Receipts(ctx, []ports.PushReceiptID{receiptID})
	if err != nil {
		return w.handleProviderError(ctx, delivery, err)
	}
	receipt, ok := receipts[receiptID]
	if !ok {
		return w.retryOrFail(ctx, delivery, "provider_receipt_missing")
	}
	switch receipt.State {
	case ports.PushDelivered:
		return w.terminal(ctx, delivery, ports.PushDeliveryDelivered, "")
	case ports.PushAccepted, ports.PushPending:
		if delivery.Attempts >= w.MaxAttempts {
			return w.terminal(ctx, delivery, ports.PushDeliveryPermanentlyFailed, "max_attempts_exhausted")
		}
		at := w.now()
		return w.Repository.TransitionPushDelivery(ctx, w.WorkerID, ports.PushDeliveryTransition{
			NotificationID: delivery.NotificationID, InstallationID: delivery.InstallationID,
			Outcome: ports.PushDeliveryAwaitingReceipt, OccurredAt: at,
			AvailableAt: at.Add(w.ReceiptDelay), ProviderTicket: delivery.ProviderTicket,
		}, audit.Event{})
	case ports.PushFailed:
		return w.handleFailure(ctx, delivery, receipt.Failure)
	default:
		return w.retryOrFail(ctx, delivery, "provider_receipt_invalid")
	}
}

func (w PushDeliveryWorker) handleProviderError(ctx context.Context, delivery ports.PushDelivery, err error) error {
	var providerError *ports.PushProviderError
	if errors.As(err, &providerError) {
		return w.handleFailure(ctx, delivery, &providerError.Failure)
	}
	return w.retryOrFail(ctx, delivery, "provider_unavailable")
}

func (w PushDeliveryWorker) handleFailure(ctx context.Context, delivery ports.PushDelivery, failure *ports.PushFailure) error {
	if failure == nil || strings.TrimSpace(failure.Code) == "" {
		return w.retryOrFail(ctx, delivery, "provider_failure_invalid")
	}
	if failure.DisableDevice {
		event := shared.NewAuditEvent(ctx, w.Clock, delivery.RecipientUserID, delivery.RecipientUserID,
			audit.ResourceDeleted, "push_installation", delivery.InstallationID, audit.Succeeded)
		return w.Repository.DisablePushInstallation(ctx, w.WorkerID, delivery.NotificationID,
			delivery.InstallationID, event.OccurredAt, safeFailureCode(failure.Code), event)
	}
	if failure.Disposition == ports.PushPermanent {
		return w.terminal(ctx, delivery, ports.PushDeliveryPermanentlyFailed, safeFailureCode(failure.Code))
	}
	return w.retryOrFail(ctx, delivery, safeFailureCode(failure.Code))
}

func (w PushDeliveryWorker) retryOrFail(ctx context.Context, delivery ports.PushDelivery, code string) error {
	if delivery.Attempts >= w.MaxAttempts {
		return w.terminal(ctx, delivery, ports.PushDeliveryPermanentlyFailed, "max_attempts_exhausted")
	}
	at := w.now()
	return w.Repository.TransitionPushDelivery(ctx, w.WorkerID, ports.PushDeliveryTransition{
		NotificationID: delivery.NotificationID, InstallationID: delivery.InstallationID,
		Outcome: ports.PushDeliveryRetry, OccurredAt: at,
		AvailableAt: at.Add(w.retryDelay(delivery.Attempts)), FailureCode: safeFailureCode(code),
	}, audit.Event{})
}

func (w PushDeliveryWorker) terminal(
	ctx context.Context,
	delivery ports.PushDelivery,
	outcome ports.PushDeliveryOutcome,
	failureCode string,
) error {
	event := shared.NewAuditEvent(ctx, w.Clock, delivery.RecipientUserID, delivery.RecipientUserID,
		audit.ResourceUpdated, "notification_push_delivery",
		delivery.NotificationID+"/"+delivery.InstallationID, audit.Succeeded)
	return w.Repository.TransitionPushDelivery(ctx, w.WorkerID, ports.PushDeliveryTransition{
		NotificationID: delivery.NotificationID, InstallationID: delivery.InstallationID,
		Outcome: outcome, OccurredAt: event.OccurredAt, FailureCode: failureCode,
	}, event)
}

func (w PushDeliveryWorker) retryDelay(attempt int) time.Duration {
	delay := w.BaseRetryDelay
	for count := 1; count < attempt && delay < w.MaxRetryDelay; count++ {
		if delay > w.MaxRetryDelay/2 {
			return w.MaxRetryDelay
		}
		delay *= 2
	}
	if delay > w.MaxRetryDelay {
		return w.MaxRetryDelay
	}
	return delay
}

func (w PushDeliveryWorker) now() time.Time { return w.Clock.Now().UTC() }

func safeFailureCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "provider_failure"
	}
	if len(code) > 64 {
		return "provider_failure"
	}
	for _, character := range code {
		if !(character == '_' || character == '-' ||
			character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9') {
			return "provider_failure"
		}
	}
	return code
}

func localizedPushMessage(
	delivery ports.PushDelivery,
	projection pathapp.InvitationNotificationProjection,
) (ports.PushMessage, bool) {
	if projection.ID != delivery.NotificationID || projection.CreatedAt.IsZero() ||
		strings.TrimSpace(projection.Actor.UserID) == "" ||
		strings.TrimSpace(projection.Actor.Username) == "" ||
		strings.TrimSpace(projection.Actor.DisplayName) == "" ||
		strings.TrimSpace(projection.PathName) == "" ||
		!validPushNotificationSubject(projection) {
		return ports.PushMessage{}, false
	}
	message := ports.PushMessage{
		DeviceToken: delivery.Token, NotificationID: delivery.NotificationID,
	}
	switch {
	case projection.Kind == pathapp.NotificationPathInvitationReceived &&
		projection.Presentation == pathapp.NotificationActionable:
		message.Presentation = ports.PushActionable
		if delivery.Locale == "es" {
			message.Title = "Invitación a un camino"
			message.Body = fmt.Sprintf("%s te invitó a unirte a %s.", projection.Actor.DisplayName, projection.PathName)
		} else if delivery.Locale == "en" {
			message.Title = "Path invitation"
			message.Body = fmt.Sprintf("%s invited you to join %s.", projection.Actor.DisplayName, projection.PathName)
		} else {
			return ports.PushMessage{}, false
		}
	case projection.Kind == pathapp.NotificationPathInvitationAccepted &&
		projection.Presentation == pathapp.NotificationInformational:
		message.Presentation = ports.PushInformational
		if delivery.Locale == "es" {
			message.Title = "Invitación aceptada"
			message.Body = fmt.Sprintf("%s aceptó tu invitación a %s.", projection.Actor.DisplayName, projection.PathName)
		} else if delivery.Locale == "en" {
			message.Title = "Invitation accepted"
			message.Body = fmt.Sprintf("%s accepted your invitation to %s.", projection.Actor.DisplayName, projection.PathName)
		} else {
			return ports.PushMessage{}, false
		}
	case projection.Kind == pathapp.NotificationPracticeReaction &&
		projection.Presentation == pathapp.NotificationInformational && projection.Reaction.Valid():
		message.Presentation = ports.PushInformational
		if delivery.Locale == "es" {
			message.Title = "Nuevo apoyo"
			message.Body = fmt.Sprintf("%s reaccionó a tu actualización en %s.", projection.Actor.DisplayName, projection.PathName)
		} else if delivery.Locale == "en" {
			message.Title = "New encouragement"
			message.Body = fmt.Sprintf("%s reacted to your update on %s.", projection.Actor.DisplayName, projection.PathName)
		} else {
			return ports.PushMessage{}, false
		}
	case projection.Kind == pathapp.NotificationPracticeComment &&
		projection.Presentation == pathapp.NotificationInformational:
		message.Presentation = ports.PushInformational
		if delivery.Locale == "es" {
			message.Title = "Nuevo comentario"
			message.Body = fmt.Sprintf("%s comentó en %s.", projection.Actor.DisplayName, projection.PathName)
		} else if delivery.Locale == "en" {
			message.Title = "New comment"
			message.Body = fmt.Sprintf("%s commented on %s.", projection.Actor.DisplayName, projection.PathName)
		} else {
			return ports.PushMessage{}, false
		}
	case projection.Kind == pathapp.NotificationCommentHeart &&
		projection.Presentation == pathapp.NotificationInformational:
		message.Presentation = ports.PushInformational
		if delivery.Locale == "es" {
			message.Title = "Nuevo corazón"
			message.Body = fmt.Sprintf("%s marcó tu comentario con un corazón en %s.", projection.Actor.DisplayName, projection.PathName)
		} else if delivery.Locale == "en" {
			message.Title = "New comment heart"
			message.Body = fmt.Sprintf("%s hearted your comment on %s.", projection.Actor.DisplayName, projection.PathName)
		} else {
			return ports.PushMessage{}, false
		}
	case projection.Kind == pathapp.NotificationNudgeReceived &&
		projection.Presentation == pathapp.NotificationInformational && projection.NudgeContent.Valid():
		preset, ok := localizedNudgePreset(delivery.Locale, projection.NudgeContent.Preset)
		if !ok {
			return ports.PushMessage{}, false
		}
		message.Presentation = ports.PushInformational
		if delivery.Locale == "es" {
			message.Title = "Nuevo ánimo"
			message.Body = fmt.Sprintf("%s te animó en %s: «%s»", projection.Actor.DisplayName, projection.PathName, preset)
		} else if delivery.Locale == "en" {
			message.Title = "New encouragement"
			message.Body = fmt.Sprintf("%s encouraged you on %s: “%s”", projection.Actor.DisplayName, projection.PathName, preset)
		} else {
			return ports.PushMessage{}, false
		}
	case projection.Kind == pathapp.NotificationPathMemberRemoved &&
		projection.Presentation == pathapp.NotificationInformational:
		message.Presentation = ports.PushInformational
		switch {
		case delivery.Locale == "en" && projection.OfferedRole == pathdomain.RoleParticipant:
			message.Title = "Path access removed"
			message.Body = fmt.Sprintf("%s removed your participant access to %s. Your activity and progress on this Path were removed.", projection.Actor.DisplayName, projection.PathName)
		case delivery.Locale == "en" && projection.OfferedRole == pathdomain.RoleSupporter:
			message.Title = "Path access removed"
			message.Body = fmt.Sprintf("%s removed your supporter access to %s.", projection.Actor.DisplayName, projection.PathName)
		case delivery.Locale == "es" && projection.OfferedRole == pathdomain.RoleParticipant:
			message.Title = "Acceso a la ruta eliminado"
			message.Body = fmt.Sprintf("%s eliminó tu acceso de participante a %s. Tu actividad y progreso en esta ruta se eliminaron.", projection.Actor.DisplayName, projection.PathName)
		case delivery.Locale == "es" && projection.OfferedRole == pathdomain.RoleSupporter:
			message.Title = "Acceso a la ruta eliminado"
			message.Body = fmt.Sprintf("%s eliminó tu acceso como persona de apoyo a %s.", projection.Actor.DisplayName, projection.PathName)
		default:
			return ports.PushMessage{}, false
		}
	case projection.Kind == pathapp.NotificationPathOwnershipTransferReceived &&
		projection.Presentation == pathapp.NotificationActionable:
		message.Presentation = ports.PushActionable
		if delivery.Locale == "es" {
			message.Title = "Solicitud de propiedad"
			message.Body = fmt.Sprintf("%s quiere transferirte %s.", projection.Actor.DisplayName, projection.PathName)
		} else if delivery.Locale == "en" {
			message.Title = "Ownership request"
			message.Body = fmt.Sprintf("%s wants to transfer %s to you.", projection.Actor.DisplayName, projection.PathName)
		} else {
			return ports.PushMessage{}, false
		}
	case projection.Kind == pathapp.NotificationPathOwnershipTransferAccepted &&
		projection.Presentation == pathapp.NotificationInformational:
		message.Presentation = ports.PushInformational
		if delivery.Locale == "es" {
			message.Title = "Propiedad transferida"
			message.Body = fmt.Sprintf("%s ahora es propietario de %s. Ahora eres administrador.", projection.Actor.DisplayName, projection.PathName)
		} else if delivery.Locale == "en" {
			message.Title = "Ownership transferred"
			message.Body = fmt.Sprintf("%s now owns %s. You are now an administrator.", projection.Actor.DisplayName, projection.PathName)
		} else {
			return ports.PushMessage{}, false
		}
	case projection.Kind == pathapp.NotificationPathDeleted &&
		projection.Presentation == pathapp.NotificationInformational:
		message.Presentation = ports.PushInformational
		if delivery.Locale == "es" {
			message.Title = "Camino eliminado"
			message.Body = fmt.Sprintf("%s eliminó %s.", projection.Actor.DisplayName, projection.PathName)
		} else if delivery.Locale == "en" {
			message.Title = "Path deleted"
			message.Body = fmt.Sprintf("%s deleted %s.", projection.Actor.DisplayName, projection.PathName)
		} else {
			return ports.PushMessage{}, false
		}
	case projection.Kind == pathapp.NotificationPathMemberLeft &&
		projection.Presentation == pathapp.NotificationInformational:
		message.Presentation = ports.PushInformational
		if delivery.Locale == "es" {
			message.Title = "Miembro fuera del camino"
			message.Body = fmt.Sprintf("%s abandonó %s.", projection.Actor.DisplayName, projection.PathName)
		} else if delivery.Locale == "en" {
			message.Title = "Path member left"
			message.Body = fmt.Sprintf("%s left %s.", projection.Actor.DisplayName, projection.PathName)
		} else {
			return ports.PushMessage{}, false
		}
	case projection.Kind == pathapp.NotificationPathVisibilityChanged &&
		projection.Presentation == pathapp.NotificationInformational:
		visibility, ok := localizedPathVisibility(delivery.Locale, projection.PathVisibility)
		if !ok {
			return ports.PushMessage{}, false
		}
		message.Presentation = ports.PushInformational
		if delivery.Locale == "es" {
			message.Title = "Visibilidad del camino actualizada"
			message.Body = fmt.Sprintf("%s cambió %s a %s. Tu identidad, progreso y actividad en este camino ahora son visibles para esa audiencia; tu perfil y otros caminos no cambiaron.", projection.Actor.DisplayName, projection.PathName, visibility)
		} else if delivery.Locale == "en" {
			message.Title = "Path visibility changed"
			message.Body = fmt.Sprintf("%s changed %s to %s. Your identity, progress, and activity on this Path are now visible to that audience; your profile and other Paths are unchanged.", projection.Actor.DisplayName, projection.PathName, visibility)
		} else {
			return ports.PushMessage{}, false
		}
	default:
		return ports.PushMessage{}, false
	}
	return message, true
}

func localizedPathVisibility(locale, visibility string) (string, bool) {
	labels := map[string]map[string]string{
		"en": {"private": "Private", "followers": "Followers", "public": "Public"},
		"es": {"private": "Privado", "followers": "Seguidores", "public": "Público"},
	}
	localized, ok := labels[locale][visibility]
	return localized, ok
}

func localizedNudgePreset(locale string, preset socialdomain.NudgePreset) (string, bool) {
	if locale == "es" {
		switch preset {
		case socialdomain.NudgeYouHaveGotThis:
			return "¡Tú puedes!", true
		case socialdomain.NudgeLetsGo:
			return "¡Vamos!", true
		case socialdomain.NudgeLittleProgressCounts:
			return "Un poco de progreso cuenta.", true
		case socialdomain.NudgeKeepItGoing:
			return "¡Sigue así!", true
		case socialdomain.NudgeTimeToWork:
			return "¡Es hora de ponerse manos a la obra!", true
		}
	}
	if locale == "en" {
		switch preset {
		case socialdomain.NudgeYouHaveGotThis:
			return "You’ve got this!", true
		case socialdomain.NudgeLetsGo:
			return "Let’s go!", true
		case socialdomain.NudgeLittleProgressCounts:
			return "A little progress counts.", true
		case socialdomain.NudgeKeepItGoing:
			return "Keep it going!", true
		case socialdomain.NudgeTimeToWork:
			return "Time to put in some work!", true
		}
	}
	return "", false
}

func validPushNotificationSubject(projection pathapp.InvitationNotificationProjection) bool {
	pathIDPresent := strings.TrimSpace(string(projection.PathID)) != ""
	invitation := pathIDPresent && projection.InvitationID != "" && projection.OfferedRole.Valid() && projection.OwnershipTransferID == ""
	transfer := pathIDPresent && projection.InvitationID == "" && projection.OfferedRole == "" && projection.OwnershipTransferID != ""
	deletion := projection.Kind == pathapp.NotificationPathDeleted &&
		projection.Presentation == pathapp.NotificationInformational &&
		!pathIDPresent && projection.InvitationID == "" && projection.OfferedRole == "" && projection.OwnershipTransferID == ""
	leave := projection.Kind == pathapp.NotificationPathMemberLeft && projection.Presentation == pathapp.NotificationInformational &&
		pathIDPresent && projection.InvitationID == "" && projection.OfferedRole == "" && projection.OwnershipTransferID == "" &&
		projection.FollowRequestID == "" && projection.SocialFeedEventID == "" && projection.CommentID == "" && projection.Reaction == ""
	removal := projection.Kind == pathapp.NotificationPathMemberRemoved && projection.Presentation == pathapp.NotificationInformational &&
		pathIDPresent && (projection.OfferedRole == pathdomain.RoleParticipant || projection.OfferedRole == pathdomain.RoleSupporter) &&
		projection.InvitationID == "" && projection.OwnershipTransferID == "" && projection.FollowRequestID == "" &&
		projection.SocialFeedEventID == "" && projection.CommentID == "" && projection.Reaction == "" && projection.InteractionDisabled == ""
	visibility := projection.Kind == pathapp.NotificationPathVisibilityChanged && projection.Presentation == pathapp.NotificationInformational &&
		pathIDPresent && (projection.PathVisibility == "private" || projection.PathVisibility == "followers" || projection.PathVisibility == "public") &&
		projection.InvitationID == "" && projection.OfferedRole == "" && projection.OwnershipTransferID == "" && projection.FollowRequestID == "" &&
		projection.SocialFeedEventID == "" && projection.CommentID == "" && projection.Reaction == "" && projection.InteractionDisabled == ""
	reaction := projection.Kind == pathapp.NotificationPracticeReaction && projection.Presentation == pathapp.NotificationInformational &&
		pathIDPresent && projection.SocialFeedEventID != "" && projection.Reaction.Valid() && projection.InvitationID == "" && projection.OwnershipTransferID == "" && projection.OfferedRole == ""
	comment := projection.Kind == pathapp.NotificationPracticeComment && projection.Presentation == pathapp.NotificationInformational &&
		pathIDPresent && projection.SocialFeedEventID != "" && projection.CommentID != "" && projection.Reaction == "" && projection.InvitationID == "" && projection.OwnershipTransferID == "" && projection.OfferedRole == ""
	heart := projection.Kind == pathapp.NotificationCommentHeart && projection.Presentation == pathapp.NotificationInformational &&
		pathIDPresent && projection.SocialFeedEventID != "" && projection.CommentID != "" && projection.Reaction == "" && projection.InvitationID == "" && projection.OwnershipTransferID == "" && projection.OfferedRole == ""
	nudge := projection.Kind == pathapp.NotificationNudgeReceived && projection.Presentation == pathapp.NotificationInformational &&
		pathIDPresent && projection.NudgeContent.Valid() && projection.InvitationID == "" && projection.OwnershipTransferID == "" && projection.OfferedRole == "" &&
		projection.FollowRequestID == "" && projection.SocialFeedEventID == "" && projection.CommentID == "" && projection.Reaction == "" && projection.InteractionDisabled == ""
	noNudgeContent := projection.NudgeContent == (socialdomain.NudgeContent{})
	return ((invitation || transfer || deletion || leave || removal || visibility || reaction || comment || heart) && noNudgeContent) || nudge
}
