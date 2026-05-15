// Package views persists user-named SQL "views" (saved queries) to a TOML
// file in the configuration directory used by the main config.
package views

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/gabiito/zdb/internal/config"
)

// viewsFilename is the fixed name of the views file inside each per-connection
// slug directory.
const viewsFilename = "views.toml"

// View is a single saved query.
type View struct {
	Name      string    `toml:"name"`
	SQL       string    `toml:"sql"`
	CreatedAt time.Time `toml:"created_at"`
}

// Store reads and writes saved views to disk for one connection. It is bound
// to a per-connection slug directory: <configDir>/views/<slug>/views.toml.
type Store struct {
	dir string // absolute path to <configDir>/views/<slug>/
}

// NewStoreForConnection resolves the per-connection slug directory for conn
// and returns a Store. Returns an error only when:
//   - the configDir cannot be resolved (e.g. HOME unset), or
//   - conn.Name normalises to an empty slug.
//
// Callers MUST guard with views.Slug(conn.Name) == "" before calling this
// function if they want a friendlier error; the check here is defensive.
func NewStoreForConnection(conn config.Connection) (*Store, error) {
	base, err := configDir()
	if err != nil {
		return nil, err
	}
	slug := Slug(conn.Name)
	if slug == "" {
		return nil, fmt.Errorf("views: connection name %q produces empty slug", conn.Name)
	}
	return &Store{dir: filepath.Join(base, "views", slug)}, nil
}

// Path returns the absolute path to the views.toml file inside this store's
// directory (for diagnostics / external callers).
func (s *Store) Path() string { return filepath.Join(s.dir, viewsFilename) }

// Dir returns the absolute path to the per-connection slug directory.
// Used by app.go for rename and delete hooks.
func (s *Store) Dir() string { return s.dir }

// Load returns the persisted views, sorted alphabetically. Returns nil, nil
// when the per-connection directory or the views.toml file does not yet exist
// (matches the "no views saved yet" expectation at the call site).
// A leftover .tmp file in the directory does NOT cause an error.
func (s *Store) Load() ([]View, error) {
	data, err := os.ReadFile(s.Path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var w wrapper
	if err := toml.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("views: parse %s: %w", s.Path(), err)
	}
	sort.SliceStable(w.Views, func(i, j int) bool {
		return w.Views[i].Name < w.Views[j].Name
	})
	return w.Views, nil
}

// renameFn is the rename hook used by Save. Tests may replace this variable
// to inject failures between the encode step and the final rename, simulating
// a mid-write crash (REQ-16 / SCN-14).
var renameFn = os.Rename

// Save writes views atomically to disk, replacing any prior views.toml.
// The per-connection directory is created lazily on first save (REQ-10).
// Write strategy: tempfile in the same directory → fsync → rename.
// If a crash occurs between the write and the rename, the prior views.toml
// remains intact; the leftover .tmp file does NOT block subsequent Load calls.
func (s *Store) Save(vs []View) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "views-*.tmp")
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			os.Remove(tmp.Name())
		}
	}()

	if err := toml.NewEncoder(tmp).Encode(wrapper{Views: vs}); err != nil {
		tmp.Close()
		return fmt.Errorf("views: encode: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := renameFn(tmp.Name(), s.Path()); err != nil {
		return fmt.Errorf("views: rename %s -> %s: %w", tmp.Name(), s.Path(), err)
	}
	committed = true
	return nil
}

// Add appends v to the stored views, replacing any entry with the same name.
// The timestamp is set to now when v.CreatedAt is zero.
func (s *Store) Add(v View) error {
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}
	existing, err := s.Load()
	if err != nil {
		return err
	}
	out := make([]View, 0, len(existing)+1)
	for _, e := range existing {
		if e.Name != v.Name {
			out = append(out, e)
		}
	}
	out = append(out, v)
	return s.Save(out)
}

// Remove deletes the view with the given name. Returns nil when no such view
// exists (idempotent).
func (s *Store) Remove(name string) error {
	existing, err := s.Load()
	if err != nil {
		return err
	}
	out := make([]View, 0, len(existing))
	for _, v := range existing {
		if v.Name != name {
			out = append(out, v)
		}
	}
	return s.Save(out)
}

type wrapper struct {
	Views []View `toml:"views"`
}

// configDir returns the absolute path of the zDB configuration directory.
// Resolution order: $ZDB_CONFIG (directory of the file) → $XDG_CONFIG_HOME/zdb
// → ~/.config/zdb.
func configDir() (string, error) {
	if p := os.Getenv("ZDB_CONFIG"); p != "" {
		return filepath.Dir(p), nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "zdb"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "zdb"), nil
}
