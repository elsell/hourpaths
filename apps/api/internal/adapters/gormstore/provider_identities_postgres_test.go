package gormstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestProviderLinkOwnershipReplayAndConcurrentFinalUnlink(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("requires PostgreSQL runtime and migration DSNs")
	}
	ctx := context.Background()
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner := userModel{ID: newTestID(), Email: "", DisplayName: "Identity fixture", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}
	other := owner
	other.ID = newTestID()
	for _, u := range []userModel{owner, other} {
		if err := admin.DB.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
	}
	issuer := "https://identity-link.example/" + newTestID()
	for _, v := range []identityModel{{Issuer: issuer, Subject: "owner-google", UserID: owner.ID, Provider: identity.ProviderGoogle}, {Issuer: issuer, Subject: "other-apple", UserID: other.ID, Provider: identity.ProviderApple}} {
		if err := admin.DB.Create(&v).Error; err != nil {
			t.Fatal(err)
		}
	}
	event := func(user string, action audit.Action, targetType, targetID string) audit.Event {
		return audit.Event{ID: newTestID(), OwnerUserID: user, ActorUserID: user, Action: action, TargetType: targetType, TargetID: targetID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}
	}
	hash := sha256.Sum256([]byte("controlled nonce"))
	admission := application.IdentityLinkAdmission{UserID: owner.ID, ID: newTestID(), Provider: identity.ProviderApple, NonceHash: hash[:], CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute), Audit: event(owner.ID, audit.ResourceCreated, "identity_link_challenge", owner.ID)}
	if err := store.AdmitIdentityLink(ctx, admission); err != nil {
		t.Fatal(err)
	}
	complete := application.IdentityLinkCompletion{UserID: owner.ID, ChallengeID: admission.ID, Issuer: issuer, Subject: "new-apple", Provider: identity.ProviderApple, NonceHash: hash[:], Now: now, Audit: event(owner.ID, audit.ResourceCreated, "linked_identity", "apple")}
	for _, tc := range []struct {
		name   string
		change func(*application.IdentityLinkCompletion)
	}{
		{"different owner", func(c *application.IdentityLinkCompletion) {
			c.UserID = other.ID
			c.Audit = event(other.ID, audit.ResourceCreated, "linked_identity", "apple")
		}},
		{"expired", func(c *application.IdentityLinkCompletion) { c.Now = admission.ExpiresAt; c.Audit.OccurredAt = c.Now }},
		{"wrong nonce", func(c *application.IdentityLinkCompletion) {
			otherHash := sha256.Sum256([]byte("another nonce"))
			c.NonceHash = otherHash[:]
		}},
		{"wrong provider", func(c *application.IdentityLinkCompletion) {
			c.Provider = identity.ProviderGoogle
			c.Audit.TargetID = "google"
		}},
		{"identity belongs to another user", func(c *application.IdentityLinkCompletion) { c.Subject = "other-apple" }},
		{"invalid audit", func(c *application.IdentityLinkCompletion) { c.Audit.ActorUserID = other.ID }},
		{"audit insert failure rolls back link and challenge consumption", func(c *application.IdentityLinkCompletion) { c.Audit.ID = admission.Audit.ID }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := complete
			tc.change(&c)
			if err := store.CompleteIdentityLink(ctx, c); err == nil {
				t.Fatal("unproven link accepted")
			}
		})
	}
	if err := store.CompleteIdentityLink(ctx, complete); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteIdentityLink(ctx, complete); err == nil {
		t.Fatal("challenge replay accepted")
	}
	providers, err := store.ListProviderIdentities(ctx, owner.ID)
	if err != nil || len(providers) != 2 {
		t.Fatalf("providers=%v err=%v", providers, err)
	}
	var eventCount int64
	if err := admin.DB.Model(&auditEventModel{}).Where("id = ?", complete.Audit.ID).Count(&eventCount).Error; err != nil || eventCount != 1 {
		t.Fatalf("atomic link audit missing: %d %v", eventCount, err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, p := range providers {
		wg.Add(1)
		go func(provider identity.Provider) {
			defer wg.Done()
			results <- store.UnlinkProviderIdentity(ctx, application.IdentityUnlink{UserID: owner.ID, Provider: provider, Audit: event(owner.ID, audit.ResourceDeleted, "linked_identity", string(provider))})
		}(p)
	}
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ports.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent unlink: successes=%d conflicts=%d", successes, conflicts)
	}
	providers, err = store.ListProviderIdentities(ctx, owner.ID)
	if err != nil || len(providers) != 1 {
		t.Fatalf("final sign-in identity lost: %v %v", providers, err)
	}
	others, err := store.ListProviderIdentities(ctx, other.ID)
	if err != nil || len(others) != 1 || others[0] != identity.ProviderApple {
		t.Fatalf("other account changed: %v %v", others, err)
	}
}
