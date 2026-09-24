package pushapp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type pushWorkerClock struct{ now time.Time }

func (clock pushWorkerClock) Now() time.Time { return clock.now }

type pushWorkerRepository struct {
	claims        []ports.PushDelivery
	claimErr      error
	transitions   []ports.PushDeliveryTransition
	audits        []audit.Event
	disabled      []string
	transitionErr error
	eligible      bool
	eligibility   []bool
	handoffs      int
}

func (*pushWorkerRepository) UpsertPushInstallation(context.Context, ports.PushInstallation, audit.Event) error {
	return nil
}
func (*pushWorkerRepository) DeletePushInstallation(context.Context, string, string, time.Time, audit.Event) error {
	return nil
}
func (repository *pushWorkerRepository) ClaimPushDeliveries(context.Context, string, time.Duration, int) ([]ports.PushDelivery, error) {
	return repository.claims, repository.claimErr
}
func (repository *pushWorkerRepository) PushDeliveryEligible(context.Context, string, string, string) (bool, error) {
	if len(repository.eligibility) > 0 {
		value := repository.eligibility[0]
		repository.eligibility = repository.eligibility[1:]
		return value, nil
	}
	return repository.eligible || repository.eligibility == nil, nil
}
func (repository *pushWorkerRepository) HandoffPushDelivery(_ context.Context, _, _, _ string, send func() ports.PushTicket) (ports.PushTicket, bool, error) {
	repository.handoffs++
	eligible := repository.eligible || repository.eligibility == nil
	if len(repository.eligibility) > 0 {
		eligible = repository.eligibility[0]
		repository.eligibility = repository.eligibility[1:]
	}
	if !eligible {
		return ports.PushTicket{}, false, nil
	}
	return send(), true, nil
}
func (repository *pushWorkerRepository) TransitionPushDelivery(_ context.Context, _ string, transition ports.PushDeliveryTransition, event audit.Event) error {
	repository.transitions = append(repository.transitions, transition)
	repository.audits = append(repository.audits, event)
	return repository.transitionErr
}
func (repository *pushWorkerRepository) DisablePushInstallation(
	_ context.Context, _ string, _, installationID string, _ time.Time, _ string, event audit.Event,
) error {
	repository.disabled = append(repository.disabled, installationID)
	repository.audits = append(repository.audits, event)
	return repository.transitionErr
}

type pushWorkerProvider struct {
	ticket       ports.PushTicket
	sendErr      error
	receipts     map[ports.PushReceiptID]ports.PushReceipt
	receiptsErr  error
	messages     []ports.PushMessage
	receiptCalls [][]ports.PushReceiptID
}

func (provider *pushWorkerProvider) Send(_ context.Context, message ports.PushMessage) (ports.PushTicket, error) {
	provider.messages = append(provider.messages, message)
	return provider.ticket, provider.sendErr
}
func (provider *pushWorkerProvider) Receipts(_ context.Context, ids []ports.PushReceiptID) (map[ports.PushReceiptID]ports.PushReceipt, error) {
	provider.receiptCalls = append(provider.receiptCalls, append([]ports.PushReceiptID(nil), ids...))
	return provider.receipts, provider.receiptsErr
}

type pushWorkerNotifications struct {
	item  pathapp.InvitationNotificationProjection
	err   error
	calls int
}

func (notifications *pushWorkerNotifications) GetNotification(
	context.Context, string, string,
) (pathapp.InvitationNotificationProjection, error) {
	notifications.calls++
	return notifications.item, notifications.err
}

