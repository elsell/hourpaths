package deletionjournal

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Files struct {
	directory string
	cipher    cipher.AEAD
}

func New(directory string, key []byte) (*Files, error) {
	if !filepath.IsAbs(directory) || len(key) != 32 {
		return nil, ports.ErrInvalidArgument
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	sealed, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0007 != 0 {
		return nil, ports.ErrInvalidArgument
	}
	return &Files{directory: directory, cipher: sealed}, nil
}
func valid(record application.DeletionRecord) bool {
	digest, err := hex.DecodeString(record.ReceiptHash)
	return record.UserID != "" && strings.TrimSpace(record.UserID) == record.UserID && len(record.UserID) <= 128 && !record.DeletedAt.IsZero() && record.AuditEventID != "" && len(record.AuditEventID) <= 128 && err == nil && len(digest) == 32
}
func (f *Files) syncDirectory() error {
	directory, err := os.Open(f.directory)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
func (f *Files) read(path, owner string) (application.DeletionRecord, error) {
	var record application.DeletionRecord
	info, err := os.Lstat(path)
	if err != nil {
		return record, err
	}
	if !info.Mode().IsRegular() || info.Size() > 8192 {
		return record, ports.ErrUnavailable
	}
	sealed, err := os.ReadFile(path)
	if err != nil {
		return record, err
	}
	size := f.cipher.NonceSize()
	if len(sealed) < size+f.cipher.Overhead() {
		return record, ports.ErrUnavailable
	}
	plain, err := f.cipher.Open(nil, sealed[:size], sealed[size:], []byte("hourpaths-deletion-v1:"+filepath.Base(path)))
	if err != nil {
		return record, ports.ErrUnavailable
	}
	if json.Unmarshal(plain, &record) != nil || !valid(record) || (owner != "" && record.UserID != owner) {
		return record, ports.ErrUnavailable
	}
	digest := sha256.Sum256([]byte(record.UserID))
	if filepath.Base(path) != hex.EncodeToString(digest[:])+".sealed" {
		return record, ports.ErrUnavailable
	}
	return record, nil
}

func (f *Files) Records(ctx context.Context) ([]application.DeletionRecord, error) {
	entries, err := os.ReadDir(f.directory)
	if err != nil {
		return nil, err
	}
	if len(entries) > 100000 {
		return nil, ports.ErrUnavailable
	}
	records := make([]application.DeletionRecord, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if strings.HasPrefix(entry.Name(), ".pending-") {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".sealed") || entry.IsDir() {
			return nil, ports.ErrUnavailable
		}
		record, err := f.read(filepath.Join(f.directory, entry.Name()), "")
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}
func (f *Files) Admit(ctx context.Context, record application.DeletionRecord) (application.DeletionRecord, error) {
	if err := ctx.Err(); err != nil {
		return application.DeletionRecord{}, err
	}
	if !valid(record) {
		return application.DeletionRecord{}, ports.ErrInvalidArgument
	}
	digest := sha256.Sum256([]byte(record.UserID))
	path := filepath.Join(f.directory, hex.EncodeToString(digest[:])+".sealed")
	existing, err := f.read(path, record.UserID)
	if err == nil {
		if existing.ReceiptHash != record.ReceiptHash {
			return application.DeletionRecord{}, ports.ErrConflict
		}
		return existing, f.syncDirectory()
	}
	if !errors.Is(err, os.ErrNotExist) {
		return application.DeletionRecord{}, err
	}
	plain, err := json.Marshal(record)
	if err != nil {
		return application.DeletionRecord{}, err
	}
	nonce := make([]byte, f.cipher.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return application.DeletionRecord{}, err
	}
	sealed := f.cipher.Seal(nonce, nonce, plain, []byte("hourpaths-deletion-v1:"+filepath.Base(path)))
	file, err := os.CreateTemp(f.directory, ".pending-")
	if err != nil {
		return application.DeletionRecord{}, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err = file.Write(sealed); err != nil {
		return application.DeletionRecord{}, err
	}
	if err = file.Sync(); err != nil {
		return application.DeletionRecord{}, err
	}
	if err = file.Close(); err != nil {
		return application.DeletionRecord{}, err
	}
	if err = os.Link(file.Name(), path); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return application.DeletionRecord{}, err
		}
		existing, err = f.read(path, record.UserID)
		if err != nil {
			return application.DeletionRecord{}, err
		}
		if existing.ReceiptHash != record.ReceiptHash {
			return application.DeletionRecord{}, ports.ErrConflict
		}
		return existing, f.syncDirectory()
	}
	return record, f.syncDirectory()
}

// CheckAccountAccess is fail-closed if the independent recovery volume is lost.
func (f *Files) CheckAccountAccess(ctx context.Context, userID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if userID == "" {
		return ports.ErrInvalidCredential
	}
	info, err := os.Lstat(f.directory)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0007 != 0 {
		return ports.ErrUnavailable
	}
	digest := sha256.Sum256([]byte(userID))
	path := filepath.Join(f.directory, hex.EncodeToString(digest[:])+".sealed")
	_, err = f.read(path, userID)
	if errors.Is(err, os.ErrNotExist) {
		// A volume disappearing between the first stat and lookup is not proof
		// that this account has no accepted request.
		current, statErr := os.Lstat(f.directory)
		if statErr != nil || !os.SameFile(info, current) {
			return ports.ErrUnavailable
		}
		return nil
	}
	if err != nil {
		return ports.ErrUnavailable
	}
	return ports.ErrInvalidCredential
}
