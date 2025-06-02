package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Config struct {
	Port            string
	ShutdownTimeout time.Duration
}

type Server struct {
	*http.Server
	ShutdownTimeout time.Duration
}

func New(cfg Config, handler http.Handler) *Server {
	return &Server{
		Server: &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		ShutdownTimeout: cfg.ShutdownTimeout,
	}
}

func (s *Server) Start() error {
	go func() {
		slog.Info("🚀 Server starting", "address", s.Addr)

		if err := s.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	slog.Info("⏹️ Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), s.ShutdownTimeout)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
		return err
	}

	slog.Info("✅ Server shutdown complete")

	return nil
}
