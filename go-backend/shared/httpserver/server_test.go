package httpserver

import (
	"log/slog"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockHandler is a simple http.Handler that records if it was called.
type mockHandler struct {
	called bool
}

func (m *mockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.called = true
	w.WriteHeader(http.StatusOK)
}

func TestNew(t *testing.T) {
	h := &mockHandler{}
	cfg := Config{
		Port:            "12345",
		ShutdownTimeout: 5 * time.Second,
	}
	s := New(cfg, h)
	assert.Equal(t, ":12345", s.Addr)
	assert.Equal(t, h, s.Handler)
	assert.Equal(t, 15*time.Second, s.ReadTimeout)
	assert.Equal(t, 15*time.Second, s.WriteTimeout)
	assert.Equal(t, 60*time.Second, s.IdleTimeout)
	assert.Equal(t, 5*time.Second, s.ShutdownTimeout)
}

func TestServer_Start_And_Shutdown(t *testing.T) {
	h := &mockHandler{}
	cfg := Config{
		Port:            "0", // Let the OS pick a free port
		ShutdownTimeout: 2 * time.Second,
	}
	s := New(cfg, h)

	// Replace slog default handler to avoid noisy output during test
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	defer slog.SetDefault(oldLogger)

	// Start the server in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- s.Start()
	}()

	// Wait for the server to start
	time.Sleep(100 * time.Millisecond)

	// Send a SIGINT to trigger shutdown
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGINT)

	// Wait for the server to shut down
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}
