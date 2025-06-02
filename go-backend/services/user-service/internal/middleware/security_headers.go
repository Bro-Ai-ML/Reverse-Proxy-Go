package middleware

import "net/http"

// SecurityHeadersMiddleware ajoute des en-têtes de sécurité HTTP
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Politique de sécurité du contenu (CSP)
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data:; " +
			"font-src 'self'; " +
			"connect-src 'self'; " +
			"frame-ancestors 'none'; " +
			"form-action 'self'; " +
			"base-uri 'self'; " +
			"block-all-mixed-content;"

		// Définir les en-têtes de sécurité
		headers := map[string]string{
			// Sécurité
			"Content-Security-Policy":   csp,
			"X-Content-Type-Options":    "nosniff",
			"X-Frame-Options":           "DENY",
			"X-XSS-Protection":          "1; mode=block",
			"Referrer-Policy":           "strict-origin-when-cross-origin",
			"Permissions-Policy":        "geolocation=(), microphone=(), camera=()",
			"Strict-Transport-Security": "max-age=31536000; includeSubDomains; preload",
		}

		// Ajouter les en-têtes à la réponse
		for key, value := range headers {
			w.Header().Set(key, value)
		}

		// Appeler le prochain handler
		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware gère les en-têtes CORS
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false

			// Vérifier si l'origine est autorisée
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Répondre immédiatement aux requêtes OPTIONS
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