func TestPushDeliveryWorkerLocalizesAndAwaitsAcceptedTicket(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repository := &pushWorkerRepository{claims: []ports.PushDelivery{pushWorkerDelivery(1, "")}}
	repository.claims[0].Locale = "es"
	provider := &pushWorkerProvider{ticket: ports.PushTicket{
		ReceiptID: "receipt-1", State: ports.PushAccepted,
	}}
	notifications := &pushWorkerNotifications{item: pushWorkerProjection(
		pathapp.NotificationPathInvitationReceived, pathapp.NotificationActionable,
	)}
	worker := testPushWorker(now, repository, provider, notifications)

	processed, err := worker.RunOnce(context.Background())
	if err != nil || processed != 1 {
		t.Fatalf("RunOnce() = %d, %v", processed, err)
	}
	if len(provider.messages) != 1 ||
		provider.messages[0].Title != "Invitación a un camino" ||
		provider.messages[0].Body != "Alex te invitó a unirte a Lectura." ||
		provider.messages[0].Presentation != ports.PushActionable {
		t.Fatalf("localized message = %+v", provider.messages)
	}
	if len(repository.transitions) != 1 ||
		repository.transitions[0].Outcome != ports.PushDeliveryAwaitingReceipt ||
		repository.transitions[0].ProviderTicket != "receipt-1" ||
		!repository.transitions[0].AvailableAt.Equal(now.Add(30*time.Second)) ||
		repository.audits[0] != (audit.Event{}) {
		t.Fatalf("ticket transition = %+v audit=%+v", repository.transitions, repository.audits)
	}
}

func TestPushDeliveryWorkerRevalidatesLeasedDeliveryImmediatelyBeforeSend(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	repository := &pushWorkerRepository{claims: []ports.PushDelivery{pushWorkerDelivery(1, "")}, eligibility: []bool{true, false}}
	provider := &pushWorkerProvider{ticket: ports.PushTicket{State: ports.PushDelivered}}
	worker := testPushWorker(now, repository, provider, &pushWorkerNotifications{item: pushWorkerProjection(pathapp.NotificationPathInvitationReceived, pathapp.NotificationActionable)})
	processed, err := worker.RunOnce(context.Background())
	if err != nil || processed != 1 || repository.handoffs != 1 || len(provider.messages) != 0 || len(repository.transitions) != 0 {
		t.Fatalf("processed=%d err=%v handoffs=%d messages=%+v transitions=%+v", processed, err, repository.handoffs, provider.messages, repository.transitions)
	}
}

func TestLocalizedPushMessageCoversSupportedLanguagesAndPresentations(t *testing.T) {
	tests := []struct {
		name         string
		locale       string
		kind         pathapp.InvitationNotificationKind
		presentation pathapp.NotificationPresentation
		title, body  string
	}{
		{"actionable English", "en", pathapp.NotificationPathInvitationReceived, pathapp.NotificationActionable,
			"Path invitation", "Alex invited you to join Lectura."},
		{"actionable Spanish", "es", pathapp.NotificationPathInvitationReceived, pathapp.NotificationActionable,
			"Invitación a un camino", "Alex te invitó a unirte a Lectura."},
		{"informational English", "en", pathapp.NotificationPathInvitationAccepted, pathapp.NotificationInformational,
			"Invitation accepted", "Alex accepted your invitation to Lectura."},
		{"informational Spanish", "es", pathapp.NotificationPathInvitationAccepted, pathapp.NotificationInformational,
			"Invitación aceptada", "Alex aceptó tu invitación a Lectura."},
		{"ownership request English", "en", pathapp.NotificationPathOwnershipTransferReceived, pathapp.NotificationActionable,
			"Ownership request", "Alex wants to transfer Lectura to you."},
		{"ownership request Spanish", "es", pathapp.NotificationPathOwnershipTransferReceived, pathapp.NotificationActionable,
			"Solicitud de propiedad", "Alex quiere transferirte Lectura."},
		{"ownership accepted English", "en", pathapp.NotificationPathOwnershipTransferAccepted, pathapp.NotificationInformational,
			"Ownership transferred", "Alex now owns Lectura. You are now an administrator."},
		{"ownership accepted Spanish", "es", pathapp.NotificationPathOwnershipTransferAccepted, pathapp.NotificationInformational,
			"Propiedad transferida", "Alex ahora es propietario de Lectura. Ahora eres administrador."},
		{"path deleted English", "en", pathapp.NotificationPathDeleted, pathapp.NotificationInformational,
			"Path deleted", "Alex deleted Lectura."},
		{"path deleted Spanish", "es", pathapp.NotificationPathDeleted, pathapp.NotificationInformational,
			"Camino eliminado", "Alex eliminó Lectura."},
		{"path member left English", "en", pathapp.NotificationPathMemberLeft, pathapp.NotificationInformational,
			"Path member left", "Alex left Lectura."},
		{"path member left Spanish", "es", pathapp.NotificationPathMemberLeft, pathapp.NotificationInformational,
			"Miembro fuera del camino", "Alex abandonó Lectura."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			delivery := pushWorkerDelivery(1, "")
			delivery.Locale = test.locale
			message, ok := localizedPushMessage(delivery, pushWorkerProjection(test.kind, test.presentation))
			if !ok || message.Title != test.title || message.Body != test.body {
				t.Fatalf("localized message = %+v, ok=%v", message, ok)
			}
		})
	}
}

