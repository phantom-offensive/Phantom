package util

import "crypto/sha256"

// SessionKeyIDFromKey returns the first 8 bytes of SHA-256(key).
// Used to match incoming envelopes to agent session keys.
func SessionKeyIDFromKey(key []byte) [8]byte {
	hash := sha256.Sum256(key)
	var id [8]byte
	copy(id[:], hash[:8])
	return id
}
