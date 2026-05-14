package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gabiito/zdb/internal/config"
	"github.com/gabiito/zdb/internal/core"
	"github.com/gabiito/zdb/internal/logging"

	// Register all DB adapters via init()
	_ "github.com/gabiito/zdb/internal/db/mysql"
	_ "github.com/gabiito/zdb/internal/db/postgres"
	_ "github.com/gabiito/zdb/internal/db/sqlite"
)

// versionString returns the binary's version, sourced from Go's build
// info. For `go install ...@vX.Y.Z` it returns the tag; for local builds
// (`make build`, `./install.sh`) it returns "(devel)" plus the commit
// hash and a "modified" marker when the worktree was dirty.
func versionString() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "zdb (unknown)"
	}
	version := info.Main.Version
	if version == "" {
		version = "(unknown)"
	}
	var rev, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 7 {
				rev = s.Value[:7]
			} else {
				rev = s.Value
			}
		case "vcs.modified":
			if s.Value == "true" {
				modified = " · modified"
			}
		}
	}
	out := "zdb " + version
	if rev != "" {
		out += " · commit " + rev + modified
	}
	return out
}

func main() {
	for _, a := range os.Args[1:] {
		switch a {
		case "-v", "--version", "version":
			fmt.Println(versionString())
			return
		case "-h", "--help":
			fmt.Println("usage: zdb [--version]")
			fmt.Println("  zdb is interactive — no flags needed for normal use.")
			fmt.Println("  set ZDB_DEBUG=1 to enable debug logging.")
			fmt.Println("  set ZDB_CONFIG=/path/to/config.toml to override the config location.")
			return
		}
	}

	debug := os.Getenv("ZDB_DEBUG") == "1"

	log, err := logging.Init(debug)
	if err != nil {
		// Can't open log file — write to stderr before TUI starts
		fmt.Fprintf(os.Stderr, "zdb: logging init: %v\n", err)
		// Non-fatal: continue without logging
	}

	loaded, err := config.LoadOrEmpty()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	app := core.NewApp(loaded, log)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "zdb: %v\n", err)
		os.Exit(1)
	}
}
