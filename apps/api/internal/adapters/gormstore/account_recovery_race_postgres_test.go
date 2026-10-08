package gormstore

import (
	"context"
	"strings"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func recoveryRaceFixture(t *testing.T) (*Store, *Store, *gorm.DB, string, application.AccountRecoveryCompletion) {
	t.Helper()
	first, second, inspect := activationRaceStores(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	source, target := newTestID(), newTestID()
	email, issuer := newTestID()+"@example.test", "https://recovery-race.example/"+newTestID()
	for _, row := range []userModel{{ID: source, Email: email, ProviderEmailVerified: true, Status: identity.StatusProvisional, CreatedAt: now, UpdatedAt: now}, {ID: target, Email: email, ProviderEmailVerified: true, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}} {
		if err := inspect.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []identityModel{{Issuer: issuer, Subject: "source", UserID: source, Provider: identity.ProviderApple}, {Issuer: issuer, Subject: "target", UserID: target, Provider: identity.ProviderGoogle}} {
		if err := inspect.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	hash, nonce := activationRaceHash("source"), activationRaceHash("nonce")
	if err := inspect.Create(&sessionModel{TokenHash: hash, UserID: source, Scopes: "api:onboarding", CreatedAt: now, ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(24 * time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	event := func(action audit.Action, kind string) audit.Event {
		return audit.Event{ID: newTestID(), OwnerUserID: source, ActorUserID: source, Action: action, TargetType: kind, TargetID: source, Outcome: audit.Succeeded, CorrelationID: source, OccurredAt: now}
	}
	challenge := newTestID()
	if _, err := first.AdmitAccountRecovery(context.Background(), application.AccountRecoveryAdmission{UserID: source, ID: challenge, SessionHash: hash, NonceHash: nonce, CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute), Audit: event(audit.ResourceCreated, "account_recovery_challenge")}); err != nil {
		t.Fatal(err)
	}
	return first, second, inspect, target, application.AccountRecoveryCompletion{UserID: source, ChallengeID: challenge, Issuer: issuer, Subject: "target", Provider: identity.ProviderGoogle, NonceHash: nonce, SessionHash: hash, NewSessionHash: activationRaceHash("replacement"), Now: now, ExpiresAt: now.Add(time.Hour), Completed: event(audit.ResourceDeleted, "account_enrollment"), Revoked: event(audit.SessionRevoked, "user"), Linked: event(audit.ResourceCreated, "linked_identity"), Created: event(audit.SessionCreated, "user")}
}
func raceActivation(ctx context.Context, store *Store, c application.AccountRecoveryCompletion) error {
	completed, revoked, created := activationAudits(c.UserID, c.Now)
	_, err := store.ActivateOnboarding(ctx, validActivation(c.UserID, "recovery_"+strings.ReplaceAll(c.UserID, "-", ""), c.Now), c.SessionHash, c.Now,
		ports.SessionRecord{TokenHash: activationRaceHash("activation"), UserID: c.UserID, Scopes: []string{"api:user"}, ExpiresAt: c.Now.Add(time.Hour)}, completed, revoked, created)
	return err
}
func TestAccountRecoverySerializesWithActivation(t *testing.T) {
	for _, recoveryFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "activation wins", true: "recovery wins"}[recoveryFirst], func(t *testing.T) {
			first, second, inspect, target, c := recoveryRaceFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			tx := first.DB.WithContext(ctx).Begin()
			defer tx.Rollback()
			held := &Store{DB: tx}
			allow := func(context.Context, string) error { return nil }
			if recoveryFirst {
				if _, err := held.CompleteAccountRecovery(ctx, c, allow); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := raceActivation(ctx, held, c); err != nil {
					t.Fatal(err)
				}
			}
			result := make(chan error, 1)
			go func() {
				if recoveryFirst {
					result <- raceActivation(ctx, second, c)
				} else {
					_, err := second.CompleteAccountRecovery(ctx, c, allow)
					result <- err
				}
			}()
			if err := tx.Commit().Error; err != nil {
				t.Fatal(err)
			}
			if err := <-result; err == nil {
				t.Fatal("both incompatible enrollment transitions committed")
			}
			var rows []identityModel
			if err := inspect.Where("issuer = ?", c.Issuer).Find(&rows).Error; err != nil || len(rows) != 2 {
				t.Fatal("identity lost", err)
			}
			for _, row := range rows {
				if row.Subject == "source" && row.UserID != map[bool]string{true: target, false: c.UserID}[recoveryFirst] {
					t.Fatal("wrong identity owner after race")
				}
			}
			var count int64
			if err := inspect.Model(&userModel{}).Where("id = ? AND status = ?", c.UserID, identity.StatusActive).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if (count == 1) == recoveryFirst {
				t.Fatal("completed account was merged or successful recovery retained enrollment")
			}
		})
	}
}
func TestAccountRecoveryCannotResurrectAConcurrentlyDeletedDestination(t *testing.T) {
	for _, recoveryFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "deletion wins", true: "recovery wins"}[recoveryFirst], func(t *testing.T) {
			first, second, inspect, target, c := recoveryRaceFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			deletion := application.AccountDeletionCommand{UserID: target, DeletedAt: c.Now, ReceiptHash: activationRaceHash("receipt"), NewID: newTestID,
				Audit: audit.Event{ID: newTestID(), OwnerUserID: target, ActorUserID: target, Action: audit.ResourceDeleted, TargetType: "account", TargetID: target, Outcome: audit.Succeeded, CorrelationID: target, OccurredAt: c.Now}}
			tx := first.DB.WithContext(ctx).Begin()
			defer tx.Rollback()
			held := &Store{DB: tx}
			allow := func(context.Context, string) error { return nil }
			if recoveryFirst {
				if _, err := held.CompleteAccountRecovery(ctx, c, allow); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := held.DeleteAccount(ctx, deletion); err != nil {
					t.Fatal(err)
				}
			}
			result := make(chan error, 1)
			go func() {
				if recoveryFirst {
					result <- second.DeleteAccount(ctx, deletion)
				} else {
					_, err := second.CompleteAccountRecovery(ctx, c, allow)
					result <- err
				}
			}()
			if err := tx.Commit().Error; err != nil {
				t.Fatal(err)
			}
			err := <-result
			if recoveryFirst && err != nil {
				t.Fatal(err)
			}
			if !recoveryFirst && err == nil {
				t.Fatal("recovered deleted destination")
			}
			for _, table := range []string{"user_models", "identity_models", "session_models"} {
				var count int64
				column := "user_id"
				if table == "user_models" {
					column = "id"
				}
				if err := inspect.Table(table).Where(column+" = ?", target).Count(&count).Error; err != nil || count != 0 {
					t.Fatal("deleted account state resurrected", table, err)
				}
			}
			var sourceCount int64
			if err := inspect.Model(&identityModel{}).Where("user_id = ?", c.UserID).Count(&sourceCount).Error; err != nil {
				t.Fatal(err)
			}
			if (sourceCount == 1) == recoveryFirst {
				t.Fatal("deletion race mishandled original enrollment")
			}
		})
	}
}
