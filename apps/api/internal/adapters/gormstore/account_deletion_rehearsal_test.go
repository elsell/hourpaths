package gormstore

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/deletionjournal"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/config"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"os"
	"testing"
	"time"
)

var rehearsalMode = flag.String("deletion-rehearsal", "", "isolated backup fixture action: seed, delete, restored, verified")
var rehearsalFile = flag.String("deletion-rehearsal-file", "", "private isolated fixture identifiers")

type deletionFixture struct{ Owner, Survivor, Path string }

// External pg_dump/pg_restore runs between these phases; this is intentionally
// not a transaction rollback simulation of a backup.
func TestPostgresDeletionBackupRehearsal(t *testing.T) {
	if *rehearsalMode == "" {
		t.Skip("explicit isolated backup rehearsal required")
	}
	if *migrationPostgresTestDSN == "" || *rehearsalFile == "" {
		t.Fatal("rehearsal configuration missing")
	}
	store, err := openAccountDeletionStore(t, *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	var fixture deletionFixture
	now := time.Now().UTC().Truncate(time.Microsecond)
	if *rehearsalMode == "seed" {
		fixture = deletionFixture{newTestID(), newTestID(), newTestID()}
		for _, id := range []string{fixture.Owner, fixture.Survivor} {
			if err := store.DB.Create(&userModel{ID: id, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
				t.Fatal(err)
			}
		}
		if err := store.DB.Table("path_models").Create(map[string]any{"id": fixture.Path, "owner_user_id": fixture.Survivor, "name": "restore fixture", "visibility": "private", "created_at": now, "updated_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{fixture.Owner, fixture.Survivor} {
			if err := store.DB.Table("path_membership_models").Create(map[string]any{"path_id": fixture.Path, "user_id": id, "role": "participant"}).Error; err != nil {
				t.Fatal(err)
			}
			if err := store.DB.Table("recorded_activity_models").Create(map[string]any{"id": newTestID(), "path_id": fixture.Path, "participant_id": id, "started_at": now.Add(-time.Minute), "ended_at": now, "occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now}).Error; err != nil {
				t.Fatal(err)
			}
		}
		data, err := json.Marshal(fixture)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(*rehearsalFile, data, 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(*rehearsalFile)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if *rehearsalMode == "delete" {
		event := audit.Event{ID: newTestID(), OwnerUserID: fixture.Owner, ActorUserID: fixture.Owner, Action: audit.ResourceDeleted, TargetType: "account", TargetID: fixture.Owner, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}
		directory, key, err := config.LoadDeletionJournal()
		if err != nil {
			t.Fatal(err)
		}
		journal, err := deletionjournal.New(directory, key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = journal.Admit(context.Background(), application.DeletionRecord{UserID: fixture.Owner, DeletedAt: now, AuditEventID: event.ID, ReceiptHash: hex.EncodeToString(bytes.Repeat([]byte{7}, 32))}); err != nil {
			t.Fatal(err)
		}
		if err := store.DeleteAccount(context.Background(), application.AccountDeletionCommand{UserID: fixture.Owner, DeletedAt: now, Audit: event, NewID: newTestID, ReceiptHash: bytes.Repeat([]byte{7}, 32)}); err != nil {
			t.Fatal(err)
		}
	} else if *rehearsalMode != "restored" && *rehearsalMode != "verified" {
		t.Fatal("unknown rehearsal phase")
	}
	want := int64(0)
	if *rehearsalMode == "restored" {
		want = 1
	}
	for _, check := range []struct {
		table, where string
		args         []any
		want         int64
	}{
		{"user_models", "id = ?", []any{fixture.Owner}, want},
		{"recorded_activity_models", "participant_id = ?", []any{fixture.Owner}, want},
		{"user_models", "id = ?", []any{fixture.Survivor}, 1},
		{"path_models", "id = ?", []any{fixture.Path}, 1},
		{"recorded_activity_models", "participant_id = ?", []any{fixture.Survivor}, 1},
	} {
		var count int64
		if err := store.DB.Table(check.table).Where(check.where, check.args...).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != check.want {
			t.Fatalf("%s: got %d want %d", check.table, count, check.want)
		}
	}
}
