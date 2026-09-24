package sessionauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

type fakeRepository struct {
	record           ports.SessionRecord
	principal        ports.Principal
	revoked          []byte
	err              error
	event            audit.Event
	rotationExpiry   time.Time
	activation       identity.OnboardingActivation
	activationNow    time.Time
	activationEvents []audit.Event
}

func (f *fakeRepository) SaveSession(_ context.Context, record ports.SessionRecord, event audit.Event) error {
	f.record = record
	f.event = event
	return nil
}
func (f *fakeRepository) ResolveSession(_ context.Context, hash []byte, _ time.Time) (ports.Principal, error) {
	if f.err != nil {
		return ports.Principal{}, f.err
	}
	if string(hash) != string(f.record.TokenHash) {
		return ports.Principal{}, ports.ErrNotFound
	}
	return f.principal, nil
}
func (f *fakeRepository) RevokeSessionHash(_ context.Context, hash []byte, _ time.Time, event audit.Event) error {
	if f.err != nil {
		return f.err
	}
	f.revoked = append([]byte(nil), hash...)
	f.event = event
	return nil
}
func (f *fakeRepository) RotateSessionHash(_ context.Context, oldHash []byte, _ time.Time, record ports.SessionRecord, _, created audit.Event) (time.Time, error) {
	if f.err != nil {
		return time.Time{}, f.err
	}
	f.revoked = append([]byte(nil), oldHash...)
	f.record = record
	f.event = created
	if !f.rotationExpiry.IsZero() {
		return f.rotationExpiry, nil
	}
	return record.ExpiresAt, nil
}

func (f *fakeRepository) ActivateOnboarding(_ context.Context, activation identity.OnboardingActivation, oldHash []byte, now time.Time, record ports.SessionRecord, completed, revoked, created audit.Event) (time.Time, error) {
	if f.err != nil {
		return time.Time{}, f.err
	}
	f.activation = activation
	f.activationNow = now
	f.activationEvents = []audit.Event{completed, revoked, created}
	f.revoked = append([]byte(nil), oldHash...)
	f.record = record
	if !f.rotationExpiry.IsZero() {
		return f.rotationExpiry, nil
	}
	return record.ExpiresAt, nil
}

func TestActivationRotatesTheExactOpaqueOnboardingCredentialThroughOneRepositoryCall(t *testing.T) {
	now := time.Date(2026, 7, 21, 18, 0, 0, 0, time.UTC)
	requestedExpiry := now.Add(time.Hour)
	actualExpiry := now.Add(40 * time.Minute)
	repository := &fakeRepository{rotationExpiry: actualExpiry}
	manager := New(repository, fakeClock{now: now})
	oldToken := base64.RawURLEncoding.EncodeToString(make([]byte, tokenBytes))
	activation := identity.OnboardingActivation{UserID: "provisional-owner", Username: "practice.time"}
	events := []audit.Event{{ID: "completed"}, {ID: "revoked"}, {ID: "created"}}

	newToken, expiresAt, err := manager.ActivateOnboarding(context.Background(), "Bearer "+oldToken, activation, requestedExpiry, events[0], events[1], events[2])
	if err != nil {
		t.Fatalf("activate onboarding: %v", err)
	}
	if newToken == "" || newToken == oldToken || !expiresAt.Equal(actualExpiry) {
		t.Fatalf("activation credential = token %q expiry %v", newToken, expiresAt)
	}
	oldHash := sha256.Sum256([]byte(oldToken))
	newHash := sha256.Sum256([]byte(newToken))
	if string(repository.revoked) != string(oldHash[:]) || string(repository.record.TokenHash) != string(newHash[:]) {
		t.Fatal("activation did not hash the old and new opaque credentials")
	}
	if repository.record.UserID != activation.UserID || len(repository.record.Scopes) != 1 || repository.record.Scopes[0] != "api:user" || !repository.record.ExpiresAt.Equal(requestedExpiry) {
		t.Fatalf("new session record = %+v", repository.record)
	}
	if repository.activation != activation || !repository.activationNow.Equal(now) || len(repository.activationEvents) != 3 {
		t.Fatalf("atomic activation call = activation %+v now %v events %+v", repository.activation, repository.activationNow, repository.activationEvents)
	}
}

func TestActivationRejectsMalformedCredentialsAndPreservesRepositoryFailures(t *testing.T) {
	now := time.Date(2026, 7, 21, 18, 0, 0, 0, time.UTC)
	activation := identity.OnboardingActivation{UserID: "provisional-owner"}
	manager := New(&fakeRepository{}, fakeClock{now: now})
	if _, _, err := manager.ActivateOnboarding(context.Background(), "Bearer short", activation, now.Add(time.Hour), audit.Event{}, audit.Event{}, audit.Event{}); !errors.Is(err, ports.ErrInvalidCredential) {
		t.Fatalf("malformed credential error = %v", err)
	}
	validToken := base64.RawURLEncoding.EncodeToString(make([]byte, tokenBytes))
	for _, dependencyErr := range []error{ports.ErrNotFound, ports.ErrConflict, errors.New("database unavailable")} {
		manager = New(&fakeRepository{err: dependencyErr}, fakeClock{now: now})
		_, _, err := manager.ActivateOnboarding(context.Background(), "Bearer "+validToken, activation, now.Add(time.Hour), audit.Event{}, audit.Event{}, audit.Event{})
		want := dependencyErr
		if errors.Is(dependencyErr, ports.ErrNotFound) {
			want = ports.ErrInvalidCredential
		}
		if !errors.Is(err, want) {
			t.Fatalf("activation error = %v, want %v", err, want)
		}
	}
}

