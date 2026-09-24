package dbmigrations

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

var canonicalMigrationName = regexp.MustCompile(`^(\d{6})_([a-z0-9][a-z0-9_]*)\.(up|down)\.sql$`)

func TestEmbeddedMigrationsContainExactlyOneMatchingPairPerVersion(t *testing.T) {
	if err := verifyEmbeddedMigrations(Files, LatestVersion); err != nil {
		t.Fatal(err)
	}
}

func TestEmbeddedMigrationInvariantRejectsAliasesParsedAsDuplicateVersions(t *testing.T) {
	for _, name := range []string{
		"1_shadow.up.sql",
		"0000001_shadow.up.sql",
	} {
		t.Run(name, func(t *testing.T) {
			files := migrationPairFiles(1)
			files[name] = &fstest.MapFile{Data: []byte("SELECT 1;\n")}

			_, parserErr := iofs.New(files, ".")
			var duplicate source.ErrDuplicateMigration
			if !errors.As(parserErr, &duplicate) || duplicate.Migration.Version != 1 {
				t.Fatalf("iofs.New() error = %v, want duplicate migration version 1", parserErr)
			}

			err := verifyEmbeddedMigrations(files, 1)
			if err == nil || !strings.Contains(err.Error(), "non-canonical migration filename") {
				t.Fatalf("verifyEmbeddedMigrations() error = %v, want non-canonical filename error", err)
			}
		})
	}
}

func verifyEmbeddedMigrations(files fs.FS, latestVersion uint) error {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return err
	}
	type migrationPair struct {
		upName   string
		downName string
	}
	pairs := make(map[uint]migrationPair)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := canonicalMigrationName.FindStringSubmatch(entry.Name())
		if len(match) != 4 {
			if strings.HasSuffix(entry.Name(), ".sql") {
				return fmt.Errorf("non-canonical migration filename %q", entry.Name())
			}
			continue
		}
		versionValue, err := strconv.ParseUint(match[1], 10, 64)
		if err != nil {
			return err
		}
		version := uint(versionValue)
		if version == 0 || version > latestVersion {
			return fmt.Errorf("migration %q has version %d outside 1..%d", entry.Name(), version, latestVersion)
		}
		pair := pairs[version]
		if match[3] == "up" {
			if pair.upName != "" {
				return fmt.Errorf("migration version %d has duplicate up files %q and %q", version, pair.upName, entry.Name())
			}
			pair.upName = match[2]
		} else {
			if pair.downName != "" {
				return fmt.Errorf("migration version %d has duplicate down files %q and %q", version, pair.downName, entry.Name())
			}
			pair.downName = match[2]
		}
		pairs[version] = pair
	}
	for version := uint(1); version <= latestVersion; version++ {
		pair := pairs[version]
		if pair.upName == "" || pair.downName == "" {
			return fmt.Errorf("migration version %d does not have exactly one up/down pair", version)
		}
		if pair.upName != pair.downName {
			return fmt.Errorf("migration version %d has mismatched up/down names %q and %q", version, pair.upName, pair.downName)
		}
	}
	return nil
}

func migrationPairFiles(latestVersion uint) fstest.MapFS {
	files := fstest.MapFS{}
	for version := uint(1); version <= latestVersion; version++ {
		for _, direction := range []string{"down", "up"} {
			name := fmt.Sprintf("%06d_fixture.%s.sql", version, direction)
			files[name] = &fstest.MapFile{Data: []byte(name + " contents\n")}
		}
	}
	return files
}
