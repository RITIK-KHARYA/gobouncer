package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ritik-kharya/gobouncer/internal/limiter"
)

type CheckRequest struct {
	Key 			string `json:"key" validate:"required"`
	Window 			int64 `json:"window" validate:"required"`
	Limit 			int64 `json:"limit" validate:"required"`
}

func makeCheckHandler (algo limiter.Algorithm) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req CheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		result := algo.Check(r.Context(), req.Key, req.Window, req.Limit)

		w.Header().Set("content-type", "application/json")
		if result.RetryAfter > 0 {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", result.RetryAfter))
		}
		json.NewEncoder(w).Encode(result)	
}}