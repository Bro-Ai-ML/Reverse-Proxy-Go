package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// LoggingMiddleware crée un middleware de journalisation conforme au RGPD
func LoggingMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Créer un enregistreur de requête avec des champs de base
			reqLogger := logger.With().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Logger()

			// Ajouter l'ID de la requête si disponible
			if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
				reqLogger = reqLogger.With().Str("request_id", requestID).Logger()
			}

			// Ajouter l'ID utilisateur si authentifié
			if userID, ok := r.Context().Value("user_id").(string); ok && userID != "" {
				reqLogger = reqLogger.With().Str("user_id", userID).Logger()
			}

			// Envelopper le writer pour capturer le statut de la réponse
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			// Appeler le prochain handler
			next.ServeHTTP(wrapped, r)

			// Journaliser les détails de la requête/réponse
			duration := time.Since(start)
			if wrapped.status >= 400 {
				reqLogger.Error().Int("status", wrapped.status).
					Int64("duration_ms", duration.Milliseconds()).
					Int64("response_size", wrapped.size).
					Msg("Request completed with error")
			} else {
				reqLogger.Info().Int("status", wrapped.status).
					Int64("duration_ms", duration.Milliseconds()).
					Int64("response_size", wrapped.size).
					Msg("Request completed")
			}
		})
	}
}

// responseWriter enveloppe http.ResponseWriter pour capturer le statut et la taille de la réponse
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += int64(size)
	return size, err
}

// RGPDLoggingFilter filtre les données sensibles dans les logs
func RGPDLoggingFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Filtrer les en-têtes sensibles
		r.Header.Del("Authorization")
		r.Header.Del("Cookie")
		r.Header.Del("Set-Cookie")
		r.Header.Del("X-Csrf-Token")

		// Appeler le prochain handler
		next.ServeHTTP(w, r)
	})
}
