package gormstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestActivateOnboardingCommitsAggregateCredentialRotationAndAuditsAtomically(t *testing.T) {
	store, tx, ctx := activationTestStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := newTestID()
	oldHash := activationHash(1)
	siblingHash := activationHash(2)
	absoluteExpiry := now.Add(4 * time.Hour)
	seedProvisionalActivationUser(t, tx, userID, oldHash, siblingHash, now, absoluteExpiry)

	activation := validActivation(userID, "timekeeper", now)
	newRecord := ports.SessionRecord{TokenHash: activationHash(3), UserID: userID, Scopes: []string{"api:user"}, ExpiresAt: now.Add(8 * time.Hour)}
	completed, revoked, created := activationAudits(userID, now)

	actualExpiry, err := store.ActivateOnboarding(ctx, activation, oldHash, now, newRecord, completed, revoked, created)
	if err != nil {
		t.Fatal(err)
	}
	if !actualExpiry.Equal(absoluteExpiry) {
		t.Fatalf("new session expiry = %s, want family absolute expiry %s", actualExpiry, absoluteExpiry)
	}

	var user struct {
		Status            identity.Status
		Username          *string
		DisplayName       string
		ProfileVisibility *identity.ProfileVisibility
	}
	if err := tx.Table("user_models").Where("id = ?", userID).Take(&user).Error; err != nil {
		t.Fatal(err)
	}
	if user.Status != identity.StatusActive || user.Username == nil || *user.Username != activation.Username || user.DisplayName != activation.DisplayName || user.ProfileVisibility == nil || *user.ProfileVisibility != activation.ProfileVisibility {
		t.Fatalf("activated user = %+v", user)
	}

	for table := range map[string]struct{}{
		"user_account_activation_models": {},
		"user_preference_models":         {},
		"user_time_zone_history_models":  {},
	} {
		assertActivationRowCount(t, tx, table, userID, 1)
	}
	assertActivationRowCount(t, tx, "user_policy_acceptance_models", userID, 3)
	var activationEvidence userAccountActivationModel
	if err := tx.Where("user_id = ?", userID).Take(&activationEvidence).Error; err != nil {
		t.Fatal(err)
	}
	if activationEvidence.PolicySetRevision != 7 {
		t.Fatalf("activation policy revision = %d, want 7", activationEvidence.PolicySetRevision)
	}

	var liveOnboarding, liveApplication int64
	if err := tx.Model(&sessionModel{}).Where("user_id = ? AND scopes = ? AND revoked_at IS NULL", userID, "api:onboarding").Count(&liveOnboarding).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&sessionModel{}).Where("user_id = ? AND scopes = ? AND revoked_at IS NULL", userID, "api:user").Count(&liveApplication).Error; err != nil {
		t.Fatal(err)
	}
	if liveOnboarding != 0 || liveApplication != 1 {
		t.Fatalf("live sessions: onboarding=%d application=%d", liveOnboarding, liveApplication)
	}
	var newSession sessionModel
	if err := tx.Where("token_hash = ?", newRecord.TokenHash).Take(&newSession).Error; err != nil {
		t.Fatal(err)
	}
	if !newSession.ExpiresAt.Equal(absoluteExpiry) || !newSession.AbsoluteExpiresAt.Equal(absoluteExpiry) {
		t.Fatalf("new session expiry=%s absolute=%s", newSession.ExpiresAt, newSession.AbsoluteExpiresAt)
	}
	for _, event := range []audit.Event{completed, revoked, created} {
		var count int64
		if err := tx.Model(&auditEventModel{}).Where("id = ? AND action = ?", event.ID, event.Action).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("audit %q count = %d", event.Action, count)
		}
	}
}

func TestActivationPersistenceAcceptsOneEarlierApplicationEventInstant(t *testing.T) {
	eventTime := time.Date(2026, 7, 21, 18, 0, 0, 0, time.UTC)
	validationTime := eventTime.Add(25 * time.Millisecond)
	userID := newTestID()
	activation := validActivation(userID, "advancing_clock", eventTime)
	completed, revoked, created := activationAudits(userID, eventTime)
	record := ports.SessionRecord{TokenHash: activationHash(91), UserID: userID, Scopes: []string{"api:user"}, ExpiresAt: validationTime.Add(time.Hour)}

	if err := validateActivationPersistenceInput(activation, activationHash(90), validationTime, record, completed, revoked, created); err != nil {
		t.Fatalf("an advancing production clock invalidated one logical activation instant: %v", err)
	}
}

