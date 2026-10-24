// Package logger builds the structured logger used across the application.
package logger

import (
	"log/slog"
	"os"
)

// New builds a JSON slog.Logger writing to stdout at the given level.
// An unknown level falls back to info.
func New(level string) *slog.Logger {
	var parsed slog.Level
	if err := parsed.UnmarshalText([]byte(level)); err != nil {
		parsed = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parsed}))
}
