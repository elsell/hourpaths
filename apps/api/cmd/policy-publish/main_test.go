package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/config"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledPublisher struct {
	got    ports.PolicySet
	result ports.PolicySet
	err    error
}

func (publisher *controlledPublisher) Publish(_ context.Context, value ports.PolicySet) (ports.PolicySet, error) {
	publisher.got = value
	if publisher.result.Revision == 0 {
		publisher.result = value
	}
	return publisher.result, publisher.err
}

func TestRunPublishesValidatedExplicitPolicySet(t *testing.T) {
	publisher := &controlledPublisher{}
	var stdout, stderr bytes.Buffer
	exit := run(context.Background(), []string{"-updated-at", "2026-07-21T12:00:00Z"}, commandDependencies{
		loadDatabase: func() (string, bool, error) { return "postgres://migrator@db/app?sslmode=verify-full", false, nil },
		loadPolicy: func(insecure bool) (config.PolicyConfiguration, error) {
			if insecure {
				t.Fatal("secure command selected insecure policy validation")
			}
			return validPolicyConfiguration(), nil
		},
		openPublisher: func(dsn string) (ports.PolicyPublisher, func() error, error) {
			if !strings.Contains(dsn, "migrator") {
				t.Fatalf("publisher DSN = %q", dsn)
			}
			return publisher, func() error { return nil }, nil
		},
	}, &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	if publisher.got.Revision != 7 || !publisher.got.UpdatedAt.Equal(time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("published = %#v", publisher.got)
	}
	if stdout.String() != "policy authority revision 7 is current\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunHasDeterministicArgumentConfigurationAndPublicationFailures(t *testing.T) {
	base := commandDependencies{
		loadDatabase: func() (string, bool, error) { return "dsn", false, nil },
		loadPolicy:   func(bool) (config.PolicyConfiguration, error) { return validPolicyConfiguration(), nil },
		openPublisher: func(string) (ports.PolicyPublisher, func() error, error) {
			return &controlledPublisher{}, func() error { return nil }, nil
		},
	}
	tests := []struct {
		name   string
		args   []string
		mutate func(*commandDependencies)
		want   int
	}{
		{name: "missing timestamp", want: 2},
		{name: "malformed timestamp", args: []string{"-updated-at", "today"}, want: 2},
		{name: "unexpected argument", args: []string{"-updated-at", "2026-07-21T12:00:00Z", "extra"}, want: 2},
		{name: "database configuration", args: []string{"-updated-at", "2026-07-21T12:00:00Z"}, mutate: func(d *commandDependencies) {
			d.loadDatabase = func() (string, bool, error) { return "", false, errors.New("bad database") }
		}, want: 1},
		{name: "policy configuration", args: []string{"-updated-at", "2026-07-21T12:00:00Z"}, mutate: func(d *commandDependencies) {
			d.loadPolicy = func(bool) (config.PolicyConfiguration, error) {
				return config.PolicyConfiguration{}, errors.New("bad policy")
			}
		}, want: 1},
		{name: "publication conflict", args: []string{"-updated-at", "2026-07-21T12:00:00Z"}, mutate: func(d *commandDependencies) {
			d.openPublisher = func(string) (ports.PolicyPublisher, func() error, error) {
				return &controlledPublisher{err: ports.ErrConflict}, func() error { return nil }, nil
			}
		}, want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps := base
			if test.mutate != nil {
				test.mutate(&deps)
			}
			var stdout, stderr bytes.Buffer
			if got := run(context.Background(), test.args, deps, &stdout, &stderr); got != test.want {
				t.Fatalf("exit=%d want=%d stderr=%q", got, test.want, stderr.String())
			}
			if stderr.Len() == 0 {
				t.Fatal("failure did not report a stable diagnostic")
			}
		})
	}
}

func validPolicyConfiguration() config.PolicyConfiguration {
	return config.PolicyConfiguration{Revision: 7, CurrentTermsVersion: "terms-v7", CurrentPrivacyPolicyVersion: "privacy-v7", CurrentCommunityGuidelinesVersion: "guidelines-v7", TermsURL: "https://app.example/legal/terms", PrivacyPolicyURL: "https://app.example/legal/privacy", CommunityGuidelinesURL: "https://app.example/community-guidelines", SupportURL: "https://app.example/support"}
}
