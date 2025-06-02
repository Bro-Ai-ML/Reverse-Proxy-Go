package logger

import (
	"log/slog"
	"os"
)

func Setup(service string) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	if os.Getenv("LOG_LEVEL") == "debug" {
		opts.Level = slog.LevelDebug
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler).With(
		"service", service,
		"version", os.Getenv("VERSION"),
	)
	slog.SetDefault(logger)
}
