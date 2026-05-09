package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stdout, "db-viewer: use --help")
	os.Exit(0)
}