func TestPathDeletionPushIsQuietAndHasNoDeletedPathTarget(t *testing.T) {
	projection := pushWorkerProjection(pathapp.NotificationPathDeleted, pathapp.NotificationInformational)
	projection.PathID = ""
	projection.InvitationID = ""
	projection.OfferedRole = ""
	message, ok := localizedPushMessage(pushWorkerDelivery(1, ""), projection)
	if !ok || message.Presentation != ports.PushInformational ||
		message.Title != "Path deleted" || message.Body != "Alex deleted Lectura." {
		t.Fatalf("deletion push = %+v, ok=%v", message, ok)
	}
	if strings.Contains(message.Body, "path-1") || strings.Contains(message.Body, "notification-1") {
		t.Fatalf("deletion push leaked internal target: %+v", message)
	}
}

func TestPathVisibilityPushUsesInformationalLocalizedPathOnlyCopy(t *testing.T) {
	projection := pushWorkerProjection(pathapp.NotificationPathVisibilityChanged, pathapp.NotificationInformational)
	projection.InvitationID, projection.OfferedRole = "", ""
	projection.PathVisibility = "followers"
	for _, test := range []struct{ locale, title, body string }{
		{"en", "Path visibility changed", "Alex changed Lectura to Followers. Your identity, progress, and activity on this Path are now visible to that audience; your profile and other Paths are unchanged."},
		{"es", "Visibilidad del camino actualizada", "Alex cambió Lectura a Seguidores. Tu identidad, progreso y actividad en este camino ahora son visibles para esa audiencia; tu perfil y otros caminos no cambiaron."},
	} {
		delivery := pushWorkerDelivery(1, "")
		delivery.Locale = test.locale
		message, ok := localizedPushMessage(delivery, projection)
		if !ok || message.Presentation != ports.PushInformational || message.Title != test.title || message.Body != test.body {
			t.Fatalf("locale=%s message=%+v ok=%v", test.locale, message, ok)
		}
	}
	projection.PathVisibility = "unsupported"
	if message, ok := localizedPushMessage(pushWorkerDelivery(1, ""), projection); ok || message != (ports.PushMessage{}) {
		t.Fatalf("invalid visibility produced push: %+v", message)
	}
}

func TestPracticeReactionPushUsesInformationalLocalizedCopy(t *testing.T) {
	projection := pushWorkerProjection(pathapp.NotificationPracticeReaction, pathapp.NotificationInformational)
	projection.InvitationID, projection.OfferedRole = "", ""
	projection.SocialFeedEventID, projection.Reaction = "practice:activity", socialdomain.ReactionFire
	for _, test := range []struct{ locale, title, body string }{
		{"en", "New encouragement", "Alex reacted to your update on Lectura."},
		{"es", "Nuevo apoyo", "Alex reaccionó a tu actualización en Lectura."},
	} {
		delivery := pushWorkerDelivery(1, "")
		delivery.Locale = test.locale
		message, ok := localizedPushMessage(delivery, projection)
		if !ok || message.Presentation != ports.PushInformational || message.Title != test.title || message.Body != test.body {
			t.Fatalf("locale=%s message=%+v ok=%v", test.locale, message, ok)
		}
	}
}

