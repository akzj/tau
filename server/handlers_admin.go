package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleAdminKeys handles GET and POST on /v1/admin/keys.
func (s *Server) handleAdminKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		keys := s.keyStore.ListKeys()
		writeJSON(w, 200, map[string]any{"keys": keys, "count": len(keys)})

	case http.MethodPost:
		var req struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		key := req.Key
		if key == "" {
			key = generateKey()
		}
		ak := s.keyStore.AddKey(key)
		writeJSON(w, 201, map[string]any{
			"key":        ak.Key,
			"masked":     ak.Masked,
			"created_at": ak.CreatedAt,
		})

	default:
		http.Error(w, "method not allowed", 405)
	}
}

// handleAdminKeyByID handles DELETE on /v1/admin/keys/{masked-or-key}.
func (s *Server) handleAdminKeyByID(w http.ResponseWriter, r *http.Request) {
	// Extract the key identifier from URL path
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/admin/keys/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "key identifier required", 400)
		return
	}
	maskedOrKey := parts[0]

	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", 405)
		return
	}

	// Try exact key match first
	if s.keyStore.DeleteKey(maskedOrKey) {
		writeJSON(w, 200, map[string]string{"deleted": maskKey(maskedOrKey)})
		return
	}

	// Try masked match
	for _, k := range s.keyStore.ListKeys() {
		if k.Masked == maskedOrKey {
			s.keyStore.DeleteKey(k.Key)
			writeJSON(w, 200, map[string]string{"deleted": maskedOrKey})
			return
		}
	}

	writeJSON(w, 404, map[string]string{"error": "key not found"})
}