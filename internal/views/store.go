// Package views persists user-named SQL "views" (saved queries) to a TOML
// file in the same configuration directory used by the main config.
package views

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/BurntSushi/toml"
)

// View is a single saved query.
type View struct {
	Name      string    `toml:"name"`
	SQL       string    `toml:"sql"`
	CreatedAt time.Time `toml:"created_at"`
}

// Store reads and writes saved views to disk.
type Store struct {
	path string
}

// NewStore resolves the views file path under the zdb config directory.
// The directory is created on first save if it doesn't exist.
func NewStore() (*Store, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	return &Store{path: filepath.Join(dir, "views.toml")}, nil
}

// Path returns the absolute path to the views file (for diagnostics).
func (s *Store) Path() string { return s.path }

// Load returns the persisted views, sorted alphabetically. Returns an empty
// slice if the file doesn't exist yet.
func (s *Store) Load() ([]View, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var w wrapper
	if err := toml.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("views: parse %s: %w", s.path, err)
	}
	sort.SliceStable(w.Views, func(i, j int) bool {
		return w.Views[i].Name < w.Views[j].Name
	})
	return w.Views, nil
}

// Save writes the views to disk, replacing the file. Creates the parent
// directory if missing.
func (s *Store) Save(views []View) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	return enc.Encode(wrapper{Views: views})
}

// Add appends a view (replacing any existing entry with the same name).
func (s *Store) Add(v View) error {
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}
	views, err := s.Load()
	if err != nil {
		return err
	}
	out := make([]View, 0, len(views)+1)
	for _, existing := range views {
		if existing.Name != v.Name {
			out = append(out, existing)
		}
	}
	out = append(out, v)
	return s.Save(out)
}

// Remove deletes the view with the given name. Returns nil even if no such
// view existed (idempotent).
func (s *Store) Remove(name string) error {
	views, err := s.Load()
	if err != nil {
		return err
	}
	out := make([]View, 0, len(views))
	for _, v := range views {
		if v.Name != name {
			out = append(out, v)
		}
	}
	return s.Save(out)
}

type wrapper struct {
	Views []View `toml:"views"`
}

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
