// Package version implements the `jughead version` subcommand.
package version

import (
	"fmt"

	"github.com/spf13/cobra"
)

// BuildInfo is set by ldflags at build time (see Taskfile build target).
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Cmd is the cobra command for `jughead version`.
var Cmd = &cobra.Command{
	Use:   "version",
	Short: "Print the jughead version",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("jughead %s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
	},
}
