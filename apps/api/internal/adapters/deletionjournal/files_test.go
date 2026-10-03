package deletionjournal

import (
	"bytes"
	"context"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEncryptedAdmissionSurvivesReopenAndRejectsTamperingOrReplacement(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	key := bytes.Repeat([]byte{7}, 32)
	journal, err := New(directory, key)
	if err != nil {
		t.Fatal(err)
	}
	original := application.DeletionRecord{UserID: "owner", DeletedAt: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), AuditEventID: "original", ReceiptHash: strings.Repeat("01", 32)}
	if _, err = journal.Admit(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	journal, err = New(directory, key)
	if err != nil {
		t.Fatal(err)
	}
	records, err := journal.Records(context.Background())
	if err != nil || len(records) != 1 || records[0].UserID != original.UserID {
		t.Fatal("external export failed", err)
	}
	retry := original
	retry.AuditEventID = "replacement"
	retry.DeletedAt = retry.DeletedAt.Add(time.Hour)
	accepted, err := journal.Admit(context.Background(), retry)
	if err != nil || accepted.AuditEventID != original.AuditEventID || !accepted.DeletedAt.Equal(original.DeletedAt) {
		t.Fatal("original intent lost", err)
	}
	retry.ReceiptHash = strings.Repeat("02", 32)
	if _, err = journal.Admit(context.Background(), retry); err == nil {
		t.Fatal("competing capability replaced intent")
	}
	files, err := os.ReadDir(directory)
	if err != nil || len(files) != 4 {
		t.Fatal("unexpected journal files", err)
	}
	var path string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".sealed") {
			path = filepath.Join(directory, file.Name())
		}
	}
	sealed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte(original.UserID)) || bytes.Contains(sealed, []byte(original.ReceiptHash)) {
		t.Fatal("unencrypted record")
	}
	sealed[len(sealed)-1] ^= 1
	if err = os.WriteFile(path, sealed, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = journal.Admit(context.Background(), original); err == nil {
		t.Fatal("tampered journal admitted")
	}
}
