package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gabiito/zdb/internal/config"
	"github.com/gabiito/zdb/internal/core"
	"github.com/gabiito/zdb/internal/logging"
	"github.com/gabiito/zdb/internal/views"

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
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "-v", "--version", "version":
			fmt.Println(versionString())
			return
		case "-h", "--help":
			printHelp()
			return
		case "config":
			os.Exit(runConfigCmd(os.Args[2:]))
		}
	}
	runTUI()
}

// printHelp prints usage information to stdout.
func printHelp() {
	fmt.Println("usage: zdb [--version] [config <subcommand>]")
	fmt.Println("  zdb is interactive — no flags needed for normal use.")
	fmt.Println("  zdb config import <path>  import a zdb config file.")
	fmt.Println("  set ZDB_DEBUG=1 to enable debug logging.")
	fmt.Println("  set ZDB_CONFIG=/path/to/config.toml to override the config location.")
}

// runTUI starts the interactive TUI. This is the default mode when no
// subcommand is given.
func runTUI() {
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

	// One-shot legacy migration: move the pre-per-connection views.toml aside.
	// This runs once per boot; subsequent boots where the file is absent are
	// no-ops. Non-fatal — if it fails we log to stderr and continue.
	movedTo, mErr := views.MigrateLegacyViews()
	if mErr != nil {
		fmt.Fprintf(os.Stderr, "zdb: legacy views migration failed: %v\n", mErr)
	}

	app := core.NewApp(loaded, log)
	if movedTo != "" {
		app.SetPendingStatusMsg(fmt.Sprintf(
			"legacy views.toml moved aside to %s — views are not carried forward; use Copy View From Connection to import individually.",
			movedTo,
		))
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "zdb: %v\n", err)
		os.Exit(1)
	}
}