func TestNudgePushIdentifiesSenderPathAndLocalizedPreset(t *testing.T) {
	for _, test := range []struct {
		locale, title string
		preset        socialdomain.NudgePreset
		body          string
	}{
		{"en", "New encouragement", socialdomain.NudgeYouHaveGotThis, "Alex encouraged you on Lectura: “You’ve got this!”"},
		{"en", "New encouragement", socialdomain.NudgeLetsGo, "Alex encouraged you on Lectura: “Let’s go!”"},
		{"en", "New encouragement", socialdomain.NudgeLittleProgressCounts, "Alex encouraged you on Lectura: “A little progress counts.”"},
		{"en", "New encouragement", socialdomain.NudgeKeepItGoing, "Alex encouraged you on Lectura: “Keep it going!”"},
		{"en", "New encouragement", socialdomain.NudgeTimeToWork, "Alex encouraged you on Lectura: “Time to put in some work!”"},
		{"es", "Nuevo ánimo", socialdomain.NudgeYouHaveGotThis, "Alex te animó en Lectura: «¡Tú puedes!»"},
		{"es", "Nuevo ánimo", socialdomain.NudgeLetsGo, "Alex te animó en Lectura: «¡Vamos!»"},
		{"es", "Nuevo ánimo", socialdomain.NudgeLittleProgressCounts, "Alex te animó en Lectura: «Un poco de progreso cuenta.»"},
		{"es", "Nuevo ánimo", socialdomain.NudgeKeepItGoing, "Alex te animó en Lectura: «¡Sigue así!»"},
		{"es", "Nuevo ánimo", socialdomain.NudgeTimeToWork, "Alex te animó en Lectura: «¡Es hora de ponerse manos a la obra!»"},
	} {
		t.Run(test.locale+"/"+string(test.preset), func(t *testing.T) {
			projection := pushWorkerProjection(pathapp.NotificationNudgeReceived, pathapp.NotificationInformational)
			projection.InvitationID, projection.OfferedRole = "", ""
			projection.NudgeContent = socialdomain.NudgeContent{Kind: socialdomain.NudgeContentPreset, Preset: test.preset}
			delivery := pushWorkerDelivery(1, "")
			delivery.Locale = test.locale
			message, ok := localizedPushMessage(delivery, projection)
			if !ok || message.Presentation != ports.PushInformational || message.Title != test.title || message.Body != test.body {
				t.Fatalf("message=%+v ok=%v", message, ok)
			}
			if strings.Contains(message.Body, projection.ID) || strings.Contains(message.Body, string(projection.PathID)) {
				t.Fatalf("push leaked internal identifiers: %+v", message)
			}
		})
	}

	invalid := pushWorkerProjection(pathapp.NotificationNudgeReceived, pathapp.NotificationInformational)
	invalid.InvitationID, invalid.OfferedRole = "", ""
	invalid.NudgeContent = socialdomain.NudgeContent{Kind: "custom", Preset: socialdomain.NudgeLetsGo}
	if message, ok := localizedPushMessage(pushWorkerDelivery(1, ""), invalid); ok || message != (ports.PushMessage{}) {
		t.Fatalf("custom nudge unexpectedly produced push: %+v", message)
	}
}

func TestMemberRemovalPushExplainsRoleSpecificAccessLossQuietly(t *testing.T) {
	projection := pushWorkerProjection(pathapp.NotificationPathMemberRemoved, pathapp.NotificationInformational)
	projection.InvitationID = ""
	for _, test := range []struct {
		locale string
		role   pathdomain.MembershipRole
		title  string
		body   string
	}{
		{"en", pathdomain.RoleParticipant, "Path access removed", "Alex removed your participant access to Lectura. Your activity and progress on this Path were removed."},
		{"en", pathdomain.RoleSupporter, "Path access removed", "Alex removed your supporter access to Lectura."},
		{"es", pathdomain.RoleParticipant, "Acceso a la ruta eliminado", "Alex eliminó tu acceso de participante a Lectura. Tu actividad y progreso en esta ruta se eliminaron."},
		{"es", pathdomain.RoleSupporter, "Acceso a la ruta eliminado", "Alex eliminó tu acceso como persona de apoyo a Lectura."},
	} {
		projection.OfferedRole = test.role
		delivery := pushWorkerDelivery(1, "")
		delivery.Locale = test.locale
		message, ok := localizedPushMessage(delivery, projection)
		if !ok || message.Presentation != ports.PushInformational || message.Title != test.title || message.Body != test.body {
			t.Fatalf("locale=%s message=%+v ok=%v", test.locale, message, ok)
		}
	}
	projection.OfferedRole = pathdomain.RoleAdministrator
	if message, ok := localizedPushMessage(pushWorkerDelivery(1, ""), projection); ok || message != (ports.PushMessage{}) {
		t.Fatalf("invalid removed role unexpectedly produced push: %+v", message)
	}
}

