package dbmigrations

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
)

var inventoryLinePattern = regexp.MustCompile(
	`^([0-9a-f]{64})  (([0-9]{6})_([a-z0-9][a-z0-9_]*)\.(up|down)\.sql)$`,
)
var migrationFilePattern = regexp.MustCompile(
	`^([0-9]{6})_[a-z0-9][a-z0-9_]*\.(up|down)\.sql$`,
)

// VerifyMigrationInventory authenticates a complete, bounded migration release.
func VerifyMigrationInventory(files fs.FS, inventory []byte, firstVersion, lastVersion uint) error {
	if firstVersion == 0 || lastVersion < firstVersion {
		return fmt.Errorf("invalid migration inventory bounds %d-%d", firstVersion, lastVersion)
	}

	type migrationDirection struct {
		version   uint
		direction string
	}
	entries := make(map[migrationDirection]string)
	inventoriedNames := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(inventory))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		match := inventoryLinePattern.FindStringSubmatch(scanner.Text())
		if match == nil {
			return fmt.Errorf("malformed migration inventory line %d", lineNumber)
		}
		versionValue, err := strconv.ParseUint(match[3], 10, 32)
		if err != nil {
			return fmt.Errorf("parse migration inventory line %d: %w", lineNumber, err)
		}
		version := uint(versionValue)
		if version < firstVersion || version > lastVersion {
			return fmt.Errorf("migration inventory line %d has out-of-range version %d", lineNumber, version)
		}
		key := migrationDirection{version: version, direction: match[5]}
		if _, exists := entries[key]; exists {
			return fmt.Errorf("duplicate migration inventory entry for version %d %s", version, match[5])
		}

		contents, err := fs.ReadFile(files, match[2])
		if err != nil {
			return fmt.Errorf("read inventoried migration %q: %w", match[2], err)
		}
		digest := sha256.Sum256(contents)
		if hex.EncodeToString(digest[:]) != match[1] {
			return fmt.Errorf("inventoried migration %q has changed", match[2])
		}
		entries[key] = match[4]
		inventoriedNames[match[2]] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read migration inventory: %w", err)
	}

	for version := firstVersion; ; version++ {
		upName, hasUp := entries[migrationDirection{version: version, direction: "up"}]
		downName, hasDown := entries[migrationDirection{version: version, direction: "down"}]
		if !hasUp || !hasDown {
			return fmt.Errorf("migration inventory is incomplete at version %d", version)
		}
		if upName != downName {
			return fmt.Errorf("migration inventory version %d has mismatched up and down names", version)
		}
		if version == lastVersion {
			break
		}
	}

	directoryEntries, err := fs.ReadDir(files, ".")
	if err != nil {
		return fmt.Errorf("read migration directory: %w", err)
	}
	for _, entry := range directoryEntries {
		if entry.IsDir() {
			continue
		}
		match := migrationFilePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		versionValue, err := strconv.ParseUint(match[1], 10, 32)
		if err != nil {
			return fmt.Errorf("parse migration filename %q: %w", entry.Name(), err)
		}
		version := uint(versionValue)
		if version < firstVersion || version > lastVersion {
			continue
		}
		if _, exists := inventoriedNames[entry.Name()]; !exists {
			return fmt.Errorf("migration %q is absent from the inventory", entry.Name())
		}
	}
	return nil
}
