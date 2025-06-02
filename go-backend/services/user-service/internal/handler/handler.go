package handler

import (
	"github.com/gorilla/mux"
)

func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/health", HealthHandler).Methods("GET")
}
