package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/akzj/tau/core"
)

// APIKeyStore manages API keys with CRUD and rate limiting.
type APIKeyStore struct {
	mu        sync.RWMutex
	keys      map[string]*APIKey
	limiters  map[string]*core.RateLimiter
	adminKey  string
	devMode   bool
	rateLimit int // requests per minute equivalent (used as per-second)
}

// APIKey represents a managed API key.
type APIKey struct {
	Key       string    `json:"key"`
	Masked    string    `json:"masked"`
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used,omitempty"`
	Calls     int64     `json:"calls"`
}

// NewAPIKeyStore creates a key store from environment variables.
func NewAPIKeyStore() *APIKeyStore {
	s := &APIKeyStore{
		keys:     make(map[string]*APIKey),
		limiters: make(map[string]*core.RateLimiter),
		devMode:  true,
	}

	// Rate limit — must be parsed before AddKey so limiters use the right value.
	s.rateLimit = 30
	if rl := os.Getenv("TAU_RATE_LIMIT"); rl != "" {
		val := strings.TrimSuffix(rl, "/min")
		fmt.Sscanf(strings.TrimSpace(val), "%d", &s.rateLimit)
	}
	if s.rateLimit < 1 {
		s.rateLimit = 1
	}

	// Admin key
	if ak := os.Getenv("TAU_ADMIN_KEY"); ak != "" {
		s.adminKey = ak
	} else {
		s.adminKey = generateKey()
		fmt.Fprintf(os.Stderr, "tau serve: TAU_ADMIN_KEY not set — generated: %s\n", s.adminKey)
	}

	// API keys from env — AddKey uses s.rateLimit parsed above.
	if keysEnv := os.Getenv("TAU_API_KEYS"); keysEnv != "" {
		s.devMode = false
		for _, k := range strings.Split(keysEnv, ",") {
			k = strings.TrimSpace(k)
			if k != "" {
				s.AddKey(k)
			}
		}
	}

	return s
}

// AddKey adds a new API key to the store.
func (s *APIKeyStore) AddKey(key string) *APIKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	ak := &APIKey{
		Key:       key,
		Masked:    maskKey(key),
		CreatedAt: time.Now(),
	}
	s.keys[key] = ak
	s.limiters[key] = core.NewRateLimiter(s.rateLimit, s.rateLimit)
	return ak
}

// Validate checks if a key is valid. Returns the APIKey and true if valid.
func (s *APIKeyStore) Validate(key string) (*APIKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ak, ok := s.keys[key]
	if !ok && s.devMode {
		return &APIKey{Key: key, Masked: "dev"}, true
	}
	if ok {
		ak.LastUsed = time.Now()
		ak.Calls++
	}
	return ak, ok
}

// GetRateLimiter returns the rate limiter for a given key.
func (s *APIKeyStore) GetRateLimiter(key string) *core.RateLimiter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.limiters[key]
}

// DeleteKey removes a key from the store. Returns true if the key existed.
func (s *APIKeyStore) DeleteKey(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.keys[key]
	if ok {
		delete(s.keys, key)
		delete(s.limiters, key)
	}
	return ok
}

// ListKeys returns a copy of all managed API keys.
func (s *APIKeyStore) ListKeys() []APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]APIKey, 0, len(s.keys))
	for _, k := range s.keys {
		out = append(out, *k)
	}
	return out
}

// IsDevMode returns true if no TAU_API_KEYS were set.
func (s *APIKeyStore) IsDevMode() bool { return s.devMode }

// AdminKey returns the admin key.
func (s *APIKeyStore) AdminKey() string { return s.adminKey }

// maskKey returns a masked version: first 4 + ... + last 4.
func maskKey(key string) string {
	if len(key) < 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// generateKey creates a random API key with "tau-" prefix.
func generateKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	h := sha256.Sum256(b)
	return "tau-" + hex.EncodeToString(h[:])[:20]
}

// --- Middleware ---

// AuthMiddleware validates API keys and applies rate limiting.
func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always bypass auth for health
		if r.URL.Path == "/v1/health" {
			next.ServeHTTP(w, r)
			return
		}

		// OpenAPI spec is public
		if r.URL.Path == "/v1/openapi.json" {
			next.ServeHTTP(w, r)
			return
		}

		// Admin endpoints require admin key
		if strings.HasPrefix(r.URL.Path, "/v1/admin/") {
			if !s.validateAdmin(r) {
				writeJSON(w, 401, map[string]string{"error": "unauthorized"})
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Extract key from Authorization header, X-API-Key, or query param
		key := extractAPIKey(r)
		if key == "" {
			if s.keyStore.IsDevMode() {
				next.ServeHTTP(w, r)
				return
			}
			writeJSON(w, 401, map[string]string{"error": "missing api key"})
			return
		}

		// Validate key
		ak, ok := s.keyStore.Validate(key)
		if !ok {
			writeJSON(w, 401, map[string]string{"error": "invalid api key"})
			return
		}

		// Rate limiting
		limiter := s.keyStore.GetRateLimiter(ak.Key)
		if limiter != nil && !limiter.Allow() {
			writeJSON(w, 429, map[string]string{"error": "rate limit exceeded"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// validateAdmin checks whether the request carries the admin key.
func (s *Server) validateAdmin(r *http.Request) bool {
	key := extractAPIKey(r)
	return key != "" && key == s.keyStore.AdminKey()
}

// extractAPIKey extracts an API key from the request in order:
// 1. Authorization: Bearer <token>
// 2. X-API-Key header
// 3. api_key query parameter
func extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	if key := r.URL.Query().Get("api_key"); key != "" {
		return key
	}
	return ""
}