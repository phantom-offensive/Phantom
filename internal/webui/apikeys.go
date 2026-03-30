package webui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ══════════════════════════════════════════
//  API KEY AUTHENTICATION
// ══════════════════════════════════════════

var (
	apiKeys   = make(map[string]*APIKey) // key -> info
	apiKeysMu sync.RWMutex
)

type APIKey struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	LastUsed  string `json:"last_used"`
	Requests  int    `json:"requests"`
}

func generateAPIKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "ph-" + hex.EncodeToString(b)
}

// ValidateAPIKey checks if a request has a valid API key in the header.
// Used as an alternative to cookie-based session auth for scripting.
func ValidateAPIKey(r *http.Request) bool {
	key := r.Header.Get("X-API-Key")
	if key == "" {
		// Also check Authorization: Bearer <key>
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ph-") {
			key = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if key == "" {
		return false
	}

	apiKeysMu.Lock()
	defer apiKeysMu.Unlock()
	ak, ok := apiKeys[key]
	if !ok {
		return false
	}
	ak.LastUsed = time.Now().Format("2006-01-02 15:04:05")
	ak.Requests++
	return true
}

func (w *WebUI) handleAPIKeys(rw http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		apiKeysMu.RLock()
		defer apiKeysMu.RUnlock()
		keys := make([]APIKey, 0, len(apiKeys))
		for _, k := range apiKeys {
			masked := APIKey{
				Key:       k.Key[:12] + "..." + k.Key[len(k.Key)-4:],
				Name:      k.Name,
				CreatedAt: k.CreatedAt,
				LastUsed:  k.LastUsed,
				Requests:  k.Requests,
			}
			keys = append(keys, masked)
		}
		writeJSON(rw, keys)
		return
	}

	var req struct {
		Action string `json:"action"` // create, revoke
		Name   string `json:"name"`
		Key    string `json:"key"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	switch req.Action {
	case "create":
		if req.Name == "" {
			req.Name = "api-key"
		}
		key := generateAPIKey()
		apiKeysMu.Lock()
		apiKeys[key] = &APIKey{
			Key:       key,
			Name:      req.Name,
			CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
		}
		apiKeysMu.Unlock()
		writeJSON(rw, map[string]string{"key": key, "name": req.Name, "status": "created"})

	case "revoke":
		apiKeysMu.Lock()
		// Find key by prefix match
		for k := range apiKeys {
			if strings.HasPrefix(k, req.Key) || strings.HasSuffix(k, req.Key[len(req.Key)-4:]) {
				delete(apiKeys, k)
				break
			}
		}
		apiKeysMu.Unlock()
		writeJSON(rw, map[string]string{"status": "revoked"})

	default:
		writeJSON(rw, map[string]string{"error": "action must be create or revoke"})
	}
}
