package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testLogRecorder struct {
	msgs []map[string]interface{}
}

func (t *testLogRecorder) Enabled(context.Context, slog.Level) bool { return true }
func (t *testLogRecorder) Handle(ctx context.Context, r slog.Record) error {
	m := map[string]interface{}{
		"level": r.Level,
		"msg":   r.Message,
	}
	r.Attrs(func(attr slog.Attr) bool {
		m[attr.Key] = attr.Value.Any()
		return true
	})
	t.msgs = append(t.msgs, m)
	return nil
}
func (t *testLogRecorder) WithAttrs(attrs []slog.Attr) slog.Handler { return t }
func (t *testLogRecorder) WithGroup(name string) slog.Handler      { return t }

func TestLoggingMiddleware(t *testing.T) {
	// Setup test logger
	logRecorder := &testLogRecorder{}
	logger := slog.New(logRecorder)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Ensure duration is non-zero
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler := LoggingMiddleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.GreaterOrEqual(t, len(logRecorder.msgs), 1)
	logMsg := logRecorder.msgs[0]
	assert.Equal(t, "request completed", logMsg["msg"])
	assert.Equal(t, "GET", logMsg["method"])
	assert.Equal(t, "/test", logMsg["path"])
	
	// Check duration is logged and greater than our sleep
	duration, ok := logMsg["duration"].(time.Duration)
	assert.True(t, ok, "duration should be a time.Duration")
	assert.Greater(t, duration, 10*time.Millisecond)
}