func TestEligibleFeedEventCommentPushIsDeliveredWithSourceNeutralLocalizedCopy(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct{ locale, title, body string }{
		{"en", "New comment", "Alex commented on Lectura."},
		{"es", "Nuevo comentario", "Alex comentó en Lectura."},
	} {
		t.Run(test.locale, func(t *testing.T) {
			delivery := pushWorkerDelivery(1, "")
			delivery.Locale = test.locale
			repository := &pushWorkerRepository{claims: []ports.PushDelivery{delivery}}
			provider := &pushWorkerProvider{ticket: ports.PushTicket{State: ports.PushDelivered}}
			projection := pushWorkerProjection(pathapp.NotificationPracticeComment, pathapp.NotificationInformational)
			projection.InvitationID, projection.OfferedRole = "", ""
			projection.SocialFeedEventID, projection.CommentID = "achievement:goal-1", "comment-1"
			worker := testPushWorker(now, repository, provider, &pushWorkerNotifications{item: projection})

			processed, err := worker.RunOnce(context.Background())
			if err != nil || processed != 1 || len(provider.messages) != 1 {
				t.Fatalf("processed=%d err=%v messages=%+v transitions=%+v", processed, err, provider.messages, repository.transitions)
			}
			message := provider.messages[0]
			if message.Title != test.title || message.Body != test.body || message.Presentation != ports.PushInformational ||
				len(repository.transitions) != 1 || repository.transitions[0].Outcome != ports.PushDeliveryDelivered {
				t.Fatalf("message=%+v transitions=%+v", message, repository.transitions)
			}
		})
	}
}

func TestFeedEventCommentPushRejectsMalformedSubjectBeforeProviderDisclosure(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	for _, mutate := range []func(*pathapp.InvitationNotificationProjection){
		func(projection *pathapp.InvitationNotificationProjection) { projection.CommentID = "" },
		func(projection *pathapp.InvitationNotificationProjection) { projection.SocialFeedEventID = "" },
		func(projection *pathapp.InvitationNotificationProjection) {
			projection.Reaction = socialdomain.ReactionHeart
		},
	} {
		repository := &pushWorkerRepository{claims: []ports.PushDelivery{pushWorkerDelivery(1, "")}}
		provider := &pushWorkerProvider{ticket: ports.PushTicket{State: ports.PushDelivered}}
		projection := pushWorkerProjection(pathapp.NotificationPracticeComment, pathapp.NotificationInformational)
		projection.InvitationID, projection.OfferedRole = "", ""
		projection.SocialFeedEventID, projection.CommentID = "practice:activity-1", "comment-1"
		mutate(&projection)
		worker := testPushWorker(now, repository, provider, &pushWorkerNotifications{item: projection})

		if processed, err := worker.RunOnce(context.Background()); err != nil || processed != 1 {
			t.Fatalf("processed=%d err=%v", processed, err)
		}
		if len(provider.messages) != 0 || len(repository.transitions) != 1 ||
			repository.transitions[0].Outcome != ports.PushDeliverySuppressed || repository.transitions[0].FailureCode != "notification_ineligible" {
			t.Fatalf("messages=%+v transitions=%+v", provider.messages, repository.transitions)
		}
	}
}

func TestCommentHeartPushUsesDedicatedLocalizedCopy(t *testing.T) {
	projection := pushWorkerProjection(pathapp.NotificationCommentHeart, pathapp.NotificationInformational)
	projection.InvitationID, projection.OfferedRole = "", ""
	projection.SocialFeedEventID, projection.CommentID = "practice:activity", "comment-1"
	for _, test := range []struct{ locale, title, body string }{
		{"en", "New comment heart", "Alex hearted your comment on Lectura."},
		{"es", "Nuevo corazón", "Alex marcó tu comentario con un corazón en Lectura."},
	} {
		delivery := pushWorkerDelivery(1, "")
		delivery.Locale = test.locale
		message, ok := localizedPushMessage(delivery, projection)
		if !ok || message.Presentation != ports.PushInformational || message.Title != test.title || message.Body != test.body {
			t.Fatalf("locale=%s message=%+v ok=%v", test.locale, message, ok)
		}
	}
}

