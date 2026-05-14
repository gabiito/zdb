package config

import "fmt"

// CurrentSchemaVersion is the version this build of zDB writes and
// understands. Bump this constant and append a migration to the
// migrations slice whenever the on-disk schema changes.
const CurrentSchemaVersion = 1

// migration is one step in the forward-only schema lineage. Run is
// expected to be idempotent: running it twice on the same input must
// produce the same output as running it once. Migrations mutate m in
// place and return an error only on truly malformed inputs.
type migration struct {
	toVersion int
	run       func(m map[string]any) error
}

// migrations is the ordered chain. v1 release ships empty.
// First real migration example:
//
//	var migrations = []migration{
//	    {toVersion: 2, run: migrateV1ToV2},
//	}
var migrations = []migration{}

// migrationsFn is a test seam: production code reads migrations
// through this indirection so _test.go files can swap in a synthetic
// chain without touching production state. Mirrors backupCurrentFn.
var migrationsFn = func() []migration { return migrations }

// currentSchemaVersionFn is a test seam: production code reads
// CurrentSchemaVersion through this indirection so _test.go files
// can override the perceived current version without mutating the
// constant. Mirrors migrationsFn.
var currentSchemaVersionFn = func() int { return CurrentSchemaVersion }

// SetMigrationsForTest replaces the live registry for the duration of a
// test and returns a restore function. Call defer restore() in the test.
// Mirrors SetBackupCurrentForTest.
func SetMigrationsForTest(fake []migration) (restore func()) {
	prev := migrationsFn
	migrationsFn = func() []migration { return fake }
	return func() { migrationsFn = prev }
}

// SetCurrentSchemaVersionForTest overrides the perceived current schema
// version for the duration of a test and returns a restore function.
// Call defer restore() in the test. The const CurrentSchemaVersion is
// never mutated; this seam wraps it behind a function variable.
func SetCurrentSchemaVersionForTest(v int) (restore func()) {
	prev := currentSchemaVersionFn
	currentSchemaVersionFn = func() int { return v }
	return func() { currentSchemaVersionFn = prev }
}

// readVersion extracts the version integer from a raw TOML map.
// If the key is absent or not an integer, it returns currentSchemaVersionFn()
// (per Decision 1: missing == current).
func readVersion(rawMap map[string]any) int {
	v, ok := rawMap["version"]
	if !ok {
		return currentSchemaVersionFn()
	}
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	default:
		return currentSchemaVersionFn()
	}
}

// runMigrations advances rawMap from fileVersion to currentSchemaVersionFn()
// by invoking every registered migration whose toVersion is strictly greater
// than the current cursor, in slice order. After each step the cursor
// advances and rawMap["version"] is updated.
//
// Returns the new cursor (always == currentSchemaVersionFn() on success)
// and nil on success. On error returns the cursor value at the point of
// failure and a wrapped error identifying the failing step.
func runMigrations(rawMap map[string]any, fileVersion int) (int, error) {
	cursor := fileVersion
	target := currentSchemaVersionFn()
	for _, m := range migrationsFn() {
		if m.toVersion <= cursor {
			continue
		}
		if m.toVersion > target {
			// Defensive: the registry is malformed — a maintainer appended a
			// future migration without bumping the constant. Refuse rather than
			// half-migrate.
			return cursor, fmt.Errorf(
				"zdb: migration chain inconsistent: migration to v%d exceeds CurrentSchemaVersion %d",
				m.toVersion, target,
			)
		}
		if err := m.run(rawMap); err != nil {
			return cursor, fmt.Errorf("zdb: migration to v%d: %w", m.toVersion, err)
		}
		cursor = m.toVersion
		rawMap["version"] = int64(cursor)
	}
	return cursor, nil
}
