// Package version implements the `jughead version` subcommand.
//
// The version, commit, and build date are resolved in priority order:
//
//  1. ldflags-injected values (set by `task build` / `task build:plugins` for
//     local development builds). These always win when set to a non-default
//     value.
//  2. runtime/debug.ReadBuildInfo, which captures the module version the
//     binary was built from. For a binary installed via
//     `go install github.com/geoffjay/jughead@vX.Y.Z`, Main.Version is the
//     tagged version and the VCS build settings (vcs.revision, vcs.time) are
//     embedded when the binary is built from a working git checkout (e.g.
//     `go build .` inside the repo).
//  3. The defaults ("dev" / "unknown"), used when neither ldflags nor build
//     info supply a value.
package version

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// ldflags-injected defaults (overridden by `task build` and friends). When
// the ldflags value is the default, the version command falls back to
// runtime/debug.ReadBuildInfo so a `go install <module>@<version>` binary
// reports its actual module version.
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
		v, commit, date := resolve()
		fmt.Printf("jughead %s (commit: %s, built: %s)\n", v, commit, date)
	},
}

// resolve returns the (version, commit, buildDate) triple, preferring
// ldflags-injected values and falling back to runtime/debug.ReadBuildInfo.
func resolve() (version, commit, buildDate string) {
	version, commit, buildDate = Version, Commit, BuildDate

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		// Build info unavailable (e.g. some stripped builds). Fall back to
		// whatever ldflags supplied; if those were defaults too, the user
		// sees "dev"/"unknown", which is honest.
		return version, commit, buildDate
	}

	// Main.Version is the tagged version for a `go install @vX.Y.Z` build,
	// or "(devel)" for a `go build .` inside a checkout with no tag pinned.
	// Only override the ldflags default ("dev"); never clobber an explicit
	// ldflags-injected version (e.g. a `task build` stamp).
	if version == "dev" {
		if mv := bi.Main.Version; mv != "" && mv != "(devel)" {
			version = mv
		}
	}

	// VCS settings are embedded when the binary is built from a git checkout
	// with -buildvcs (the default for `go build .` in a repo). A proxy-
	// installed binary has no VCS info (the proxy ships a stripped tarball),
	// so commit/date fall back to "unknown" there — that's expected.
	settings := bi.Settings
	if commit == "unknown" {
		commit = buildSetting(settings, "vcs.revision", "unknown")
	}
	if buildDate == "unknown" {
		buildDate = buildSetting(settings, "vcs.time", "unknown")
	}

	return version, commit, buildDate
}

// buildSetting looks up a key in debug.BuildInfo.Settings, returning def when
// the key is absent or empty.
func buildSetting(settings []debug.BuildSetting, key, def string) string {
	for _, s := range settings {
		if s.Key == key && s.Value != "" {
			return s.Value
		}
	}
	return def
}
