package httpserver

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type deletionRouteState struct {
	commands map[string]app.AccountDeletionCommand
}

func (s *deletionRouteState) Authenticate(_ context.Context, credential string) (ports.Principal, error) {
	owner := strings.TrimPrefix(credential, "Bearer ")
	if credential != "Bearer owner" && credential != "Bearer other" {
		return ports.Principal{}, app.ErrUnauthenticated
	}
	if _, deleted := s.commands[owner]; deleted {
		return ports.Principal{}, app.ErrUnauthenticated
	}
	return ports.Principal{UserID: owner, Scopes: []string{"api:user"}}, nil
}
func (s *deletionRouteState) DeleteAccount(_ context.Context, c app.AccountDeletionCommand) error {
	s.commands[c.UserID] = c
	return nil
}
func (s *deletionRouteState) DeletionReceipt(_ context.Context, id string, hash []byte, now time.Time) (bool, error) {
	c, ok := s.commands[id]
	return ok && bytes.Equal(c.ReceiptHash, hash) && now.Before(c.DeletedAt.Add(30*24*time.Hour)), nil
}

type deletionRouteUsers struct{ timeZoneRouteUsers }

func (deletionRouteUsers) GetUser(_ context.Context, id string) (identity.User, error) {
	return identity.User{ID: id, Status: identity.StatusActive}, nil
}

func TestAccountDeletionHTTPSeparatesRevokedSessionsFromDeletionReceipts(t *testing.T) {
	state := &deletionRouteState{commands: map[string]app.AccountDeletionCommand{}}
	application := app.App{Auth: state, Users: deletionRouteUsers{}, AccountDeletion: state, DeletionJournal: state, Clock: docsClock{now: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)}, Audits: timeZoneRouteAudits{}, AuditRateLimiter: docsLimiter{}}
	handler, _ := New(application, nil, Options{})
	secret := strings.Repeat("a", 64)
	call := func(path, authorization, body string, want int) {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", authorization)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("%s status=%d want=%d body=%s", path, response.Code, want, response.Body.String())
		}
	}
	body := fmt.Sprintf(`{"confirmed":true,"reviewedUserId":"owner","receiptSecret":%q}`, secret)
	for _, token := range []string{"", "Bearer expired", "malformed"} {
		call("/v1/me/deletion", token, body, http.StatusUnauthorized)
	}
	call("/v1/me/deletion", "Bearer other", body, http.StatusConflict)
	call("/v1/me/deletion", "Bearer owner", strings.Replace(body, `"confirmed":true`, `"confirmed":false`, 1), http.StatusBadRequest)
	if len(state.commands) != 0 {
		t.Fatal("rejected deletion mutated state")
	}
	call("/v1/me/deletion", "Bearer owner", body, http.StatusNoContent)
	call("/v1/me/deletion", "Bearer owner", body, http.StatusUnauthorized)
	receipt := fmt.Sprintf(`{"userId":"owner","receiptSecret":%q}`, secret)
	call("/v1/account-deletion/receipt", "", receipt, http.StatusNoContent)
	call("/v1/account-deletion/receipt", "", strings.Replace(receipt, "owner", "other", 1), http.StatusUnauthorized)
	call("/v1/account-deletion/receipt", "", strings.Replace(receipt, secret, strings.Repeat("b", 64), 1), http.StatusUnauthorized)
	request := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	request.Header.Set("Authorization", "Bearer owner")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatal("deletion receipt restored ordinary session access")
	}
}

func (s *deletionRouteState) Admit(_ context.Context, record app.DeletionRecord) (app.DeletionRecord, error) {
	return record, nil
}

func (r *deletionRouteState) ConfirmDeletionReceipt(ctx context.Context, id string, hash []byte, now time.Time, event audit.Event) (bool, error) {
	return r.DeletionReceipt(ctx, id, hash, now)
}
