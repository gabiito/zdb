package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gabiito/zdb/internal/config"
	"github.com/gabiito/zdb/internal/core"
	"github.com/gabiito/zdb/internal/logging"

	// Register all DB adapters via init()
	_ "github.com/gabiito/zdb/internal/db/mysql"
	_ "github.com/gabiito/zdb/internal/db/postgres"
	_ "github.com/gabiito/zdb/internal/db/sqlite"
)

func main() {
	debug := os.Getenv("ZDB_DEBUG") == "1"

	log, err := logging.Init(debug)
	if err != nil {
		// Can't open log file — write to stderr before TUI starts
		fmt.Fprintf(os.Stderr, "zdb: logging init: %v\n", err)
		// Non-fatal: continue without logging
	}

	cfg, err := config.LoadOrEmpty()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	app := core.NewApp(cfg, log)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "zdb: %v\n", err)
		os.Exit(1)
	}
}
