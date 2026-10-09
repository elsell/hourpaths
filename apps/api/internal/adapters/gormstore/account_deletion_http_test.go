package gormstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/deletionjournal"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/ratelimit"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/sessionauth"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPostgresAccountDeletionHTTPWithPersistedSessions(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("isolated database roles required")
	}
	ctx := context.Background()
	store, err := openAccountDeletionStore(t, *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := openAccountDeletionStore(t, *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner, other := newTestID(), newTestID()
	for _, id := range []string{owner, other} {
		if err = seed.DB.Create(&userModel{ID: id, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	directory := filepath.Join(t.TempDir(), "journal")
	journal, err := deletionjournal.New(directory, bytes.Repeat([]byte{8}, 32))
	if err != nil {
		t.Fatal(err)
	}
	manager := sessionauth.NewWithAccountAccess(store, deletionTestClock{now}, journal.CheckAccountAccess)
	issue := func(id string, at time.Time) string {
		t.Helper()
		issuer := sessionauth.NewWithAccountAccess(store, deletionTestClock{at}, journal.CheckAccountAccess)
		event := audit.Event{ID: newTestID(), OwnerUserID: id, ActorUserID: id, Action: audit.SessionCreated, TargetType: "user", TargetID: id, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}
		identityHash := sha256.Sum256([]byte(newTestID()))
		token, err := issuer.CreateSession(ctx, id, []string{"api:user"}, identityHash[:], at.Add(time.Hour), at.Add(24*time.Hour), event)
		if err != nil {
			t.Fatal(err)
		}
		return "Bearer " + token
	}
	first, second, foreign, expired := issue(owner, now), issue(owner, now), issue(other, now), issue(owner, now.Add(-2*time.Hour))
	limiter, err := ratelimit.New(store.DB, "audit", 1000, time.Minute, 1000)
	if err != nil {
		t.Fatal(err)
	}
	app := application.App{Auth: manager, Users: store, AccountDeletion: store, DeletionJournal: journal, Clock: deletionTestClock{now}, Audits: store, AuditRateLimiter: limiter}
	handler, _ := newPolicyAcceptedHTTPTestServer(app, nil, httpserver.Options{})
	server := httptest.NewServer(handler)
	defer server.Close()
	call := func(method, path, token, body string, want int) {
		t.Helper()
		request, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.Header.Set("Authorization", token)
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		output, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		if response.StatusCode != want {
			t.Fatalf("%s %s status %d want %d: %s", method, path, response.StatusCode, want, output)
		}
	}
	secret := strings.Repeat("a", 64)
	body := fmt.Sprintf(`{"confirmed":true,"reviewedUserId":%q,"receiptSecret":%q}`, owner, secret)
	for _, token := range []string{"", "Bearer malformed", expired, "Bearer wrong.issuer.audience"} {
		call("POST", "/v1/me/deletion", token, body, 401)
	}
	call("POST", "/v1/me/deletion", foreign, body, 409)
	call("POST", "/v1/me/deletion", first, strings.Replace(body, `"confirmed":true`, `"confirmed":false`, 1), 400)
	var remaining int64
	if err = seed.DB.Model(&userModel{}).Where("id IN ?", []string{owner, other}).Count(&remaining).Error; err != nil || remaining != 2 {
		t.Fatal("rejected deletion removed an account", err)
	}
	records, err := journal.Records(ctx)
	if err != nil || len(records) != 0 {
		t.Fatal("rejected deletion was admitted", err)
	}
	call("POST", "/v1/me/deletion", first, body, 204)
	for _, token := range []string{first, second} {
		call("GET", "/v1/me", token, "", 401)
	}
	call("GET", "/v1/me", foreign, "", 200)
	receipt := fmt.Sprintf(`{"userId":%q,"receiptSecret":%q}`, owner, secret)
	call("POST", "/v1/account-deletion/receipt", "", receipt, 204)
	call("POST", "/v1/account-deletion/receipt", "", strings.Replace(receipt, owner, other, 1), 401)
	call("POST", "/v1/account-deletion/receipt", "", strings.Replace(receipt, secret, strings.Repeat("b", 64), 1), 401)
	// A lost independent volume must not silently enable ordinary sessions.
	if err = os.Rename(directory, directory+"-offline"); err != nil {
		t.Fatal(err)
	}
	call("GET", "/v1/me", foreign, "", 503)
	if err = os.Rename(directory+"-offline", directory); err != nil {
		t.Fatal(err)
	}
	call("GET", "/v1/me", foreign, "", 200)
}
