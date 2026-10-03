package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/deletionjournal"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/config"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type manifest struct {
	MinimumBackupTime time.Time                    `json:"minimumBackupTime"`
	Version           int                          `json:"version"`
	ExportedAt        time.Time                    `json:"exportedAt"`
	Records           []application.DeletionRecord `json:"records"`
}

func readManifest(path string, backup, now time.Time) (manifest, error) {
	var value manifest
	file, err := os.Open(path)
	if err != nil {
		return value, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return value, err
	}
	if info.Size() > 64<<20 {
		return value, errors.New("manifest too large")
	}
	decoder := json.NewDecoder(io.LimitReader(file, 64<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return value, errors.New("manifest must contain exactly one object")
	}
	if value.Version != 2 || value.MinimumBackupTime.IsZero() || value.MinimumBackupTime.After(value.ExportedAt) || backup.Before(value.MinimumBackupTime) || value.Records == nil || value.ExportedAt.IsZero() || value.ExportedAt.Before(backup) || value.ExportedAt.After(now) || backup.IsZero() || backup.After(now) || backup.Before(now.Add(-30*24*time.Hour)) {
		return value, errors.New("manifest or backup time invalid")
	}
	for _, record := range value.Records {
		if record.DeletedAt.After(value.ExportedAt) {
			return value, errors.New("deletion after manifest snapshot")
		}
	}
	return value, nil
}

func writeManifest(path string, value manifest) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		file.Close()
		if !success {
			os.Remove(path)
		}
	}()
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	if err = directory.Sync(); err != nil {
		return err
	}
	success = true
	return nil
}

func run(ctx context.Context, exportPath, replayPath, backupTime string, offline bool) (int, error) {
	if (exportPath == "") == (replayPath == "") {
		return 0, errors.New("choose exactly one export or replay file")
	}
	if exportPath != "" {
		directory, key, err := config.LoadDeletionJournal()
		if err != nil {
			return 0, err
		}
		// Export must not create a missing recovery source and report an empty success.
		if _, err = os.Stat(directory); err != nil {
			return 0, err
		}
		journal, err := deletionjournal.New(directory, key)
		if err != nil {
			return 0, err
		}
		records, err := journal.Records(ctx)
		if err != nil {
			return 0, err
		}
		now := time.Now().UTC()
		return len(records), writeManifest(exportPath, manifest{Version: 2, ExportedAt: now, MinimumBackupTime: now.Add(-30 * 24 * time.Hour), Records: records})
	}
	dsn := os.Getenv("HOURPATHS_DATABASE_DSN")
	if dsn == "" {
		return 0, errors.New("database configuration required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return 0, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return 0, err
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	store := &gormstore.Store{DB: db}
	if !offline {
		return 0, errors.New("offline replay must be explicitly selected")
	}
	backup, err := time.Parse(time.RFC3339Nano, backupTime)
	if err != nil {
		return 0, err
	}
	records, err := readManifest(replayPath, backup, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	var clients int64
	if err = db.WithContext(ctx).Raw("SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND backend_type='client backend'").Scan(&clients).Error; err != nil {
		return 0, err
	}
	if clients != 0 {
		return 0, errors.New("other database clients must be stopped before replay")
	}
	if err = application.ReapplyDeletionRecords(ctx, records.Records, store, uuid.NewString); err != nil {
		return 0, err
	}
	return len(records.Records), nil
}

func main() {
	exportPath := flag.String("export", "", "new private deletion manifest file")
	replayPath := flag.String("replay", "", "external deletion manifest to reapply")
	backupTime := flag.String("backup-created-at", "", "RFC3339 creation time of restored backup")
	offline := flag.Bool("offline", false, "confirm API and workers are stopped")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	count, err := run(ctx, *exportPath, *replayPath, *backupTime, *offline)
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err != nil {
		log.Error("deletion record operation failed; traffic must remain stopped")
		os.Exit(1)
	}
	log.Info("deletion record operation complete; restore acceptance is still required", "records", count)
}