func TestActivateOnboardingRollsBackEveryWriteWhenNewCredentialCannotBeCreated(t *testing.T) {
	store, tx, ctx := activationTestStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := newTestID()
	oldHash := activationHash(11)
	conflictingHash := activationHash(12)
	seedProvisionalActivationUser(t, tx, userID, oldHash, nil, now, now.Add(4*time.Hour))
	otherUserID := newTestID()
	if err := tx.Create(&userModel{ID: otherUserID, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&sessionModel{TokenHash: conflictingHash, UserID: otherUserID, Scopes: "api:user", ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(2 * time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	completed, revoked, created := activationAudits(userID, now)

	_, err := store.ActivateOnboarding(ctx, validActivation(userID, "rollback_user", now), oldHash, now, ports.SessionRecord{TokenHash: conflictingHash, UserID: userID, Scopes: []string{"api:user"}, ExpiresAt: now.Add(time.Hour)}, completed, revoked, created)
	if err == nil {
		t.Fatal("activation unexpectedly succeeded with duplicate new credential hash")
	}

	var status identity.Status
	if err := tx.Model(&userModel{}).Select("status").Where("id = ?", userID).Scan(&status).Error; err != nil {
		t.Fatal(err)
	}
	if status != identity.StatusProvisional {
		t.Fatalf("status = %q after rollback", status)
	}
	for _, table := range []string{"user_account_activation_models", "user_policy_acceptance_models", "user_preference_models", "user_time_zone_history_models"} {
		assertActivationRowCount(t, tx, table, userID, 0)
	}
	var old sessionModel
	if err := tx.Where("token_hash = ?", oldHash).Take(&old).Error; err != nil {
		t.Fatal(err)
	}
	if old.RevokedAt != nil {
		t.Fatal("presented onboarding credential was revoked despite transaction rollback")
	}
	for _, event := range []audit.Event{completed, revoked, created} {
		var count int64
		if err := tx.Model(&auditEventModel{}).Where("id = ?", event.ID).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("audit %q survived rollback", event.Action)
		}
	}
}

func TestActivateOnboardingRejectsAStaleSharedPolicyRevisionBeforeAnyMutation(t *testing.T) {
	store, tx, ctx := activationTestStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := newTestID()
	oldHash := activationHash(15)
	seedProvisionalActivationUser(t, tx, userID, oldHash, nil, now, now.Add(4*time.Hour))
	activation := validActivation(userID, "stale_policy", now)
	activation.PolicySetRevision = 6
	completed, revoked, created := activationAudits(userID, now)

	_, err := store.ActivateOnboarding(ctx, activation, oldHash, now, ports.SessionRecord{
		TokenHash: activationHash(16), UserID: userID, Scopes: []string{"api:user"}, ExpiresAt: now.Add(time.Hour),
	}, completed, revoked, created)
	if !errors.Is(err, ports.ErrPolicySetChanged) {
		t.Fatalf("activation error = %v, want ErrPolicySetChanged", err)
	}
	for _, table := range []string{"user_account_activation_models", "user_policy_acceptance_models", "user_preference_models", "user_time_zone_history_models"} {
		assertActivationRowCount(t, tx, table, userID, 0)
	}
	assertLiveActivationSessionCounts(t, tx, userID, 1, 0)
}

func TestActivateOnboardingMapsCaseInsensitiveUsernameCollisionToConflict(t *testing.T) {
	store, tx, ctx := activationTestStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	claimed := "Claimed_Name"
	if err := tx.Create(&userModel{ID: newTestID(), Username: &claimed, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	userID := newTestID()
	oldHash := activationHash(21)
	seedProvisionalActivationUser(t, tx, userID, oldHash, nil, now, now.Add(4*time.Hour))
	completed, revoked, created := activationAudits(userID, now)

	_, err := store.ActivateOnboarding(ctx, validActivation(userID, "claimed_name", now), oldHash, now, ports.SessionRecord{TokenHash: activationHash(22), UserID: userID, Scopes: []string{"api:user"}, ExpiresAt: now.Add(time.Hour)}, completed, revoked, created)
	if !errors.Is(err, ports.ErrUsernameUnavailable) {
		t.Fatalf("ActivateOnboarding returned %v, want ErrUsernameUnavailable", err)
	}
	assertActivationRowCount(t, tx, "user_account_activation_models", userID, 0)
}

func TestConcurrentActivationOfOneProvisionalUserConsumesOnboardingExactlyOnce(t *testing.T) {
	firstStore, secondStore, inspect := activationRaceStores(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := newTestID()
	firstOldHash := activationRaceHash("same-user-first-old")
	secondOldHash := activationRaceHash("same-user-second-old")
	absoluteExpiry := now.Add(4 * time.Hour)
	seedProvisionalActivationUser(t, inspect, userID, firstOldHash, secondOldHash, now, absoluteExpiry)

	type attempt struct {
		err       error
		newHash   []byte
		completed audit.Event
		revoked   audit.Event
		created   audit.Event
	}
	attempts := []attempt{
		{newHash: activationRaceHash("same-user-first-new")},
		{newHash: activationRaceHash("same-user-second-new")},
	}
	stores := []*Store{firstStore, secondStore}
	oldHashes := [][]byte{firstOldHash, secondOldHash}
	start := make(chan struct{})
	var workers sync.WaitGroup
	for index := range attempts {
		attempts[index].completed, attempts[index].revoked, attempts[index].created = activationAudits(userID, now)
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			_, attempts[index].err = stores[index].ActivateOnboarding(
				context.Background(),
				validActivation(userID, "single_winner", now),
				oldHashes[index],
				now,
				ports.SessionRecord{TokenHash: attempts[index].newHash, UserID: userID, Scopes: []string{"api:user"}, ExpiresAt: now.Add(time.Hour)},
				attempts[index].completed,
				attempts[index].revoked,
				attempts[index].created,
			)
		}(index)
	}
	close(start)
	workers.Wait()

	succeeded, stale := 0, 0
	for index := range attempts {
		switch {
		case attempts[index].err == nil:
			succeeded++
		case errors.Is(attempts[index].err, ports.ErrNotFound):
			stale++
		default:
			t.Fatalf("activation attempt %d returned %v, want success or stale ErrNotFound", index, attempts[index].err)
		}
	}
	if succeeded != 1 || stale != 1 {
		t.Fatalf("concurrent activation outcomes: succeeded=%d stale=%d", succeeded, stale)
	}

	var user userModel
	if err := inspect.Where("id = ?", userID).Take(&user).Error; err != nil {
		t.Fatal(err)
	}
	if user.Status != identity.StatusActive {
		t.Fatalf("user status = %q, want active", user.Status)
	}
	assertActivationRowCount(t, inspect, "user_account_activation_models", userID, 1)
	assertActivationRowCount(t, inspect, "user_policy_acceptance_models", userID, 3)
	assertActivationRowCount(t, inspect, "user_preference_models", userID, 1)
	assertActivationRowCount(t, inspect, "user_time_zone_history_models", userID, 1)
	assertLiveActivationSessionCounts(t, inspect, userID, 0, 1)

	var completedAudits, revokedAudits, createdAudits int64
	for action, destination := range map[audit.Action]*int64{
		audit.UserOnboardingCompleted: &completedAudits,
		audit.SessionRevoked:          &revokedAudits,
		audit.SessionCreated:          &createdAudits,
	} {
		if err := inspect.Model(&auditEventModel{}).Where("owner_user_id = ? AND action = ?", userID, action).Count(destination).Error; err != nil {
			t.Fatal(err)
		}
	}
	if completedAudits != 1 || revokedAudits != 1 || createdAudits != 1 {
		t.Fatalf("activation audits duplicated: completed=%d revoked=%d created=%d", completedAudits, revokedAudits, createdAudits)
	}
}

func TestConcurrentCaseVariantUsernameClaimsHaveOneWinnerAndPreserveLoser(t *testing.T) {
	firstStore, secondStore, inspect := activationRaceStores(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	userIDs := []string{newTestID(), newTestID()}
	oldHashes := [][]byte{activationRaceHash("case-first-old"), activationRaceHash("case-second-old")}
	newHashes := [][]byte{activationRaceHash("case-first-new"), activationRaceHash("case-second-new")}
	for index := range userIDs {
		seedProvisionalActivationUser(t, inspect, userIDs[index], oldHashes[index], nil, now, now.Add(4*time.Hour))
	}

	usernames := []string{"Race_Claim", "race_claim"}
	stores := []*Store{firstStore, secondStore}
	errorsByAttempt := make([]error, len(stores))
	start := make(chan struct{})
	var workers sync.WaitGroup
	for index := range stores {
		completed, revoked, created := activationAudits(userIDs[index], now)
		workers.Add(1)
		go func(index int, completed, revoked, created audit.Event) {
			defer workers.Done()
			<-start
			_, errorsByAttempt[index] = stores[index].ActivateOnboarding(
				context.Background(),
				validActivation(userIDs[index], usernames[index], now),
				oldHashes[index],
				now,
				ports.SessionRecord{TokenHash: newHashes[index], UserID: userIDs[index], Scopes: []string{"api:user"}, ExpiresAt: now.Add(time.Hour)},
				completed,
				revoked,
				created,
			)
		}(index, completed, revoked, created)
	}
	close(start)
	workers.Wait()

	winner, loser := -1, -1
	for index, err := range errorsByAttempt {
		switch {
		case err == nil:
			winner = index
		case errors.Is(err, ports.ErrUsernameUnavailable):
			loser = index
		default:
			t.Fatalf("username activation attempt %d returned %v, want success or ErrConflict", index, err)
		}
	}
	if winner == -1 || loser == -1 || winner == loser {
		t.Fatalf("concurrent username outcomes = %v, want exactly one winner and one conflict", errorsByAttempt)
	}

	var winningUser, losingUser userModel
	if err := inspect.Where("id = ?", userIDs[winner]).Take(&winningUser).Error; err != nil {
		t.Fatal(err)
	}
	if err := inspect.Where("id = ?", userIDs[loser]).Take(&losingUser).Error; err != nil {
		t.Fatal(err)
	}
	if winningUser.Status != identity.StatusActive || losingUser.Status != identity.StatusProvisional {
		t.Fatalf("race lifecycle states: winner=%q loser=%q", winningUser.Status, losingUser.Status)
	}
	assertActivationRowCount(t, inspect, "user_account_activation_models", userIDs[winner], 1)
	assertActivationRowCount(t, inspect, "user_account_activation_models", userIDs[loser], 0)
	assertActivationRowCount(t, inspect, "user_policy_acceptance_models", userIDs[loser], 0)
	assertActivationRowCount(t, inspect, "user_preference_models", userIDs[loser], 0)
	assertActivationRowCount(t, inspect, "user_time_zone_history_models", userIDs[loser], 0)
	assertLiveActivationSessionCounts(t, inspect, userIDs[winner], 0, 1)
	assertLiveActivationSessionCounts(t, inspect, userIDs[loser], 1, 0)
}

func activationRaceStores(t *testing.T) (*Store, *Store, *gorm.DB) {
	t.Helper()
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	seedActivationPolicyAuthority(t, context.Background())
	stores := make([]*Store, 3)
	for index := range stores {
		store, err := Open("postgres", *postgresTestDSN)
		if err != nil {
			t.Fatal(err)
		}
		stores[index] = store
		sqlDB, err := store.DB.DB()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return stores[0], stores[1], stores[2].DB.WithContext(context.Background())
}

func activationRaceHash(label string) []byte {
	hash := sha256.Sum256([]byte(label + newTestID()))
	return hash[:]
}

func assertLiveActivationSessionCounts(t *testing.T, db *gorm.DB, userID string, wantOnboarding, wantApplication int64) {
	t.Helper()
	var onboarding, application int64
	if err := db.Model(&sessionModel{}).Where("user_id = ? AND scopes = ? AND revoked_at IS NULL", userID, "api:onboarding").Count(&onboarding).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&sessionModel{}).Where("user_id = ? AND scopes = ? AND revoked_at IS NULL", userID, "api:user").Count(&application).Error; err != nil {
		t.Fatal(err)
	}
	if onboarding != wantOnboarding || application != wantApplication {
		t.Fatalf("live sessions for %s: onboarding=%d application=%d, want onboarding=%d application=%d", userID, onboarding, application, wantOnboarding, wantApplication)
	}
}

func activationTestStore(t *testing.T) (*Store, *gorm.DB, context.Context) {
	t.Helper()
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	ctx := context.Background()
	seedActivationPolicyAuthority(t, ctx)

	base, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	tx := base.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	return &Store{DB: tx}, tx, ctx
}

func seedActivationPolicyAuthority(t *testing.T, ctx context.Context) {
	t.Helper()
	policy := currentPolicySetModel{
		Singleton: true, Revision: 7, TermsVersion: "terms-v1", PrivacyPolicyVersion: "privacy-v1",
		CommunityGuidelinesVersion: "guidelines-v1", TermsURL: "https://app.example/terms",
		PrivacyPolicyURL: "https://app.example/privacy", CommunityGuidelinesURL: "https://app.example/guidelines",
		SupportURL: "https://app.example/support", UpdatedAt: time.Date(2026, 7, 21, 15, 0, 0, 0, time.UTC),
	}
	migrationStore, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "singleton"}},
		DoUpdates: clause.AssignmentColumns([]string{"revision", "terms_version", "privacy_policy_version", "community_guidelines_version", "terms_url", "privacy_policy_url", "community_guidelines_url", "support_url", "updated_at"}),
	}).Create(&policy).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = migrationStore.DB.Where("singleton = ? AND revision = ?", true, policy.Revision).Delete(&currentPolicySetModel{}).Error
	})
}

