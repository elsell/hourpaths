package gormstore

import (
	"context"
	"strings"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
)

func TestAvailableUsernameSuggestionSkipsCaseInsensitiveReservations(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	tx := store.DB.WithContext(context.Background()).Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	txStore := &Store{DB: tx}

	for index, username := range []string{"Identity.Seed", "identity.seed.2"} {
		row := userModel{ID: newTestID(), Username: &username, Status: identity.StatusActive}
		if err := tx.Create(&row).Error; err != nil {
			t.Fatalf("seed username %d: %v", index, err)
		}
	}

	suggestion, err := txStore.SuggestAvailableUsername(context.Background(), "Identity Seed")
	if err != nil {
		t.Fatal(err)
	}
	if suggestion != "identity.seed.3" {
		t.Fatalf("suggestion = %q, want identity.seed.3", suggestion)
	}
}

func TestAvailableUsernameSuggestionContinuesPastTenThousandReservations(t *testing.T) {
	reservedCount := 0
	lookupCalls := 0
	suggestion, err := suggestAvailableUsername(context.Background(), "popular", func(_ context.Context, candidates []string) (map[string]struct{}, error) {
		lookupCalls++
		reserved := make(map[string]struct{}, len(candidates))
		for _, candidate := range candidates {
			reservedCount++
			if reservedCount <= 10000 {
				reserved[candidate] = struct{}{}
			}
		}
		return reserved, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if suggestion != "popular.10001" {
		t.Fatalf("suggestion = %q, want popular.10001", suggestion)
	}
	if lookupCalls <= 1 {
		t.Fatalf("lookup calls = %d, want batched continuation", lookupCalls)
	}
}

func TestAvailableUsernameSuggestionPreservesTheFormatAtMaximumLength(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	tx := store.DB.WithContext(context.Background()).Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	txStore := &Store{DB: tx}

	base := strings.Repeat("a", 64)
	if err := tx.Create(&userModel{ID: newTestID(), Username: &base, Status: identity.StatusActive}).Error; err != nil {
		t.Fatal(err)
	}
	suggestion, err := txStore.SuggestAvailableUsername(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if suggestion != strings.Repeat("a", 62)+".2" {
		t.Fatalf("suggestion = %q", suggestion)
	}
	if err := identity.ValidateUsername(suggestion); err != nil {
		t.Fatalf("suggestion is invalid: %v", err)
	}
}

func TestAvailableUsernameSuggestionAllowsManualEntryWhenTheProviderNameHasNoCandidate(t *testing.T) {
	store := &Store{}
	suggestion, err := store.SuggestAvailableUsername(context.Background(), "李雷")
	if err != nil || suggestion != "" {
		t.Fatalf("suggestion=%q err=%v, want empty without a database query", suggestion, err)
	}
}
