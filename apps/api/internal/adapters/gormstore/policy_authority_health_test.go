package gormstore

import (
	"context"
	"errors"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledPolicyAuthority struct {
	policySet ports.PolicySet
	err       error
}

func (a controlledPolicyAuthority) Current(context.Context) (ports.PolicySet, error) {
	return a.policySet, a.err
}

func TestPolicyAuthorityHealthFailsClosedUntilTheSharedAuthorityIsReadable(t *testing.T) {
	databaseErr := errors.New("policy database unavailable")
	for _, test := range []struct {
		name      string
		authority ports.PolicyAuthority
		wantErr   bool
	}{
		{name: "current", authority: controlledPolicyAuthority{policySet: policySet(7, "current")}},
		{name: "malformed", authority: controlledPolicyAuthority{}, wantErr: true},
		{name: "missing", authority: controlledPolicyAuthority{err: ports.ErrNotFound}, wantErr: true},
		{name: "unavailable", authority: controlledPolicyAuthority{err: databaseErr}, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := (PolicyAuthorityHealth{Authority: test.authority}).Health(context.Background())
			if (err != nil) != test.wantErr {
				t.Fatalf("health error = %v, wantErr=%v", err, test.wantErr)
			}
		})
	}
}
