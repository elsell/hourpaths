package activitystore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestEveryActivityMutationUsesCurrentMembershipGuard(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	deletion, err := os.ReadFile("deletion.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"lockActivePathAt(tx, command.Timer.PathID, command.Timer.ParticipantID, command.Timer.StartedAt)",
		"lockActivePath(tx, command.PathID, command.ParticipantID)",
		"lockActivePathAt(tx, entry.PathID, entry.ParticipantID, entry.StartedAt)",
	} {
		if !strings.Contains(string(repository), required) {
			t.Errorf("activity repository missing guard %q", required)
		}
	}
	if !strings.Contains(string(deletion), "lockActivePath(tx, command.PathID, command.ParticipantID)") {
		t.Errorf("activity deletion missing membership guard")
	}
}

func TestActivityMembershipGuardLocksProgressBeforeRows(t *testing.T) {
	source, err := os.ReadFile("path_state.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	progress := strings.Index(text, "progresslock.Lock(tx, participantID, pathID)")
	pathRow := strings.Index(text, `tx.Table("path_models")`)
	if progress < 0 || pathRow < 0 || progress > pathRow {
		t.Fatal("activity membership guard must lock progress before Path and membership rows")
	}
}

func TestPostgresV56UpgradeBackfillFencesPriorInvitationIncarnation(t *testing.T) {
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeDB, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	migrationDB, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, member, pathID := "upgrade-owner-"+suffix, "upgrade-member-"+suffix, "upgrade-path-"+suffix
	cutover := time.Now().UTC().Truncate(time.Microsecond)
	pathCreated := cutover.Add(-72 * time.Hour)
	priorAccepted, currentAccepted := cutover.Add(-48*time.Hour), cutover.Add(-time.Hour)
	seedParticipantAndPathCreatedAt(t, migrationDB, owner, pathID, pathCreated, pathCreated)
	type userRow struct {
		ID                   string `gorm:"primaryKey"`
		Status               identity.Status
		CreatedAt, UpdatedAt time.Time
	}
	if err := migrationDB.Table("user_models").Create(&userRow{ID: member, Status: identity.StatusActive, CreatedAt: pathCreated, UpdatedAt: cutover}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("path_membership_models").Create(map[string]any{"path_id": pathID, "user_id": member, "role": "participant", "joined_at": pathCreated}).Error; err != nil {
		t.Fatal(err)
	}
	for index, acceptedAt := range []time.Time{priorAccepted, currentAccepted} {
		changeID, invitationID := fmt.Sprintf("upgrade-change-%d-%s", index, suffix), fmt.Sprintf("upgrade-invitation-%d-%s", index, suffix)
		if err := migrationDB.Table("authorization_outbox_models").Create(map[string]any{"id": changeID, "resource_type": "path", "resource_id": pathID, "relation": "participant", "subject_type": "user", "subject_id": member, "owner_user_id": owner, "actor_user_id": owner, "operation": "touch", "created_at": acceptedAt}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("path_invitation_models").Create(map[string]any{"id": invitationID, "path_id": pathID, "inviter_user_id": owner, "recipient_user_id": member, "offered_role": "participant", "authorization_change_id": changeID, "created_at": acceptedAt.Add(-time.Minute), "accepted_at": acceptedAt}).Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		migrationDB.Table("path_models").Where("id = ?", pathID).Delete(&struct{}{})
		migrationDB.Table("authorization_outbox_models").Where("id LIKE ?", "upgrade-change-%-"+suffix).Delete(&struct{}{})
		migrationDB.Table("audit_event_models").Where("owner_user_id IN ?", []string{owner, member}).Delete(&struct{}{})
		migrationDB.Table("user_models").Where("id IN ?", []string{owner, member}).Delete(&struct{}{})
	})

	// This is the invitation-history branch of v56's upgrade CTE, applied to
	// the fixture after seeding its pre-v56 placeholder timestamp.
	backfilled := migrationDB.Exec(`UPDATE public.path_membership_models AS membership SET joined_at = evidence.accepted_at FROM (SELECT invitation.path_id, invitation.recipient_user_id AS user_id, max(invitation.accepted_at) AS accepted_at FROM public.path_invitation_models AS invitation WHERE invitation.accepted_at IS NOT NULL GROUP BY invitation.path_id, invitation.recipient_user_id) AS evidence WHERE membership.path_id = evidence.path_id AND membership.user_id = evidence.user_id`)
	if backfilled.Error != nil || backfilled.RowsAffected != 1 {
		t.Fatalf("upgrade backfill rows=%d err=%v", backfilled.RowsAffected, backfilled.Error)
	}
	var joinedAt time.Time
	if err := migrationDB.Table("path_membership_models").Select("joined_at").Where("path_id = ? AND user_id = ?", pathID, member).Scan(&joinedAt).Error; err != nil || !joinedAt.Equal(currentAccepted) {
		t.Fatalf("joined_at=%s want=%s err=%v", joinedAt, currentAccepted, err)
	}

	repository := New(runtimeDB)
	oldTimer, _ := domain.StartTimer("upgrade-old-timer-"+suffix, pathID, member, priorAccepted.Add(time.Minute), "Etc/UTC", cutover)
	if _, err := repository.StartTimer(context.Background(), application.StartTimerCommand{Timer: oldTimer, Idempotency: idempotency(member, application.StartTimerOperation, "upgrade-old-start-"+suffix, 111), Audit: timerAudit("upgrade-old-start-audit-"+suffix, member, oldTimer.ID, audit.ActivityTimerStarted, cutover)}); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("prior-incarnation StartTimer error=%v", err)
	}
	oldManual, _ := domain.RecordManualActivity(domain.ManualActivity{ID: "upgrade-old-manual-" + suffix, PathID: pathID, ParticipantID: member, StartedAt: priorAccepted.Add(time.Minute), DurationSeconds: 30, OccurrenceTimeZone: "Etc/UTC"}, cutover)
	if _, err := repository.CreateManualActivity(context.Background(), application.CreateManualActivityCommand{Activity: oldManual, Idempotency: idempotency(member, application.CreateManualActivityOperation, "upgrade-old-manual-key-"+suffix, 112), Audit: activityAudit("upgrade-old-manual-audit-"+suffix, member, oldManual.ID, audit.ResourceCreated, cutover)}); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("prior-incarnation manual error=%v", err)
	}

	currentTimer, _ := domain.StartTimer("upgrade-current-timer-"+suffix, pathID, member, currentAccepted.Add(time.Minute), "Etc/UTC", cutover)
	if _, err := repository.StartTimer(context.Background(), application.StartTimerCommand{Timer: currentTimer, Idempotency: idempotency(member, application.StartTimerOperation, "upgrade-current-start-"+suffix, 113), Audit: timerAudit("upgrade-current-start-audit-"+suffix, member, currentTimer.ID, audit.ActivityTimerStarted, cutover)}); err != nil {
		t.Fatalf("current-incarnation StartTimer error=%v", err)
	}
	if err := migrationDB.Table("running_timer_models").Where("id = ?", currentTimer.ID).Delete(&struct{}{}).Error; err != nil {
		t.Fatal(err)
	}
	currentManual, _ := domain.RecordManualActivity(domain.ManualActivity{ID: "upgrade-current-manual-" + suffix, PathID: pathID, ParticipantID: member, StartedAt: currentAccepted.Add(2 * time.Minute), DurationSeconds: 30, OccurrenceTimeZone: "Etc/UTC"}, cutover)
	if _, err := repository.CreateManualActivity(context.Background(), application.CreateManualActivityCommand{Activity: currentManual, Idempotency: idempotency(member, application.CreateManualActivityOperation, "upgrade-current-manual-key-"+suffix, 114), Audit: activityAudit("upgrade-current-manual-audit-"+suffix, member, currentManual.ID, audit.ResourceCreated, cutover)}); err != nil {
		t.Fatalf("current-incarnation manual error=%v", err)
	}
}

