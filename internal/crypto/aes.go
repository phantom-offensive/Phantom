package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

const AESKeySize = 32 // AES-256

// GenerateAESKey generates a random 256-bit AES key.
func GenerateAESKey() ([]byte, error) {
	key := make([]byte, AESKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// AESEncrypt encrypts plaintext using AES-256-GCM.
// Returns nonce (12 bytes) prepended to the ciphertext.
func AESEncrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != AESKeySize {
		return nil, errors.New("invalid AES key size: expected 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// AESDecrypt decrypts AES-256-GCM ciphertext.
// Expects nonce (12 bytes) prepended to the ciphertext.
func AESDecrypt(key, data []byte) ([]byte, error) {
	if len(key) != AESKeySize {
		return nil, errors.New("invalid AES key size: expected 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
