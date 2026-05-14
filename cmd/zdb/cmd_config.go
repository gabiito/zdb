package main

import (
	"fmt"
	"os"

	"github.com/gabiito/zdb/internal/config"
)

// runConfigCmd dispatches the "zdb config <subcommand>" family. It returns an
// exit code: 0 for success, 1 for a subcommand error, 2 for a usage error.
func runConfigCmd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: zdb config <subcommand>")
		fmt.Fprintln(os.Stderr, "  zdb config import <path>")
		return 2
	}
	switch args[0] {
	case "import":
		return runConfigImport(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "zdb config: unknown subcommand %q\n", args[0])
		fmt.Fprintln(os.Stderr, "available subcommands: import")
		return 2
	}
}

// runConfigImport implements "zdb config import <path>". It returns an exit
// code: 0 on success, 1 on import error, 2 on usage error.
func runConfigImport(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: zdb config import <path>")
		return 2
	}
	srcPath := args[0]

	dstPath, err := config.ResolveDefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "zdb: import failed: resolve destination: %v\n", err)
		return 1
	}

	if err := config.Import(srcPath, dstPath); err != nil {
		fmt.Fprintf(os.Stderr, "zdb: import failed: %v\n", err)
		return 1
	}

	fmt.Printf("zdb: imported config from %s to %s (version %d)\n", srcPath, dstPath, config.CurrentSchemaVersion)
	return 0
}
