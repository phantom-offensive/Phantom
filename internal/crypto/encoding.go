package crypto

import (
	"encoding/base64"
	"encoding/hex"
)

// Base64Encode encodes bytes to base64 string.
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode decodes a base64 string to bytes.
func Base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// HexEncode encodes bytes to hex string.
func HexEncode(data []byte) string {
	return hex.EncodeToString(data)
}

// HexDecode decodes a hex string to bytes.
func HexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
