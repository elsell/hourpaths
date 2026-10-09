package app

import (
	"bytes"
	"context"
	"errors"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type controlledPolicyAcceptances struct {
	accepted bool
	err      error
	owners   []string
	commands []PolicyRenewalCommand
}

func (r *controlledPolicyAcceptances) HasAcceptedPolicies(_ context.Context, owner string, _ identity.CurrentPolicyVersions) (bool, error) {
	r.owners = append(r.owners, owner)
	return r.accepted, r.err
}
func (r *controlledPolicyAcceptances) RenewPolicyAcceptance(_ context.Context, c PolicyRenewalCommand) (PolicyRenewalResult, error) {
	r.commands = append(r.commands, c)
	return PolicyRenewalResult{Policies: c.Policies, AcceptedAt: c.AcceptedAt}, r.err
}
func policyRenewalTestApp(r *controlledPolicyAcceptances) App {
	a := timeZoneTestApp(nil)
	a.PolicyAcceptances = r
	a.CursorSigningKey = bytes.Repeat([]byte{17}, 32)
	a.PolicyAuthority = staticPolicyAuthority{policySet: testPolicySet(identity.CurrentPolicyVersions{TermsOfService: "terms-v2", PrivacyPolicy: "privacy-v1", CommunityGuidelines: "guidelines-v1"})}
	return a
}
func TestPolicyRenewalReviewRequiresMissingAcceptanceWithoutLosingAccount(t *testing.T) {
	r := &controlledPolicyAcceptances{}
	a := policyRenewalTestApp(r)
	review, err := a.ReviewCurrentPolicies(context.Background(), "Bearer session")
	if err != nil || !review.Required || len(r.owners) != 1 || r.owners[0] != "owner" {
		t.Fatalf("review=%+v owners=%v err=%v", review, r.owners, err)
	}
	payload, err := shared.DecodePolicyRenewalReviewToken(a.CursorSigningKey, review.ReviewToken)
	if err != nil || payload.Owner != "owner" || payload.Policies.TermsOfService != "terms-v2" {
		t.Fatalf("review binding=%+v err=%v", payload, err)
	}
	r.accepted = true
	review, err = a.ReviewCurrentPolicies(context.Background(), "Bearer session")
	if err != nil || review.Required {
		t.Fatalf("accepted review=%+v err=%v", review, err)
	}
	r.err = ports.ErrUnavailable
	if _, err = a.ReviewCurrentPolicies(context.Background(), "Bearer session"); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatalf("storage failure=%v", err)
	}
}
func TestPolicyRenewalRejectsForeignOrIncompleteReviewAndCarriesAtomicEvidence(t *testing.T) {
	r := &controlledPolicyAcceptances{}
	a := policyRenewalTestApp(r)
	review, err := a.ReviewCurrentPolicies(context.Background(), "Bearer session")
	if err != nil {
		t.Fatal(err)
	}
	input := PolicyRenewalInput{ReviewToken: review.ReviewToken, TermsAccepted: true, PrivacyAcknowledged: true, GuidelinesAccepted: true}
	incomplete := input
	incomplete.PrivacyAcknowledged = false
	if _, err = a.AcceptCurrentPolicies(context.Background(), "Bearer session", "renewal-key-0001", incomplete); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("incomplete=%v", err)
	}
	a.Auth = fakeAuth{principal: ports.Principal{UserID: "other", Scopes: []string{"api:user"}}}
	a.Users = fakeUsers{user: identity.User{ID: "other", Status: identity.StatusActive}}
	if _, err = a.AcceptCurrentPolicies(context.Background(), "Bearer other", "renewal-key-0001", input); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("foreign=%v", err)
	}
	if len(r.commands) != 0 {
		t.Fatal("invalid review reached persistence")
	}
	a = policyRenewalTestApp(r)
	result, err := a.AcceptCurrentPolicies(context.Background(), "Bearer session", "renewal-key-0001", input)
	if err != nil || result.AcceptedAt.IsZero() || len(r.commands) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	c := r.commands[0]
	if c.ActorUserID != "owner" || c.PolicyRevision != 7 || !c.Audit.Valid() || c.Audit.OccurredAt != c.AcceptedAt || c.Idempotency.PrincipalID != "owner" {
		t.Fatalf("command=%+v", c)
	}
	a.Clock = fakeClock{now: a.Clock.Now().Add(time.Hour)}
	_, err = a.AcceptCurrentPolicies(context.Background(), "Bearer session", "renewal-key-0001", input)
	if err != nil || !bytes.Equal(c.Idempotency.RequestHash, r.commands[1].Idempotency.RequestHash) {
		t.Fatal("retry changed request identity")
	}
}

func TestPolicyAdmissionDeniesWithoutInvalidatingSessionAndFailsClosed(t *testing.T) {
	r := &controlledPolicyAcceptances{}
	a := policyRenewalTestApp(r)
	if err := a.AdmitPolicyUse(context.Background(), "Bearer session"); !errors.Is(err, ports.ErrPolicyAcceptanceRequired) {
		t.Fatalf("missing acceptance=%v", err)
	}
	r.accepted = true
	if err := a.AdmitPolicyUse(context.Background(), "Bearer session"); err != nil {
		t.Fatalf("accepted=%v", err)
	}
	r.err = ports.ErrUnavailable
	if err := a.AdmitPolicyUse(context.Background(), "Bearer session"); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatalf("dependency failure=%v", err)
	}
	a.PolicyAcceptances = nil
	if err := a.AdmitPolicyUse(context.Background(), "Bearer session"); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatalf("missing dependency=%v", err)
	}
}