func TestLocalizedPushMessageKeepsTransferDeclineAndCancellationInApplicationOnly(t *testing.T) {
	for _, kind := range []pathapp.InvitationNotificationKind{
		pathapp.NotificationPathOwnershipTransferDeclined,
		pathapp.NotificationPathOwnershipTransferCanceled,
	} {
		if message, ok := localizedPushMessage(pushWorkerDelivery(1, ""), pushWorkerProjection(kind, pathapp.NotificationInformational)); ok || message != (ports.PushMessage{}) {
			t.Fatalf("%s unexpectedly produced push message %+v", kind, message)
		}
	}
}

func TestPushDeliveryWorkerPollsReceiptAndAuditsTerminalDeliveryWithoutResolvingCopy(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repository := &pushWorkerRepository{claims: []ports.PushDelivery{pushWorkerDelivery(2, "receipt-1")}}
	provider := &pushWorkerProvider{receipts: map[ports.PushReceiptID]ports.PushReceipt{
		"receipt-1": {ReceiptID: "receipt-1", State: ports.PushDelivered},
	}}
	notifications := &pushWorkerNotifications{}
	worker := testPushWorker(now, repository, provider, notifications)

	if processed, err := worker.RunOnce(shared.WithCorrelationID(context.Background(), "push-run")); err != nil || processed != 1 {
		t.Fatalf("RunOnce() = %d, %v", processed, err)
	}
	if notifications.calls != 0 || len(provider.messages) != 0 || len(provider.receiptCalls) != 1 {
		t.Fatalf("resolver=%d sends=%d receiptCalls=%d", notifications.calls, len(provider.messages), len(provider.receiptCalls))
	}
	transition, event := repository.transitions[0], repository.audits[0]
	if transition.Outcome != ports.PushDeliveryDelivered || !event.Valid() ||
		event.OwnerUserID != "recipient-1" || event.ActorUserID != "recipient-1" ||
		event.Action != audit.ResourceUpdated || event.TargetType != "notification_push_delivery" ||
		event.TargetID != "notification-1/installation-1" || event.CorrelationID != "push-run" {
		t.Fatalf("terminal transition=%+v event=%+v", transition, event)
	}
}

func TestPushDeliveryWorkerSuppressesResolvedNotificationBeforeContentDisclosure(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repository := &pushWorkerRepository{claims: []ports.PushDelivery{pushWorkerDelivery(1, "")}}
	provider := &pushWorkerProvider{}
	notifications := &pushWorkerNotifications{err: ports.ErrNotFound}
	worker := testPushWorker(now, repository, provider, notifications)

	if _, err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(provider.messages) != 0 || repository.transitions[0].Outcome != ports.PushDeliverySuppressed ||
		repository.transitions[0].FailureCode != "notification_ineligible" || !repository.audits[0].Valid() {
		t.Fatalf("sends=%d transitions=%+v audits=%+v", len(provider.messages), repository.transitions, repository.audits)
	}
}

func TestPushDeliveryWorkerRetriesCapsAttemptsAndDisablesUnregisteredDevice(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	projection := pushWorkerProjection(pathapp.NotificationPathInvitationAccepted, pathapp.NotificationInformational)

	t.Run("bounded exponential retry", func(t *testing.T) {
		repository := &pushWorkerRepository{claims: []ports.PushDelivery{pushWorkerDelivery(3, "")}}
		provider := &pushWorkerProvider{sendErr: errors.New("network unavailable")}
		worker := testPushWorker(now, repository, provider, &pushWorkerNotifications{item: projection})
		if _, err := worker.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		transition := repository.transitions[0]
		if transition.Outcome != ports.PushDeliveryRetry ||
			!transition.AvailableAt.Equal(now.Add(4*time.Second)) ||
			transition.FailureCode != "provider_unavailable" {
			t.Fatalf("retry = %+v", transition)
		}
	})

	t.Run("max attempts terminal", func(t *testing.T) {
		repository := &pushWorkerRepository{claims: []ports.PushDelivery{pushWorkerDelivery(4, "")}}
		provider := &pushWorkerProvider{sendErr: errors.New("network unavailable")}
		worker := testPushWorker(now, repository, provider, &pushWorkerNotifications{item: projection})
		if _, err := worker.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if repository.transitions[0].Outcome != ports.PushDeliveryPermanentlyFailed ||
			repository.transitions[0].FailureCode != "max_attempts_exhausted" ||
			!repository.audits[0].Valid() {
			t.Fatalf("terminal = %+v audit=%+v", repository.transitions, repository.audits)
		}
	})

	t.Run("device not registered", func(t *testing.T) {
		repository := &pushWorkerRepository{claims: []ports.PushDelivery{pushWorkerDelivery(1, "")}}
		provider := &pushWorkerProvider{ticket: ports.PushTicket{
			State:   ports.PushFailed,
			Failure: &ports.PushFailure{Code: "DeviceNotRegistered", Disposition: ports.PushPermanent, DisableDevice: true},
		}}
		worker := testPushWorker(now, repository, provider, &pushWorkerNotifications{item: projection})
		if _, err := worker.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(repository.disabled) != 1 || repository.disabled[0] != "installation-1" ||
			!repository.audits[0].Valid() || repository.audits[0].Action != audit.ResourceDeleted ||
			repository.audits[0].ActorUserID != "recipient-1" {
			t.Fatalf("disabled=%v audits=%+v", repository.disabled, repository.audits)
		}
	})
}

