package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore"
	"github.com/elsell/hour-paths/apps/api/internal/config"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type commandDependencies struct {
	loadDatabase  func() (string, bool, error)
	loadPolicy    func(bool) (config.PolicyConfiguration, error)
	openPublisher func(string) (ports.PolicyPublisher, func() error, error)
}

type commandOptions struct {
	updatedAt               time.Time
	allowInsecurePolicyURLs bool
}

func parseOptions(arguments []string) (commandOptions, error) {
	flags := flag.NewFlagSet("policy-publish", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	updatedAt := flags.String("updated-at", "", "exact RFC3339 policy publication instant")
	allowInsecure := flags.Bool("allow-insecure-policy-urls", false, "allow HTTP policy locations for explicit local development")
	if err := flags.Parse(arguments); err != nil {
		return commandOptions{}, err
	}
	if flags.NArg() != 0 || *updatedAt == "" {
		return commandOptions{}, ports.ErrInvalidArgument
	}
	parsed, err := time.Parse(time.RFC3339Nano, *updatedAt)
	if err != nil || parsed.IsZero() || !parsed.Equal(parsed.Truncate(time.Microsecond)) {
		return commandOptions{}, ports.ErrInvalidArgument
	}
	return commandOptions{updatedAt: parsed, allowInsecurePolicyURLs: *allowInsecure}, nil
}

func run(ctx context.Context, arguments []string, dependencies commandDependencies, stdout, stderr io.Writer) int {
	options, err := parseOptions(arguments)
	if err != nil {
		fmt.Fprintln(stderr, "policy publication arguments invalid")
		return 2
	}
	dsn, _, err := dependencies.loadDatabase()
	if err != nil {
		fmt.Fprintln(stderr, "policy publication database configuration invalid")
		return 1
	}
	configured, err := dependencies.loadPolicy(options.allowInsecurePolicyURLs)
	if err != nil {
		fmt.Fprintln(stderr, "policy publication authority configuration invalid")
		return 1
	}
	requested := ports.PolicySet{
		Revision:                   configured.Revision,
		TermsVersion:               configured.CurrentTermsVersion,
		PrivacyPolicyVersion:       configured.CurrentPrivacyPolicyVersion,
		CommunityGuidelinesVersion: configured.CurrentCommunityGuidelinesVersion,
		TermsURL:                   configured.TermsURL,
		PrivacyPolicyURL:           configured.PrivacyPolicyURL,
		CommunityGuidelinesURL:     configured.CommunityGuidelinesURL,
		SupportURL:                 configured.SupportURL,
		UpdatedAt:                  options.updatedAt,
	}
	if err := ports.ValidatePolicySet(requested); err != nil {
		fmt.Fprintln(stderr, "policy publication authority configuration invalid")
		return 1
	}
	publisher, closePublisher, err := dependencies.openPublisher(dsn)
	if err != nil {
		fmt.Fprintln(stderr, "policy publication database open failed")
		return 1
	}
	shared, publishErr := publisher.Publish(ctx, requested)
	closeErr := closePublisher()
	if publishErr != nil {
		fmt.Fprintln(stderr, "policy publication failed")
		return 1
	}
	if closeErr != nil {
		fmt.Fprintln(stderr, "policy publication database close failed")
		return 1
	}
	fmt.Fprintf(stdout, "policy authority revision %d is current\n", shared.Revision)
	return 0
}

func main() {
	dependencies := commandDependencies{
		loadDatabase: config.LoadDatabase,
		loadPolicy:   config.LoadPolicyPublication,
		openPublisher: func(dsn string) (ports.PolicyPublisher, func() error, error) {
			store, err := gormstore.Open("postgres", dsn)
			if err != nil {
				return nil, nil, err
			}
			database, err := store.DB.DB()
			if err != nil {
				return nil, nil, err
			}
			return store, database.Close, nil
		},
	}
	os.Exit(run(context.Background(), os.Args[1:], dependencies, os.Stdout, os.Stderr))
}
