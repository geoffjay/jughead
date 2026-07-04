package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/geoffjay/jughead/cmd/jughead/serve"
	"github.com/geoffjay/jughead/cmd/jughead/version"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

// rootCmd is the top-level jughead CLI. When invoked with no subcommand it
// falls through to runServer (the full multi-tenant application server),
// preserving the prior "go run ." workflow.
var rootCmd = &cobra.Command{
	Use:   "jughead",
	Short: "Jughead — multi-tenant site platform and plugin runner",
	Long: `Jughead is a multi-tenant site platform with a plugin architecture for
external site development.

Without a subcommand, it starts the full application server (admin UI, database,
auth, built-in sites, and plugins from JUGHEAD_PLUGINS_DIR).

The 'serve' subcommand loads a single site plugin (.so) and runs it standalone,
which is useful for site developers who want to iterate without running the
entire jughead system.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return runServer()
	},
}

func main() {
	// Load .env if present. Missing file is fine (e.g. production where env
	// vars are set by the platform); a parse error is fatal since it usually
	// means a malformed secret.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		slog.Error("Failed to load .env", "details", err.Error())
		os.Exit(1)
	}

	rootCmd.AddCommand(serve.Cmd)
	rootCmd.AddCommand(version.Cmd)
	serve.SetupFlags()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
