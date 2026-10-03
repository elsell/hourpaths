package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/deletionjournal"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fixtureManifest = flag.String("restore-manifest", "", "isolated database fixture manifest")

func TestManifestRejectsStaleOrAmbiguousRecoveryInput(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "records.json")
	value := manifest{Version: 1, ExportedAt: now.Add(-time.Hour), Records: []application.DeletionRecord{}}
	if err := writeManifest(path, value); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(path, value); err == nil {
		t.Fatal("overwrote authoritative manifest")
	}
	if _, err := readManifest(path, now.Add(-2*time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if _, err := readManifest(path, now.Add(-time.Minute), now); err == nil {
		t.Fatal("manifest predating backup accepted")
	}
	if _, err := readManifest(path, now.Add(-31*24*time.Hour), now); err == nil {
		t.Fatal("expired backup accepted")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("manifest not private")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{string(raw) + " {}", strings.Replace(string(raw), `"version":1`, `"version":1,"unexpected":true`, 1)} {
		if err := os.WriteFile(path, []byte(invalid), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readManifest(path, now.Add(-2*time.Hour), now); err == nil {
			t.Fatal("ambiguous manifest accepted")
		}
	}
}

func TestReplayExportedFixture(t *testing.T) {
	if *fixtureManifest == "" {
		t.Skip("isolated database manifest required")
	}
	raw, err := os.ReadFile(*fixtureManifest)
	if err != nil {
		t.Fatal(err)
	}
	var value manifest
	if err = json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	count, err := run(context.Background(), "", *fixtureManifest, value.ExportedAt.Add(-time.Second).Format(time.RFC3339Nano), true)
	if err != nil {
		t.Fatal(err)
	}
	if count != len(value.Records) {
		t.Fatal("incomplete replay")
	}
}

func TestExportSurvivesPrimaryDatabaseLoss(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	key := make([]byte, 32)
	journal, err := deletionjournal.New(directory, key)
	if err != nil {
		t.Fatal(err)
	}
	record := application.DeletionRecord{UserID: "deleted-owner", DeletedAt: time.Now().UTC().Add(-time.Minute), AuditEventID: "deletion-audit", ReceiptHash: strings.Repeat("01", 32)}
	if _, err = journal.Admit(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOURPATHS_DATABASE_DSN", "")
	t.Setenv("HOURPATHS_DELETION_JOURNAL_DIRECTORY", directory)
	t.Setenv("HOURPATHS_DELETION_JOURNAL_KEY", hex.EncodeToString(key))
	output := filepath.Join(t.TempDir(), "records.json")
	count, err := run(context.Background(), output, "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("accepted deletion missing from export")
	}
	restored, err := readManifest(output, record.DeletedAt.Add(-time.Hour), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Records) != 1 || restored.Records[0].UserID != record.UserID || restored.Records[0].ReceiptHash != record.ReceiptHash {
		t.Fatal("deletion evidence changed")
	}
}
