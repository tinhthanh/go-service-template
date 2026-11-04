// Package logging is the service's logger: log/slog, as JSON on stdout.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New is a JSON logger at the level named (debug, info, warn, error); an unknown name is info.
func New(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(strings.ToUpper(level))); err != nil {
		l = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}
