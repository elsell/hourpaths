package main

import (
	"context"
	"errors"
	"flag"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/auditretention"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/observability"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func main() {
	deletedAccounts := flag.Bool("deleted-accounts", false, "expire deleted-account audit details independently of general retention")
	flag.Parse()
	dsn := os.Getenv("HOURPATHS_AUDIT_RETENTION_DSN")
	days, daysErr := strconv.Atoi(os.Getenv("HOURPATHS_AUDIT_RETENTION_DAYS"))
	batch, batchErr := strconv.Atoi(os.Getenv("HOURPATHS_AUDIT_RETENTION_BATCH_SIZE"))
	dryRun, dryRunErr := strconv.ParseBool(value("HOURPATHS_AUDIT_RETENTION_DRY_RUN", "true"))
	if dsn == "" || (!*deletedAccounts && (daysErr != nil || days < 30 || days > 3650)) || batchErr != nil || batch < 1 || batch > 10000 || dryRunErr != nil {
		panic(errors.New("validated retention DSN, 30-3650 days, batch size 1-10000, and dry-run flag are required"))
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		slog.Error("audit retention operation failed")
		os.Exit(1)
	}
	now := time.Now().UTC()
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	runner := auditretention.Runner{DB: db, Probe: observability.StructuredLog{Logger: logger}}
	if *deletedAccounts {
		if dryRun {
			panic(errors.New("deleted-account retention requires explicit HOURPATHS_AUDIT_RETENTION_DRY_RUN=false"))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		total := 0
		for batches := 0; batches < 1000; batches++ {
			deleted, err := runner.DeletedAccountBatch(ctx, batch)
			if err != nil {
				slog.Error("audit retention operation failed")
				os.Exit(1)
			}
			total += deleted
			if deleted < batch {
				logger.Info("deleted account retention complete", "deleted_records", total)
				return
			}
		}
		panic(errors.New("deleted-account retention batch budget exhausted; rerun required"))
	}
	if dryRun {
		count, err := runner.Count(context.Background(), cutoff)
		if err != nil {
			slog.Error("audit retention operation failed")
			os.Exit(1)
		}
		logger.Info("audit retention dry run", "eligible_events", count)
		return
	}
	total := 0
	for {
		deleted, err := runner.DeleteBatch(context.Background(), cutoff, time.Now().UTC(), batch)
		if err != nil {
			slog.Error("audit retention operation failed")
			os.Exit(1)
		}
		total += deleted
		if deleted < batch {
			break
		}
	}
	logger.Info("audit retention complete", "deleted_events", total)
}

func value(key, fallback string) string {
	// hourpaths-env-inventory: allow-computed reviewed bounded retention value helper
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