func TestPostgresV56UpgradeBackfillPreservesContinuousOwnershipTransferMembership(t *testing.T) {
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeDB, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	migrationDB, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	creator, participant, pathID := "lineage-creator-"+suffix, "lineage-participant-"+suffix, "lineage-path-"+suffix
	cutover := time.Now().UTC().Truncate(time.Microsecond)
	pathCreated, joinedAt, transferredAt := cutover.Add(-72*time.Hour), cutover.Add(-48*time.Hour), cutover.Add(-24*time.Hour)
	seedParticipantAndPathCreatedAt(t, migrationDB, creator, pathID, pathCreated, pathCreated)
	type userRow struct {
		ID                   string `gorm:"primaryKey"`
		Status               identity.Status
		CreatedAt, UpdatedAt time.Time
	}
	if err := migrationDB.Table("user_models").Create(&userRow{ID: participant, Status: identity.StatusActive, CreatedAt: pathCreated, UpdatedAt: cutover}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("path_membership_models").Create(map[string]any{"path_id": pathID, "user_id": participant, "role": "participant", "joined_at": pathCreated}).Error; err != nil {
		t.Fatal(err)
	}
	changeID, invitationID := "lineage-change-"+suffix, "lineage-invitation-"+suffix
	if err := migrationDB.Table("authorization_outbox_models").Create(map[string]any{"id": changeID, "resource_type": "path", "resource_id": pathID, "relation": "participant", "subject_type": "user", "subject_id": participant, "owner_user_id": creator, "actor_user_id": creator, "operation": "touch", "created_at": joinedAt}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("path_invitation_models").Create(map[string]any{"id": invitationID, "path_id": pathID, "inviter_user_id": creator, "recipient_user_id": participant, "offered_role": "participant", "authorization_change_id": changeID, "created_at": joinedAt.Add(-time.Minute), "accepted_at": joinedAt}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("path_ownership_transfer_models").Create(map[string]any{"id": "lineage-transfer-" + suffix, "path_id": pathID, "initiator_user_id": creator, "recipient_user_id": participant, "reviewed_at": transferredAt.Add(-2 * time.Minute), "created_at": transferredAt.Add(-time.Minute), "expires_at": transferredAt.Add(time.Hour), "accepted_at": transferredAt}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("path_models").Where("id = ?", pathID).Update("owner_user_id", participant).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", pathID, creator).Update("role", "administrator").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		migrationDB.Table("path_models").Where("id = ?", pathID).Delete(&struct{}{})
		migrationDB.Table("authorization_outbox_models").Where("id = ?", changeID).Delete(&struct{}{})
		migrationDB.Table("audit_event_models").Where("owner_user_id IN ?", []string{creator, participant}).Delete(&struct{}{})
		migrationDB.Table("user_models").Where("id IN ?", []string{creator, participant}).Delete(&struct{}{})
	})

	backfilled := migrationDB.Exec(`UPDATE public.path_membership_models AS membership SET joined_at = COALESCE((SELECT max(invitation.accepted_at) FROM public.path_invitation_models AS invitation WHERE invitation.path_id = membership.path_id AND invitation.recipient_user_id = membership.user_id AND invitation.accepted_at IS NOT NULL), CASE WHEN membership.user_id = COALESCE((SELECT transfer.initiator_user_id FROM public.path_ownership_transfer_models AS transfer WHERE transfer.path_id = membership.path_id AND transfer.accepted_at IS NOT NULL ORDER BY transfer.accepted_at, transfer.created_at, transfer.id LIMIT 1), path.owner_user_id) THEN path.created_at END, CURRENT_TIMESTAMP) FROM public.path_models AS path WHERE path.id = membership.path_id AND path.id = ?`, pathID)
	if backfilled.Error != nil || backfilled.RowsAffected != 2 {
		t.Fatalf("lineage backfill rows=%d err=%v", backfilled.RowsAffected, backfilled.Error)
	}
	var rows []struct {
		UserID   string
		JoinedAt time.Time
	}
	if err := migrationDB.Table("path_membership_models").Select("user_id, joined_at").Where("path_id = ?", pathID).Order("user_id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	want := map[string]time.Time{creator: pathCreated, participant: joinedAt}
	if len(rows) != 2 {
		t.Fatalf("lineage rows=%+v", rows)
	}
	for _, row := range rows {
		if !row.JoinedAt.Equal(want[row.UserID]) {
			t.Fatalf("%s joined_at=%s want=%s", row.UserID, row.JoinedAt, want[row.UserID])
		}
	}

	repository := New(runtimeDB)
	for index, fixture := range []struct {
		name, user string
		occurrence time.Time
	}{{"creator-to-administrator", creator, pathCreated.Add(time.Hour)}, {"participant-to-creator", participant, joinedAt.Add(time.Hour)}} {
		t.Run(fixture.name, func(t *testing.T) {
			timer, _ := domain.StartTimer(fmt.Sprintf("lineage-timer-%d-%s", index, suffix), pathID, fixture.user, fixture.occurrence, "Etc/UTC", cutover)
			if _, err := repository.StartTimer(context.Background(), application.StartTimerCommand{Timer: timer, Idempotency: idempotency(fixture.user, application.StartTimerOperation, fmt.Sprintf("lineage-start-%d-%s", index, suffix), byte(120+index)), Audit: timerAudit(fmt.Sprintf("lineage-start-audit-%d-%s", index, suffix), fixture.user, timer.ID, audit.ActivityTimerStarted, cutover)}); err != nil {
				t.Fatalf("pre-transfer current-incarnation StartTimer=%v", err)
			}
			if err := migrationDB.Table("running_timer_models").Where("id = ?", timer.ID).Delete(&struct{}{}).Error; err != nil {
				t.Fatal(err)
			}
			entry, _ := domain.RecordManualActivity(domain.ManualActivity{ID: fmt.Sprintf("lineage-manual-%d-%s", index, suffix), PathID: pathID, ParticipantID: fixture.user, StartedAt: fixture.occurrence.Add(time.Minute), DurationSeconds: 30, OccurrenceTimeZone: "Etc/UTC"}, cutover)
			if _, err := repository.CreateManualActivity(context.Background(), application.CreateManualActivityCommand{Activity: entry, Idempotency: idempotency(fixture.user, application.CreateManualActivityOperation, fmt.Sprintf("lineage-manual-key-%d-%s", index, suffix), byte(122+index)), Audit: activityAudit(fmt.Sprintf("lineage-manual-audit-%d-%s", index, suffix), fixture.user, entry.ID, audit.ResourceCreated, cutover)}); err != nil {
				t.Fatalf("pre-transfer current-incarnation manual=%v", err)
			}
		})
	}
}

func TestPostgresActivityMutationGuardClosesRemovalRaceAndRejectsPreReinviteOccurrence(t *testing.T) {
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeDB, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	migrationDB, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, member, pathID := "guard-owner-"+suffix, "guard-member-"+suffix, "guard-path-"+suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedParticipantAndPath(t, migrationDB, owner, pathID, now)
	type userRow struct {
		ID                   string `gorm:"primaryKey"`
		Status               identity.Status
		CreatedAt, UpdatedAt time.Time
	}
	if err := migrationDB.Table("user_models").Create(&userRow{ID: member, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("path_membership_models").Create(map[string]any{"path_id": pathID, "user_id": member, "role": "participant", "joined_at": now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		migrationDB.Table("path_models").Where("id = ?", pathID).Delete(&struct{}{})
		migrationDB.Table("user_models").Where("id IN ?", []string{owner, member}).Delete(&struct{}{})
	})

	removal := migrationDB.Begin()
	if removal.Error != nil {
		t.Fatal(removal.Error)
	}
	if err := removal.Table("path_models").Where("id = ?", pathID).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&struct{ ID string }{}).Error; err != nil {
		removal.Rollback()
		t.Fatal(err)
	}
	started, finished := make(chan struct{}), make(chan error, 1)
	go func() {
		close(started)
		finished <- runtimeDB.Transaction(func(tx *gorm.DB) error { return lockActivePath(tx, pathID, member) })
	}()
	<-started
	select {
	case err := <-finished:
		removal.Rollback()
		t.Fatalf("mutation passed removal lock early: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := removal.Table("path_membership_models").Where("path_id = ? AND user_id = ?", pathID, member).Delete(&struct{}{}).Error; err != nil {
		removal.Rollback()
		t.Fatal(err)
	}
	if err := removal.Commit().Error; err != nil {
		t.Fatal(err)
	}
	if err := <-finished; !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("post-removal mutation guard=%v want not found", err)
	}

	rejoinedAt := now.Add(time.Hour)
	if err := migrationDB.Table("path_membership_models").Create(map[string]any{"path_id": pathID, "user_id": member, "role": "participant", "joined_at": rejoinedAt}).Error; err != nil {
		t.Fatal(err)
	}
	if err := runtimeDB.Transaction(func(tx *gorm.DB) error { return lockActivePathAt(tx, pathID, member, now) }); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("pre-reinvite queued occurrence=%v want not found", err)
	}
	if err := runtimeDB.Transaction(func(tx *gorm.DB) error { return lockActivePathAt(tx, pathID, member, rejoinedAt) }); err != nil {
		t.Fatalf("current-incarnation occurrence=%v", err)
	}
}