func TestPushDeliveryWorkerErrorsNeverExposeTokenOrNotificationCopy(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repository := &pushWorkerRepository{
		claims:        []ports.PushDelivery{pushWorkerDelivery(1, "")},
		transitionErr: errors.New("ExponentPushToken[private] Reading private path"),
	}
	provider := &pushWorkerProvider{sendErr: errors.New("ExponentPushToken[private] Reading private path")}
	worker := testPushWorker(now, repository, provider, &pushWorkerNotifications{item: pushWorkerProjection(
		pathapp.NotificationPathInvitationReceived, pathapp.NotificationActionable,
	)})
	_, err := worker.RunOnce(context.Background())
	if err == nil || strings.Contains(err.Error(), "ExponentPushToken") || strings.Contains(err.Error(), "Reading") {
		t.Fatalf("unsafe worker error = %v", err)
	}
}

func testPushWorker(
	now time.Time,
	repository *pushWorkerRepository,
	provider *pushWorkerProvider,
	notifications *pushWorkerNotifications,
) PushDeliveryWorker {
	return PushDeliveryWorker{
		Repository: repository, Provider: provider, Notifications: notifications,
		Clock: pushWorkerClock{now: now}, WorkerID: "worker-1",
		Lease: time.Minute, ReceiptDelay: 30 * time.Second,
		BaseRetryDelay: time.Second, MaxRetryDelay: 8 * time.Second,
		MaxAttempts: 4, BatchSize: 10,
	}
}

func pushWorkerDelivery(attempts int, ticket string) ports.PushDelivery {
	return ports.PushDelivery{
		NotificationID: "notification-1", InstallationID: "installation-1",
		RecipientUserID: "recipient-1", Provider: "expo", Platform: "ios",
		Locale: "en", Token: "ExponentPushToken[private]", Attempts: attempts,
		CreatedAt: time.Date(2026, time.July, 23, 11, 0, 0, 0, time.UTC),
		LockedBy:  "worker-1", LockedUntil: time.Date(2026, time.July, 23, 12, 1, 0, 0, time.UTC),
		ProviderTicket: ticket,
	}
}

func pushWorkerProjection(
	kind pathapp.InvitationNotificationKind,
	presentation pathapp.NotificationPresentation,
) pathapp.InvitationNotificationProjection {
	projection := pathapp.InvitationNotificationProjection{
		ID: "notification-1", Kind: kind, Presentation: presentation,
		CreatedAt: time.Date(2026, time.July, 23, 11, 0, 0, 0, time.UTC),
		Actor: pathapp.InvitationPublicIdentity{
			UserID: "actor-1", Username: "alex", DisplayName: "Alex",
		},
		PathID: "path-1", PathName: "Lectura",
	}
	if kind == pathapp.NotificationPathOwnershipTransferReceived || kind == pathapp.NotificationPathOwnershipTransferAccepted ||
		kind == pathapp.NotificationPathOwnershipTransferDeclined || kind == pathapp.NotificationPathOwnershipTransferCanceled {
		projection.OwnershipTransferID = "transfer-1"
	} else {
		projection.InvitationID = "invitation-1"
		projection.OfferedRole = pathdomain.RoleParticipant
	}
	return projection
}
