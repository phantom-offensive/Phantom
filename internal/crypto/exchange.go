package crypto

import (
	"crypto/rsa"
	"crypto/sha256"
	"errors"
)

// KeyExchangeRequest is sent by the agent during registration.
// It contains the proposed AES session key encrypted with the server's RSA public key.
// Format: [32-byte AES key | remaining payload bytes]
type KeyExchangeRequest struct {
	EncryptedBlob []byte // RSA-OAEP encrypted: AES key + serialized registration data
}

// PackKeyExchange encrypts an AES session key and payload together using RSA.
// The agent calls this to build the registration request.
func PackKeyExchange(serverPubKey *rsa.PublicKey, sessionKey []byte, payload []byte) ([]byte, error) {
	if len(sessionKey) != AESKeySize {
		return nil, errors.New("session key must be 32 bytes")
	}

	// Combine: [32-byte AES key][payload]
	blob := make([]byte, 0, len(sessionKey)+len(payload))
	blob = append(blob, sessionKey...)
	blob = append(blob, payload...)

	return RSAEncrypt(serverPubKey, blob)
}

// UnpackKeyExchange decrypts the registration blob using the server's RSA private key.
// Returns the AES session key and the remaining payload.
func UnpackKeyExchange(serverPrivKey *rsa.PrivateKey, encrypted []byte) (sessionKey []byte, payload []byte, err error) {
	blob, err := RSADecrypt(serverPrivKey, encrypted)
	if err != nil {
		return nil, nil, err
	}

	if len(blob) < AESKeySize {
		return nil, nil, errors.New("decrypted blob too short to contain AES key")
	}

	sessionKey = blob[:AESKeySize]
	payload = blob[AESKeySize:]
	return sessionKey, payload, nil
}

// SessionKeyID returns the first 8 bytes of SHA-256(sessionKey).
// Used to identify which session key to use for decryption without exposing the key.
func SessionKeyID(sessionKey []byte) [8]byte {
	hash := sha256.Sum256(sessionKey)
	var id [8]byte
	copy(id[:], hash[:8])
	return id
}
