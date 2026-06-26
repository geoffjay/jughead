package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env if present. Missing file is fine (e.g. production where env
	// vars are set by the platform); a parse error is fatal since it usually
	// means a malformed secret.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		slog.Error("Failed to load .env", "details", err.Error())
		os.Exit(1)
	}

	if err := runServer(); err != nil {
		slog.Error("Failed to start server!", "details", err.Error())
		os.Exit(1)
	}
}
