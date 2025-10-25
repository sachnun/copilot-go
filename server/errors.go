package server

import (
	"encoding/json"
	"net/http"

	appErr "internal/errors"
	"internal/logger"
)

type errorResponse struct {
	Error any `json:"error"`
}

func writeError(w http.ResponseWriter, err error) {
	logger.Error("Request failed: %v", err)
	if httpErr, ok := err.(*appErr.HTTPError); ok {
		for key, values := range httpErr.Response.Header {
			for _, v := range values {
				w.Header().Add(key, v)
			}
		}
		if len(httpErr.Body) > 0 {
			w.WriteHeader(httpErr.Response.StatusCode)
			_, _ = w.Write(httpErr.Body)
			return
		}
		w.WriteHeader(httpErr.Response.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	payload, _ := json.Marshal(errorResponse{Error: map[string]any{
		"message": err.Error(),
		"type":    "error",
	}})
	_, _ = w.Write(payload)
}