func activationHash(marker byte) []byte {
	hash := make([]byte, 32)
	hash[0] = marker
	return hash
}

func seedProvisionalActivationUser(t *testing.T, tx *gorm.DB, userID string, presentedHash, siblingHash []byte, now, absoluteExpiry time.Time) {
	t.Helper()
	if err := tx.Create(&userModel{ID: userID, Status: identity.StatusProvisional, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	for _, hash := range [][]byte{presentedHash, siblingHash} {
		if hash == nil {
			continue
		}
		if err := tx.Create(&sessionModel{TokenHash: hash, UserID: userID, Scopes: "api:onboarding", ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: absoluteExpiry}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func validActivation(userID, username string, now time.Time) identity.OnboardingActivation {
	return identity.OnboardingActivation{
		PolicySetRevision: 7,
		UserID:            userID,
		Username:          username,
		DisplayName:       "App profile name",
		ProfileVisibility: identity.ProfileVisibilityPrivate,
		TimeZone:          identity.IANATimeZone("America/New_York"),
		FirstDayOfWeek:    identity.FirstDayMonday,
		AgeAttestation:    identity.MinimumAgeAttestedAtLeast16,
		PolicyAcceptance: identity.PolicyAcceptance{
			TermsOfServiceAcceptedVersion:      "terms-v1",
			PrivacyPolicyAcknowledgedVersion:   "privacy-v1",
			CommunityGuidelinesAcceptedVersion: "guidelines-v1",
			AcceptedAt:                         now,
		},
	}
}

func activationAudits(userID string, now time.Time) (audit.Event, audit.Event, audit.Event) {
	correlationID := newTestID()
	event := func(action audit.Action) audit.Event {
		return audit.Event{ID: newTestID(), OwnerUserID: userID, ActorUserID: userID, Action: action, TargetType: "user", TargetID: userID, Outcome: audit.Succeeded, CorrelationID: correlationID, OccurredAt: now}
	}
	return event(audit.UserOnboardingCompleted), event(audit.SessionRevoked), event(audit.SessionCreated)
}

func assertActivationRowCount(t *testing.T, tx *gorm.DB, table, userID string, want int64) {
	t.Helper()
	var count int64
	if err := tx.Table(table).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("%s rows = %d, want %d", table, count, want)
	}
}
