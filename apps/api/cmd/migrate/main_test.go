package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/dbmigrations"
	"github.com/golang-migrate/migrate/v4"
)

type controlledMigrationRunner struct {
	version    uint
	dirty      bool
	versionErr error
	calls      []string
}

type controlledMigrationPreflight struct {
	pending bool
	err     error
	calls   int
}

func (p *controlledMigrationPreflight) HasPendingAuthorizationChanges(context.Context) (bool, error) {
	p.calls++
	return p.pending, p.err
}

func (r *controlledMigrationRunner) Version() (uint, bool, error) {
	return r.version, r.dirty, r.versionErr
}

func (r *controlledMigrationRunner) Migrate(version uint) error {
	r.calls = append(r.calls, "migrate")
	r.version = version
	return nil
}

func (r *controlledMigrationRunner) Up() error {
	r.calls = append(r.calls, "up")
	return nil
}

func TestParseTargetVersion(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		want      *uint
		wantError bool
	}{
		{name: "current", arguments: nil, want: nil},
		{name: "prior release", arguments: []string{"-target-version", "14"}, want: uintPointer(14)},
		{name: "frozen prior release", arguments: []string{"-target-version", "15"}, want: uintPointer(15)},
		{name: "zero", arguments: []string{"-target-version", "0"}, wantError: true},
		{name: "beyond current", arguments: []string{"-target-version", fmt.Sprint(dbmigrations.LatestVersion + 1)}, wantError: true},
		{name: "malformed", arguments: []string{"-target-version", "fourteen"}, wantError: true},
		{name: "unexpected argument", arguments: []string{"extra"}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseTargetVersion(test.arguments)
			if (err != nil) != test.wantError {
				t.Fatalf("parseTargetVersion() error = %v, wantError = %t", err, test.wantError)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("parseTargetVersion() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestApplyMigrations(t *testing.T) {
	tests := []struct {
		name               string
		runner             *controlledMigrationRunner
		preflight          *controlledMigrationPreflight
		target             *uint
		wantCalls          []string
		wantPreflightCalls int
		wantError          bool
	}{
		{
			name:               "current release",
			runner:             &controlledMigrationRunner{version: 17},
			preflight:          &controlledMigrationPreflight{},
			wantCalls:          []string{"up"},
			wantPreflightCalls: 1,
		},
		{
			name:      "fresh database to prior release",
			runner:    &controlledMigrationRunner{versionErr: migrate.ErrNilVersion},
			preflight: &controlledMigrationPreflight{},
			target:    uintPointer(14),
			wantCalls: []string{"migrate"},
		},
		{
			name:      "already at target",
			runner:    &controlledMigrationRunner{version: 14},
			preflight: &controlledMigrationPreflight{},
			target:    uintPointer(14),
			wantCalls: nil,
		},
		{
			name:      "downgrade refused",
			runner:    &controlledMigrationRunner{version: 15},
			preflight: &controlledMigrationPreflight{},
			target:    uintPointer(14),
			wantCalls: nil,
			wantError: true,
		},
		{
			name:      "dirty ledger refused",
			runner:    &controlledMigrationRunner{version: 13, dirty: true},
			preflight: &controlledMigrationPreflight{},
			target:    uintPointer(14),
			wantCalls: nil,
			wantError: true,
		},
		{
			name:      "version read failure",
			runner:    &controlledMigrationRunner{versionErr: errors.New("database unavailable")},
			preflight: &controlledMigrationPreflight{},
			target:    uintPointer(14),
			wantCalls: nil,
			wantError: true,
		},
		{
			name:               "pending authorization work refuses version 18",
			runner:             &controlledMigrationRunner{version: 17},
			preflight:          &controlledMigrationPreflight{pending: true},
			target:             uintPointer(authorizationOutboxOrderingVersion),
			wantPreflightCalls: 1,
			wantError:          true,
		},
		{
			name:               "authorization preflight failure refuses version 18",
			runner:             &controlledMigrationRunner{version: 17},
			preflight:          &controlledMigrationPreflight{err: errors.New("database unavailable")},
			wantPreflightCalls: 1,
			wantError:          true,
		},
		{
			name:      "version 17 target does not require drain",
			runner:    &controlledMigrationRunner{version: 16},
			preflight: &controlledMigrationPreflight{pending: true},
			target:    uintPointer(17),
			wantCalls: []string{"migrate"},
		},
		{
			name:      "fresh database current release",
			runner:    &controlledMigrationRunner{versionErr: migrate.ErrNilVersion},
			preflight: &controlledMigrationPreflight{pending: true},
			wantCalls: []string{"up"},
		},
		{
			name:      "already past authorization ordering migration",
			runner:    &controlledMigrationRunner{version: authorizationOutboxOrderingVersion},
			preflight: &controlledMigrationPreflight{pending: true},
			wantCalls: []string{"up"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := applyMigrations(context.Background(), test.runner, test.target, test.preflight)
			if (err != nil) != test.wantError {
				t.Fatalf("applyMigrations() error = %v, wantError = %t", err, test.wantError)
			}
			if !reflect.DeepEqual(test.runner.calls, test.wantCalls) {
				t.Fatalf("calls = %v, want %v", test.runner.calls, test.wantCalls)
			}
			if test.preflight.calls != test.wantPreflightCalls {
				t.Fatalf("preflight calls = %d, want %d", test.preflight.calls, test.wantPreflightCalls)
			}
		})
	}
}

func uintPointer(value uint) *uint {
	return &value
}
