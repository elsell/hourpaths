package gormstore

import (
	"context"
	"crypto/sha256"
	"errors"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestPostgresPolicyRenewalRetainsEvidenceAndReplaysAcrossPublication(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
	}
	runtime, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner, peer := "policy-owner-"+newTestID(), "policy-peer-"+newTestID()
	for _, id := range []string{owner, peer} {
		seedTimeZonePreferenceUser(t, admin, id, "Etc/UTC", now)
		t.Cleanup(func() { admin.DB.Table("user_models").Where("id = ?", id).Delete(&struct{ ID string }{}) })
	}
	current, err := admin.Current(ctx)
	if errors.Is(err, ports.ErrNotFound) {
		current, err = admin.Publish(ctx, policySet(1, "renewal-initial"))
	}
	if err != nil {
		t.Fatal(err)
	}
	versions := func(p ports.PolicySet) identity.CurrentPolicyVersions {
		return identity.CurrentPolicyVersions{TermsOfService: p.TermsVersion, PrivacyPolicy: p.PrivacyPolicyVersion, CommunityGuidelines: p.CommunityGuidelinesVersion}
	}
	command := func(p ports.PolicySet, key string) application.PolicyRenewalCommand {
		hash := sha256.Sum256([]byte(p.TermsVersion + ":" + key))
		return application.PolicyRenewalCommand{ActorUserID: owner, PolicyRevision: p.Revision, Policies: versions(p), AcceptedAt: now, Idempotency: ports.Idempotency{PrincipalID: owner, Operation: application.RenewPolicyAcceptanceOperation, Key: key, RequestHash: hash[:]}, Audit: audit.Event{ID: newTestID(), OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceUpdated, TargetType: "policy_acceptance", TargetID: owner, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}}
	}
	first := command(current, "policy-renewal-first-01")
	if accepted, err := runtime.HasAcceptedPolicies(ctx, owner, versions(current)); err != nil || accepted {
		t.Fatalf("initial=%v %v", accepted, err)
	}
	saved, err := runtime.RenewPolicyAcceptance(ctx, first)
	if err != nil || saved.Replayed {
		t.Fatalf("save=%+v %v", saved, err)
	}
	if accepted, err := runtime.HasAcceptedPolicies(ctx, peer, versions(current)); err != nil || accepted {
		t.Fatalf("cross-user=%v %v", accepted, err)
	}
	next := current
	next.Revision++
	next.TermsVersion = "terms-" + newTestID()
	next.UpdatedAt = now
	if _, err = admin.Publish(ctx, next); err != nil {
		t.Fatal(err)
	}
	stale := command(current, "policy-renewal-stale-01")
	if _, err = runtime.RenewPolicyAcceptance(ctx, stale); !errors.Is(err, ports.ErrPolicySetChanged) {
		t.Fatalf("stale=%v", err)
	}
	bad := command(next, "policy-renewal-bad-001")
	bad.Audit.ID = first.Audit.ID
	if _, err = runtime.RenewPolicyAcceptance(ctx, bad); err == nil {
		t.Fatal("duplicate audit accepted")
	}
	if accepted, err := runtime.HasAcceptedPolicies(ctx, owner, versions(next)); err != nil || accepted {
		t.Fatalf("audit rollback=%v %v", accepted, err)
	}
	second := command(next, "policy-renewal-next-001")
	if _, err = runtime.RenewPolicyAcceptance(ctx, second); err != nil {
		t.Fatal(err)
	}
	replay, err := runtime.RenewPolicyAcceptance(ctx, first)
	if err != nil || !replay.Replayed || replay.Policies != versions(current) {
		t.Fatalf("replay=%+v %v", replay, err)
	}
	if accepted, err := runtime.HasAcceptedPolicies(ctx, owner, versions(next)); err != nil || !accepted {
		t.Fatalf("current=%v %v", accepted, err)
	}
	var count int64
	if err = admin.DB.Table("user_policy_acceptance_models").Where("user_id = ?", owner).Count(&count).Error; err != nil || count != 4 {
		t.Fatalf("retained versions=%d %v", count, err)
	}
	if err = admin.DB.Table("audit_event_models").Where("owner_user_id = ? AND target_type = ?", owner, "policy_acceptance").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("audit=%d %v", count, err)
	}
	first.Idempotency.RequestHash = make([]byte, 32)
	if _, err = runtime.RenewPolicyAcceptance(ctx, first); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("key reuse=%v", err)
	}
	first.Idempotency.PrincipalID = peer
	if _, err = runtime.RenewPolicyAcceptance(ctx, first); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("forged principal=%v", err)
	}
}
