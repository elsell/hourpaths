package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/operationalmoderation"
	app "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/config"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
)

type dependencies struct {
	loadDatabase func() (string, bool, error)
	readReason   func(string) ([]byte, error)
	open         func(string) (app.AccountEnforcementPort, func() error, error)
}

func run(ctx context.Context, arguments []string, deps dependencies, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("account-enforce", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	caseID := flags.String("case", "", "reviewed case ID")
	noticeID := flags.String("notice", "", "stable unique notice ID; reuse for retries")
	action := flags.String("action", "", "suspension or ban")
	until := flags.String("until", "", "RFC3339 suspension expiry; omit for bans")
	reasonFile := flags.String("reason-file", "", "UTF-8 file containing the subject-visible policy reason")
	if flags.Parse(arguments) != nil || flags.NArg() != 0 || *reasonFile == "" {
		fmt.Fprintln(stderr, "account enforcement arguments invalid")
		return 2
	}
	command := app.AccountEnforcementCommand{CaseID: *caseID, NoticeID: *noticeID, Action: domain.EnforcementAction(*action)}
	if *until != "" {
		parsed, err := time.Parse(time.RFC3339Nano, *until)
		if err != nil || !parsed.Equal(parsed.Truncate(time.Microsecond)) {
			fmt.Fprintln(stderr, "account enforcement expiry invalid")
			return 2
		}
		command.Until = parsed.UTC()
	}
	reason, err := deps.readReason(*reasonFile)
	if err != nil || len(reason) > 4096 {
		fmt.Fprintln(stderr, "account enforcement reason unavailable")
		return 2
	}
	command.PolicyReason = strings.TrimSpace(string(reason))
	if command.Validate() != nil {
		fmt.Fprintln(stderr, "account enforcement arguments invalid")
		return 2
	}
	dsn, _, err := deps.loadDatabase()
	if err != nil {
		fmt.Fprintln(stderr, "account enforcement database configuration invalid")
		return 1
	}
	service, closeDB, err := deps.open(dsn)
	if err != nil {
		fmt.Fprintln(stderr, "account enforcement database open failed")
		return 1
	}
	_, executeErr := service.RestrictAccount(ctx, command)
	closeErr := closeDB()
	if executeErr != nil {
		fmt.Fprintln(stderr, "account enforcement failed; retry with the same notice ID to confirm the outcome")
		return 1
	}
	if closeErr != nil {
		fmt.Fprintln(stderr, "account enforcement committed; database close failed")
		return 1
	}
	fmt.Fprintln(stdout, "account restriction committed; server timers stopped and elapsed time saved")
	return 0
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

var _ ports.Clock = systemClock{}

func main() {
	deps := dependencies{loadDatabase: config.LoadDatabase, readReason: os.ReadFile, open: func(dsn string) (app.AccountEnforcementPort, func() error, error) {
		store, err := gormstore.Open("postgres", dsn)
		if err != nil {
			return nil, nil, err
		}
		db, err := store.DB.DB()
		if err != nil {
			return nil, nil, err
		}
		return operationalmoderation.AccountEnforcer{DB: store.DB, Clock: systemClock{}, NewID: uuid.NewString}, db.Close, nil
	}}
	os.Exit(run(context.Background(), os.Args[1:], deps, os.Stdout, os.Stderr))
}
