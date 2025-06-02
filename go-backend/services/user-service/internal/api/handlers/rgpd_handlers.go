package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"stripe-demo/services/user-service/internal/model"
	"stripe-demo/services/user-service/internal/service"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

type RGPDHandlers struct {
	rgpdSvc service.RGPDServices
}

func NewRGPDHandlers(rgpdSvc service.RGPDServices) *RGPDHandlers {
	return &RGPDHandlers{
		rgpdSvc: rgpdSvc,
	}
}

// RegisterRoutes enregistre les routes RGPD
func (h *RGPDHandlers) RegisterRoutes(router *mux.Router, authMiddleware mux.MiddlewareFunc) {
	rgpdRouter := router.PathPrefix("/rgpd").Subrouter()
	rgpdRouter.Use(authMiddleware)

	rgpdRouter.HandleFunc("/export", h.exportUserData).Methods("GET")
	rgpdRouter.HandleFunc("/consent", h.updateConsent).Methods("PUT")
	rgpdRouter.HandleFunc("/delete-account", h.deleteAccount).Methods("POST")
}

// exportUserData exporte toutes les données personnelles de l'utilisateur
func (h *RGPDHandlers) exportUserData(w http.ResponseWriter, r *http.Request) {
	// Récupérer l'ID utilisateur du contexte (ajouté par le middleware d'authentification)
	userID, ok := r.Context().Value("user_id").(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Exporter les données
	data, err := h.rgpdSvc.ExportUserData(r.Context(), userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to export user data")
		http.Error(w, "Failed to export user data", http.StatusInternalServerError)
		return
	}

	// Renvoyer les données en JSON
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="user-data-`+time.Now().Format("20060102")+`.json"`)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// updateConsent met à jour les préférences de consentement de l'utilisateur
func (h *RGPDHandlers) updateConsent(w http.ResponseWriter, r *http.Request) {
	// Récupérer l'ID utilisateur du contexte
	userID, ok := r.Context().Value("user_id").(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Décoder la requête
	var consent model.ConsentUpdate
	if err := json.NewDecoder(r.Body).Decode(&consent); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Mettre à jour le consentement
	if err := h.rgpdSvc.UpdateConsent(r.Context(), userID, &consent); err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to update user consent")
		http.Error(w, "Failed to update consent", http.StatusInternalServerError)
		return
	}

	// Répondre avec succès
	w.WriteHeader(http.StatusNoContent)
}

// deleteAccount supprime le compte utilisateur et anonymise les données
func (h *RGPDHandlers) deleteAccount(w http.ResponseWriter, r *http.Request) {
	// Récupérer l'ID utilisateur du contexte
	userID, ok := r.Context().Value("user_id").(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Anonymiser l'utilisateur
	if err := h.rgpdSvc.AnonymizeUser(r.Context(), userID); err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to delete user account")
		http.Error(w, "Failed to delete account", http.StatusInternalServerError)
		return
	}

	// Répondre avec succès
	w.WriteHeader(http.StatusNoContent)
}
