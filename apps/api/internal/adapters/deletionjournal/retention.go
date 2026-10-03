package deletionjournal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const boundaryFile = ".backup-boundary"
const lockFile = ".retention-lock"
const initializedFile = ".initialized"

// Cross-process locking keeps export snapshots consistent with retirement on the
// shared journal volume. No database credential is needed to export a snapshot.
func (f *Files) locked(ctx context.Context, run func() error) error {
	file, err := os.OpenFile(filepath.Join(f.directory, lockFile), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return run()
}

func (f *Files) boundary() (time.Time, error) {
	var value time.Time
	data, err := os.ReadFile(filepath.Join(f.directory, boundaryFile))
	if err != nil {
		return value, err
	}
	n := f.cipher.NonceSize()
	if len(data) < n+f.cipher.Overhead() || len(data) > 1024 {
		return value, ports.ErrUnavailable
	}
	plain, err := f.cipher.Open(nil, data[:n], data[n:], []byte("hourpaths-deletion-backup-boundary-v1"))
	if err != nil {
		return value, ports.ErrUnavailable
	}
	if err = json.Unmarshal(plain, &value); err != nil {
		return value, ports.ErrUnavailable
	}
	return value, nil
}

func (f *Files) writeBoundary(value time.Time) error {
	plain, err := json.Marshal(value)
	if err != nil {
		return err
	}
	nonce := make([]byte, f.cipher.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	data := f.cipher.Seal(nonce, nonce, plain, []byte("hourpaths-deletion-backup-boundary-v1"))
	file, err := os.CreateTemp(f.directory, ".pending-boundary-")
	if err != nil {
		return err
	}
	defer file.Close()
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(file.Name(), filepath.Join(f.directory, boundaryFile)); err != nil {
		return err
	}
	return f.syncDirectory()
}

func (f *Files) initializeBoundary(ctx context.Context) error {
	return f.locked(ctx, func() error {
		_, err := f.boundary()
		if err == nil {
			return f.markInitialized()
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		entries, err := os.ReadDir(f.directory)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Name() != lockFile {
				return ports.ErrUnavailable
			}
		}
		if err := f.writeBoundary(time.Time{}); err != nil {
			return err
		}
		return f.markInitialized()
	})
}

func (f *Files) Snapshot(ctx context.Context) (records []application.DeletionRecord, minimumBackup time.Time, err error) {
	err = f.locked(ctx, func() error {
		var failure error
		minimumBackup, failure = f.boundary()
		if failure != nil {
			return failure
		}
		records, failure = f.records(ctx)
		return failure
	})
	return
}

func (f *Files) Records(ctx context.Context) ([]application.DeletionRecord, error) {
	records, _, err := f.Snapshot(ctx)
	return records, err
}

func (f *Files) Admit(ctx context.Context, record application.DeletionRecord) (accepted application.DeletionRecord, err error) {
	err = f.locked(ctx, func() error {
		if _, failure := f.boundary(); failure != nil {
			return failure
		}
		var failure error
		accepted, failure = f.admit(ctx, record)
		return failure
	})
	return
}

// Retire removes only aged records whose authoritative cleanup is complete.
// Advance the restore boundary before unlinking: a crash can retain redundant
// records, but cannot export missing evidence with an older permissive boundary.
func (f *Files) Retire(ctx context.Context, now time.Time, limit int, completed func(context.Context, application.DeletionRecord) (bool, error)) (removed int, err error) {
	if now.IsZero() || limit < 1 || limit > 1000 || completed == nil {
		return 0, ports.ErrInvalidArgument
	}
	err = f.locked(ctx, func() error {
		boundary, failure := f.boundary()
		if failure != nil {
			return failure
		}
		if boundary.After(now) {
			return ports.ErrUnavailable
		}
		records, failure := f.records(ctx)
		if failure != nil {
			return failure
		}
		var candidates []application.DeletionRecord
		for _, record := range records {
			if record.DeletedAt.After(now.Add(-30 * 24 * time.Hour)) {
				continue
			}
			done, failure := completed(ctx, record)
			if failure != nil {
				return failure
			}
			if done {
				candidates = append(candidates, record)
			}
			if len(candidates) == limit {
				break
			}
		}
		if len(candidates) == 0 {
			return nil
		}
		if failure = f.writeBoundary(now.UTC()); failure != nil {
			return failure
		}
		for _, record := range candidates {
			if failure = ctx.Err(); failure != nil {
				return failure
			}
			digest := sha256.Sum256([]byte(record.UserID))
			if failure = os.Remove(filepath.Join(f.directory, hex.EncodeToString(digest[:])+".sealed")); failure != nil {
				return failure
			}
			removed++
		}
		return f.syncDirectory()
	})
	return
}

func (f *Files) markInitialized() error {
	file, err := os.OpenFile(filepath.Join(f.directory, initializedFile), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	if err = file.Sync(); err != nil {
		return err
	}
	return f.syncDirectory()
}
