package crypto

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// KeyExchangeRequest is sent by the agent during registration.
// It contains the proposed AES session key encrypted with the server's RSA public key.
// Format: [32-byte AES key | remaining payload bytes]
type KeyExchangeRequest struct {
	EncryptedBlob []byte // RSA-OAEP encrypted: AES key + serialized registration data
}

// PackKeyExchange builds the registration blob using hybrid encryption: the
// 32-byte AES session key is RSA-OAEP encrypted (small, fits in one RSA
// block), while the registration payload is AES-GCM encrypted under that
// session key. This removes RSA's message-size limit so larger registration
// payloads (e.g. with implant ID) do not overflow.
//
// Wire format: [2-byte big-endian rsaLen][rsaBlob][aesBlob]
func PackKeyExchange(serverPubKey *rsa.PublicKey, sessionKey []byte, payload []byte) ([]byte, error) {
	if len(sessionKey) != AESKeySize {
		return nil, errors.New("session key must be 32 bytes")
	}

	rsaBlob, err := RSAEncrypt(serverPubKey, sessionKey)
	if err != nil {
		return nil, err
	}

	aesBlob, err := AESEncrypt(sessionKey, payload)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, 2+len(rsaBlob)+len(aesBlob))
	var lenBuf [2]byte
	binary.BigEndian.PutUint16(lenBuf[:], uint16(len(rsaBlob)))
	out = append(out, lenBuf[:]...)
	out = append(out, rsaBlob...)
	out = append(out, aesBlob...)
	return out, nil
}

// UnpackKeyExchange parses a registration blob, returning the AES session key
// and the decrypted registration payload. It supports both the legacy format
// (a single RSA-OAEP blob of [AES key][payload], used by PhantomImplant) and
// the hybrid format ([2-byte rsaLen][rsaBlob][aesBlob]).
func UnpackKeyExchange(serverPrivKey *rsa.PrivateKey, encrypted []byte) (sessionKey []byte, payload []byte, err error) {
	// Legacy format: RSA ciphertext is exactly one key-size block.
	if len(encrypted) == serverPrivKey.Size() {
		blob, derr := RSADecrypt(serverPrivKey, encrypted)
		if derr != nil {
			return nil, nil, derr
		}
		if len(blob) < AESKeySize {
			return nil, nil, errors.New("decrypted blob too short to contain AES key")
		}
		return blob[:AESKeySize], blob[AESKeySize:], nil
	}

	// Hybrid format.
	if len(encrypted) < 2 {
		return nil, nil, errors.New("key exchange blob too short")
	}
	rsaLen := int(binary.BigEndian.Uint16(encrypted[:2]))
	if len(encrypted) < 2+rsaLen {
		return nil, nil, errors.New("key exchange blob truncated")
	}

	sessionKey, err = RSADecrypt(serverPrivKey, encrypted[2:2+rsaLen])
	if err != nil {
		return nil, nil, err
	}

	payload, err = AESDecrypt(sessionKey, encrypted[2+rsaLen:])
	if err != nil {
		return nil, nil, err
	}
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
