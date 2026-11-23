package logger

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

func New(level string, output io.Writer) (*slog.Logger, error) {
	var slogLevel slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		return nil, fmt.Errorf("unsupported log level %q", level)
	}
	if output == nil {
		return nil, fmt.Errorf("logger output is required")
	}
	return slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: slogLevel})), nil
}