func TestSessionRepositoryFailuresAreNotReportedAsBadCredentials(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	manager := New(&fakeRepository{err: databaseErr}, fakeClock{})
	token := base64.RawURLEncoding.EncodeToString(make([]byte, tokenBytes))
	if _, err := manager.Authenticate(context.Background(), "Bearer "+token); !errors.Is(err, databaseErr) || errors.Is(err, ports.ErrInvalidCredential) {
		t.Fatalf("authentication persistence failure was misclassified: %v", err)
	}
	event := audit.Event{ID: "event", OwnerUserID: "user", ActorUserID: "user", Action: audit.SessionRevoked, TargetType: "user", TargetID: "user", Outcome: audit.Succeeded, CorrelationID: "request", OccurredAt: time.Now()}
	if err := manager.RevokeSession(context.Background(), "Bearer "+token, event); !errors.Is(err, databaseErr) || errors.Is(err, ports.ErrInvalidCredential) {
		t.Fatalf("revocation persistence failure was misclassified: %v", err)
	}
}

func TestApplicationSessionsAreOpaqueHashedScopedAndRevocable(t *testing.T) {
	now := time.Now().UTC()
	repository := &fakeRepository{principal: ports.Principal{UserID: "user", Scopes: []string{"api:user"}}}
	manager := New(repository, fakeClock{now})
	createdEvent := audit.Event{ID: "created", OwnerUserID: "user", ActorUserID: "user", Action: audit.SessionCreated, TargetType: "user", TargetID: "user", Outcome: audit.Succeeded, CorrelationID: "request", OccurredAt: now}
	token, err := manager.CreateSession(context.Background(), "user", []string{"api:user"}, make([]byte, 32), now.Add(time.Hour), now.Add(12*time.Hour), createdEvent)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || string(repository.record.TokenHash) == token {
		t.Fatal("session was not opaque and hashed")
	}
	principal, err := manager.Authenticate(context.Background(), "Bearer "+token)
	if err != nil || principal.UserID != "user" {
		t.Fatalf("session rejected: %+v %v", principal, err)
	}
	revokedEvent := createdEvent
	revokedEvent.ID, revokedEvent.Action = "revoked", audit.SessionRevoked
	if err := manager.RevokeSession(context.Background(), "Bearer "+token, revokedEvent); err != nil {
		t.Fatal(err)
	}
	if string(repository.revoked) != string(repository.record.TokenHash) {
		t.Fatal("wrong session revoked")
	}
}

func TestApplicationSessionsRejectOIDCAndMalformedBearerValues(t *testing.T) {
	manager := New(&fakeRepository{}, fakeClock{})
	for _, header := range []string{"", "Basic token", "Bearer signed.jwt.value", "Bearer short", "bearer token"} {
		if _, err := manager.Authenticate(context.Background(), header); err == nil {
			t.Errorf("accepted %q", header)
		}
	}
	if _, err := manager.CreateSession(context.Background(), "user", []string{"admin"}, make([]byte, 32), time.Now().Add(time.Hour), time.Now().Add(12*time.Hour), audit.Event{}); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("accepted escalated scope: %v", err)
	}
}

func TestCreateSessionSupportsOnlyOneAccountLifecycleScope(t *testing.T) {
	now := time.Now().UTC()
	createdEvent := audit.Event{ID: "created", OwnerUserID: "user", ActorUserID: "user", Action: audit.SessionCreated, TargetType: "user", TargetID: "user", Outcome: audit.Succeeded, CorrelationID: "request", OccurredAt: now}
	for _, scope := range []string{"api:user", "api:onboarding"} {
		t.Run(scope, func(t *testing.T) {
			repository := &fakeRepository{}
			manager := New(repository, fakeClock{now: now})
			if _, err := manager.CreateSession(context.Background(), "user", []string{scope}, make([]byte, 32), now.Add(time.Hour), now.Add(12*time.Hour), createdEvent); err != nil {
				t.Fatalf("CreateSession(%q) returned %v", scope, err)
			}
			if len(repository.record.Scopes) != 1 || repository.record.Scopes[0] != scope {
				t.Fatalf("persisted scopes = %v, want only %q", repository.record.Scopes, scope)
			}
		})
	}

	manager := New(&fakeRepository{}, fakeClock{now: now})
	for name, scopes := range map[string][]string{
		"arbitrary": {"admin"},
		"multiple":  {"api:user", "api:onboarding"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := manager.CreateSession(context.Background(), "user", scopes, make([]byte, 32), now.Add(time.Hour), now.Add(12*time.Hour), createdEvent); !errors.Is(err, ports.ErrInvalidArgument) {
				t.Fatalf("CreateSession(%v) returned %v, want ErrInvalidArgument", scopes, err)
			}
		})
	}
}

func TestRotationReturnsRepositoryCappedFamilyExpiry(t *testing.T) {
	now := time.Now().UTC()
	absoluteExpiry := now.Add(30 * time.Minute)
	repository := &fakeRepository{rotationExpiry: absoluteExpiry}
	manager := New(repository, fakeClock{now: now})
	oldToken := base64.RawURLEncoding.EncodeToString(make([]byte, tokenBytes))

	newToken, actualExpiry, err := manager.RotateSession(context.Background(), "Bearer "+oldToken, "user", []string{"api:user"}, now.Add(time.Hour), audit.Event{}, audit.Event{})
	if err != nil {
		t.Fatal(err)
	}
	if newToken == "" || !actualExpiry.Equal(absoluteExpiry) {
		t.Fatalf("rotation did not preserve capped expiry: token=%q expiry=%v", newToken, actualExpiry)
	}
}
