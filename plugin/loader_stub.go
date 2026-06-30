//go:build !linux && !darwin

// Package plugin is a no-op stub on platforms that don't support the Go
// `plugin` package (e.g. windows). LoadAll logs a warning and returns nil so
// server.go can call it unconditionally; runtime plugins are simply
// unavailable on these platforms. Built-in providers and in-repo sites remain
// fully functional.
package plugin

import "log/slog"

// LoadAll is a no-op on unsupported platforms.
func LoadAll(dir string, strict bool) error {
	if dir != "" {
		slog.Warn("plugin loader: runtime plugins are not supported on this platform; ignoring JUGHEAD_PLUGINS_DIR",
			"dir", dir)
	}
	return nil
}
