package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Tyler-Dinh/z2api-go/services"
	"github.com/Tyler-Dinh/z2api-go/types"
)

// ModelsHandler handles model listing requests
func ModelsHandler(w http.ResponseWriter, r *http.Request) {
	// Handle OPTIONS for CORS preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only allow GET and POST
	if r.Method != "GET" && r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get models from service
	modelsService := services.GetModelsService()
	models, err := modelsService.GetModels()
	if err != nil {
		log.Printf("Error fetching models: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch models: "+err.Error())
		return
	}

	// Return models
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models)
}

// writeError writes an error response
func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(types.ErrorResponse{
		Error:   statusCode,
		Message: message,
	})
}
