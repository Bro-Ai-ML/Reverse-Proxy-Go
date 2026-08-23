package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type Service interface {
	HealthCheck() error
	GetInvoice(id string) (interface{}, error)
}

type Handler struct {
	service Service
	log     *zerolog.Logger
}

func New(svc Service, log *zerolog.Logger) *Handler {
	return &Handler{service: svc, log: log}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if err := h.service.HealthCheck(); err != nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/billing/invoices/")
	invoice, err := h.service.GetInvoice(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, invoice)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// notImplemented answers 501 for billing endpoints whose business logic has
// not been written yet. Returning a clear 501 (instead of not compiling, as
// before) keeps the API surface honest while the features are built.
func (h *Handler) notImplemented(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"error": "not implemented",
			"route": name,
		})
	}
}

func (h *Handler) ListInvoices(w http.ResponseWriter, r *http.Request)      { h.notImplemented("ListInvoices")(w, r) }
func (h *Handler) PayInvoice(w http.ResponseWriter, r *http.Request)        { h.notImplemented("PayInvoice")(w, r) }
func (h *Handler) CancelInvoice(w http.ResponseWriter, r *http.Request)     { h.notImplemented("CancelInvoice")(w, r) }
func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) { h.notImplemented("ListSubscriptions")(w, r) }
func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request)   { h.notImplemented("GetSubscription")(w, r) }
func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	h.notImplemented("CreateSubscription")(w, r)
}
func (h *Handler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	h.notImplemented("CancelSubscription")(w, r)
}
func (h *Handler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	h.notImplemented("UpdateSubscription")(w, r)
}
func (h *Handler) ListPaymentMethods(w http.ResponseWriter, r *http.Request) {
	h.notImplemented("ListPaymentMethods")(w, r)
}
func (h *Handler) GetPaymentMethod(w http.ResponseWriter, r *http.Request) {
	h.notImplemented("GetPaymentMethod")(w, r)
}
func (h *Handler) AddPaymentMethod(w http.ResponseWriter, r *http.Request) {
	h.notImplemented("AddPaymentMethod")(w, r)
}
func (h *Handler) RemovePaymentMethod(w http.ResponseWriter, r *http.Request) {
	h.notImplemented("RemovePaymentMethod")(w, r)
}
func (h *Handler) SetDefaultPaymentMethod(w http.ResponseWriter, r *http.Request) {
	h.notImplemented("SetDefaultPaymentMethod")(w, r)
}
func (h *Handler) RecordUsage(w http.ResponseWriter, r *http.Request)     { h.notImplemented("RecordUsage")(w, r) }
func (h *Handler) GetUsageSummary(w http.ResponseWriter, r *http.Request) { h.notImplemented("GetUsageSummary")(w, r) }

// AuthMiddleware protects billing routes with an HS256 JWT check using the
// shared AUTH_JWT_SECRET. It fails CLOSED: without a configured secret every
// protected request is rejected with 503 — the previous code had no
// middleware at all. (Deliberately dependency-free.)
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	secret := os.Getenv("AUTH_JWT_SECRET")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret == "" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "authentication is not configured (AUTH_JWT_SECRET missing)",
			})
			return
		}

		authHeader := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(authHeader) <= len(prefix) || !strings.EqualFold(authHeader[:len(prefix)], prefix) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}
		if !validHS256(strings.TrimSpace(authHeader[len(prefix):]), []byte(secret)) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// validHS256 verifies an HS256 JWT: signature first (constant time), then
// exp/nbf claims. Minimal and dependency-free on purpose.
func validHS256(token string, secret []byte) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return false
	}
	if header.Alg != "HS256" {
		return false // refuse alg confusion/none attacks
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return false
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var claims struct {
		Exp *int64 `json:"exp"`
		Nbf *int64 `json:"nbf"`
	}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return false
	}
	now := time.Now().Unix()
	if claims.Exp != nil && now >= *claims.Exp {
		return false
	}
	if claims.Nbf != nil && now < *claims.Nbf {
		return false
	}
	return true
}
