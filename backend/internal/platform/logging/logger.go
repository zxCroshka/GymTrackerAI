package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
)

// New creates the repository-standard structured JSON logger.
func New(output io.Writer, level, service, environment string) (*slog.Logger, error) {
	parsedLevel, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	return newLogger(output, parsedLevel, service, environment), nil
}

// Bootstrap creates a repository-formatted logger for failures that happen before configuration is available.
func Bootstrap(output io.Writer) *slog.Logger {
	return newLogger(output, slog.LevelInfo, "gymtracker-backend", "bootstrap")
}

func newLogger(output io.Writer, level slog.Level, service, environment string) *slog.Logger {
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, attribute slog.Attr) slog.Attr {
			if len(groups) != 0 {
				return attribute
			}

			switch attribute.Key {
			case slog.TimeKey:
				attribute.Key = "timestamp"
				attribute.Value = slog.StringValue(attribute.Value.Time().UTC().Format(time.RFC3339Nano))
			case slog.LevelKey:
				attribute.Value = slog.StringValue(strings.ToLower(attribute.Value.String()))
			case slog.MessageKey:
				attribute.Key = "message"
			}
			return attribute
		},
	})

	return slog.New(handler).With(
		slog.String("service", service),
		slog.String("environment", environment),
	)
}

func parseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level")
	}
}
