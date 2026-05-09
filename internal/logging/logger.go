// Package logging provides structured logging for db-viewer.
// It writes JSON to a file under $XDG_STATE_HOME/dbviewer/ and never to stderr,
// since Bubbletea owns the terminal after tea.NewProgram starts.
package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// Init initializes the global logger, writing JSON to the log file.
// If debug is true, the log level is set to Debug; otherwise Info.
// Returns the logger and any error encountered while setting up the log file.
// Fatal startup errors must be handled by the caller.
func Init(debug bool) (*slog.Logger, error) {
	logPath, err := logFilePath()
	if err != nil {
		return nil, fmt.Errorf("logging: resolve log path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return nil, fmt.Errorf("logging: create log dir: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("logging: open log file: %w", err)
	}

	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	return logger, nil
}

// logFilePath returns the path to the log file, following XDG conventions.
func logFilePath() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "dbviewer", "log"), nil
}
