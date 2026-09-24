package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/dbmigrations"
	"github.com/elsell/hour-paths/apps/api/internal/config"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type migrationRunner interface {
	Version() (uint, bool, error)
	Migrate(uint) error
	Up() error
}

type migrationPreflight interface {
	HasPendingAuthorizationChanges(context.Context) (bool, error)
}

type postgresMigrationPreflight struct {
	db *sql.DB
}

const authorizationOutboxOrderingVersion uint = 18

func (p postgresMigrationPreflight) HasPendingAuthorizationChanges(ctx context.Context) (bool, error) {
	var pending bool
	err := p.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM public.authorization_outbox_models
			WHERE completed_at IS NULL
		)
	`).Scan(&pending)
	return pending, err
}

func parseTargetVersion(arguments []string) (*uint, error) {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	target := flags.Uint("target-version", 0, "apply migrations up through this version")
	if err := flags.Parse(arguments); err != nil {
		return nil, err
	}
	if flags.NArg() != 0 {
		return nil, fmt.Errorf("unexpected migration arguments: %v", flags.Args())
	}
	specified := false
	flags.Visit(func(current *flag.Flag) {
		if current.Name == "target-version" {
			specified = true
		}
	})
	if !specified {
		return nil, nil
	}
	if *target == 0 || *target > dbmigrations.LatestVersion {
		return nil, fmt.Errorf("target migration version must be between 1 and %d", dbmigrations.LatestVersion)
	}
	return target, nil
}

func applyMigrations(ctx context.Context, runner migrationRunner, target *uint, preflight migrationPreflight) error {
	current, dirty, err := runner.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("read migration version: %w", err)
	}
	hasCurrentVersion := err == nil
	if hasCurrentVersion && dirty {
		return fmt.Errorf("migration ledger is dirty at version %d", current)
	}
	if target != nil && hasCurrentVersion {
		if current > *target {
			return fmt.Errorf("refusing to migrate backward from version %d to %d", current, *target)
		}
		if current == *target {
			return nil
		}
	}
	if hasCurrentVersion && current < authorizationOutboxOrderingVersion &&
		(target == nil || *target >= authorizationOutboxOrderingVersion) {
		pending, err := preflight.HasPendingAuthorizationChanges(ctx)
		if err != nil {
			return fmt.Errorf("check authorization outbox drain before migration 18: %w", err)
		}
		if pending {
			return errors.New("migration 18 requires every authorization outbox row to be completed; drain authorization delivery before retrying")
		}
	}
	if target == nil {
		if err := runner.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		return nil
	}
	if err := runner.Migrate(*target); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func main() {
	target, err := parseTargetVersion(os.Args[1:])
	if err != nil {
		slog.Error("migration arguments invalid", "error", err)
		os.Exit(2)
	}
	dsn, _, err := config.LoadDatabase()
	if err != nil {
		slog.Error("database configuration invalid", "error", err)
		os.Exit(1)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		slog.Error("database open failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	source, err := iofs.New(dbmigrations.Files, ".")
	if err != nil {
		slog.Error("migration source failed", "error", err)
		os.Exit(1)
	}
	database, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		slog.Error("migration database failed", "error", err)
		os.Exit(1)
	}
	runner, err := migrate.NewWithInstance("iofs", source, "postgres", database)
	if err != nil {
		slog.Error("migration runner failed", "error", err)
		os.Exit(1)
	}
	if err := applyMigrations(context.Background(), runner, target, postgresMigrationPreflight{db: db}); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
}
