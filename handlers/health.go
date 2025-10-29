package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/vantaidinh04/z2api-go/types"
)

// HealthHandler handles health check requests
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	response := types.HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UnixMilli(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
