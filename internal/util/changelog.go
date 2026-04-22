package util

import "fmt"

type ChangelogEntry struct {
	Version     string
	Breaking    bool
	Title       string
	Changes     []string
	MigrationFn func() error
}

func FilterPendingEntries(lastVersion string, entries []ChangelogEntry) []ChangelogEntry {
	var pending []ChangelogEntry
	for _, entry := range entries {
		if IsNewerVersion(lastVersion, entry.Version) {
			pending = append(pending, entry)
		}
	}
	return pending
}

func RunMigrations(entries []ChangelogEntry) error {
	for _, e := range entries {
		if e.MigrationFn == nil {
			continue
		}
		if err := e.MigrationFn(); err != nil {
			return fmt.Errorf("migration for v%s: %w", e.Version, err)
		}
	}
	return nil
}
