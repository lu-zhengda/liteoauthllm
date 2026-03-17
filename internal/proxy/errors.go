package proxy

import (
	"encoding/json"
	"net/http"
)

func writeOpenAIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    "auth_error",
			"code":    status,
		},
	})
}

func writeAnthropicError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    "authentication_error",
			"message": message,
		},
	})
}

func writeAuthError(w http.ResponseWriter, providerName string, message string) {
	switch providerName {
	case "anthropic":
		writeAnthropicError(w, http.StatusUnauthorized, message)
	default:
		writeOpenAIError(w, http.StatusUnauthorized, message)
	}
}
