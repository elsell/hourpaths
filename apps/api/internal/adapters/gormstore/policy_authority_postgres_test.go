package gormstore

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestPolicyAuthorityFailsClosedUntilPublishedAndEnforcesRevisionOrdering(t *testing.T) {
	if *migrationPostgresTestDSN == "" {
		t.Skip("migration-capable PostgreSQL integration test DSN not configured")
	}
	store, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.DB.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&currentPolicySetModel{}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = store.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&currentPolicySetModel{}).Error
	})

	if _, err := store.Current(ctx); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("empty authority error = %v, want not found", err)
	}
	first := policySet(1, "v1")
	if got, err := store.Publish(ctx, first); err != nil || !ports.PolicySetsEqual(got, first) {
		t.Fatalf("first publication = %#v, %v", got, err)
	}
	if got, err := store.Publish(ctx, first); err != nil || !ports.PolicySetsEqual(got, first) {
		t.Fatalf("identical retry = %#v, %v", got, err)
	}
	retriedLater := first
	retriedLater.UpdatedAt = retriedLater.UpdatedAt.Add(time.Hour)
	if got, err := store.Publish(ctx, retriedLater); err != nil || !ports.PolicySetsEqual(got, first) {
		t.Fatalf("same revision retry must preserve original publication time = %#v, %v", got, err)
	}
	changedSameRevision := first
	changedSameRevision.TermsVersion = "different"
	if got, err := store.Publish(ctx, changedSameRevision); !errors.Is(err, ports.ErrConflict) || !ports.PolicySetsEqual(got, first) {
		t.Fatalf("same revision conflict = %#v, %v", got, err)
	}
	higher := policySet(3, "v3")
	if got, err := store.Publish(ctx, higher); err != nil || !ports.PolicySetsEqual(got, higher) {
		t.Fatalf("higher publication = %#v, %v", got, err)
	}
	actualLower := policySet(2, "v2")
	if got, err := store.Publish(ctx, actualLower); err != nil || !ports.PolicySetsEqual(got, higher) {
		t.Fatalf("lower publication should return shared authority = %#v, %v", got, err)
	}
	if got, err := store.Current(ctx); err != nil || !ports.PolicySetsEqual(got, higher) {
		t.Fatalf("current authority = %#v, %v", got, err)
	}
}

func TestConcurrentPolicyPublicationsConvergeOnHighestRevision(t *testing.T) {
	if *migrationPostgresTestDSN == "" {
		t.Skip("migration-capable PostgreSQL integration test DSN not configured")
	}
	store, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.DB.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&currentPolicySetModel{}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = store.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&currentPolicySetModel{}).Error
	})

	var wg sync.WaitGroup
	errorsByRevision := make(chan error, 8)
	for revision := int64(1); revision <= 8; revision++ {
		wg.Add(1)
		go func(revision int64) {
			defer wg.Done()
			_, err := store.Publish(ctx, policySet(revision, "concurrent"))
			errorsByRevision <- err
		}(revision)
	}
	wg.Wait()
	close(errorsByRevision)
	for err := range errorsByRevision {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.Current(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != 8 {
		t.Fatalf("current revision = %d, want 8", got.Revision)
	}
}

func TestRuntimePolicyAuthorityCanReadButCannotPublish(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("runtime and migration-capable PostgreSQL integration test DSNs are required")
	}
	ctx := context.Background()
	migrationStore, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	current := policySet(41, "least-privilege")
	if err := migrationStore.DB.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&currentPolicySetModel{}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = migrationStore.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&currentPolicySetModel{}).Error
	})
	if _, err := migrationStore.Publish(ctx, current); err != nil {
		t.Fatal(err)
	}

	runtimeStore, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := runtimeStore.Current(ctx); err != nil || !ports.PolicySetsEqual(got, current) {
		t.Fatalf("runtime current policy = %#v, %v", got, err)
	}
	higher := policySet(42, "forbidden-runtime-publication")
	if _, err := runtimeStore.Publish(ctx, higher); err == nil {
		t.Fatal("runtime role published a current policy set")
	}
	if got, err := migrationStore.Current(ctx); err != nil || !ports.PolicySetsEqual(got, current) {
		t.Fatalf("runtime publication attempt changed authority = %#v, %v", got, err)
	}
}

func policySet(revision int64, suffix string) ports.PolicySet {
	return ports.PolicySet{
		Revision:     revision,
		TermsVersion: "terms-" + suffix, PrivacyPolicyVersion: "privacy-" + suffix, CommunityGuidelinesVersion: "guidelines-" + suffix,
		TermsURL: "https://app.example/legal/terms", PrivacyPolicyURL: "https://app.example/legal/privacy",
		CommunityGuidelinesURL: "https://app.example/community-guidelines", SupportURL: "https://app.example/support",
		UpdatedAt: time.Date(2026, 7, 21, 12, int(revision), 0, 0, time.UTC),
	}
}
