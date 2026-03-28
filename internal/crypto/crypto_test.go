package crypto

import (
	"bytes"
	"testing"
)

func TestAESEncryptDecrypt(t *testing.T) {
	key, err := GenerateAESKey()
	if err != nil {
		t.Fatalf("GenerateAESKey: %v", err)
	}

	plaintext := []byte("phantom c2 test payload - AES-256-GCM round trip")

	ciphertext, err := AESEncrypt(key, plaintext)
	if err != nil {
		t.Fatalf("AESEncrypt: %v", err)
	}

	if bytes.Equal(plaintext, ciphertext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := AESDecrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("AESDecrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("decrypted text mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestAESDecryptWrongKey(t *testing.T) {
	key1, _ := GenerateAESKey()
	key2, _ := GenerateAESKey()

	ciphertext, _ := AESEncrypt(key1, []byte("secret data"))

	_, err := AESDecrypt(key2, ciphertext)
	if err == nil {
		t.Fatal("expected decryption to fail with wrong key")
	}
}

func TestAESInvalidKeySize(t *testing.T) {
	shortKey := []byte("tooshort")

	_, err := AESEncrypt(shortKey, []byte("data"))
	if err == nil {
		t.Fatal("expected error for short key")
	}

	_, err = AESDecrypt(shortKey, []byte("data"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestRSAEncryptDecrypt(t *testing.T) {
	privKey, pubKey, err := GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("GenerateRSAKeyPair: %v", err)
	}

	plaintext := []byte("phantom c2 test payload - RSA-OAEP round trip")

	ciphertext, err := RSAEncrypt(pubKey, plaintext)
	if err != nil {
		t.Fatalf("RSAEncrypt: %v", err)
	}

	decrypted, err := RSADecrypt(privKey, ciphertext)
	if err != nil {
		t.Fatalf("RSADecrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("decrypted text mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestRSAKeySaveLoad(t *testing.T) {
	privKey, pubKey, err := GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("GenerateRSAKeyPair: %v", err)
	}

	privPath := t.TempDir() + "/test.key"
	pubPath := t.TempDir() + "/test.pub"

	if err := SavePrivateKey(privKey, privPath); err != nil {
		t.Fatalf("SavePrivateKey: %v", err)
	}
	if err := SavePublicKey(pubKey, pubPath); err != nil {
		t.Fatalf("SavePublicKey: %v", err)
	}

	loadedPriv, err := LoadPrivateKey(privPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey: %v", err)
	}
	loadedPub, err := LoadPublicKey(pubPath)
	if err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}

	// Verify loaded keys work
	plaintext := []byte("round trip through saved keys")
	ct, err := RSAEncrypt(loadedPub, plaintext)
	if err != nil {
		t.Fatalf("RSAEncrypt with loaded key: %v", err)
	}
	pt, err := RSADecrypt(loadedPriv, ct)
	if err != nil {
		t.Fatalf("RSADecrypt with loaded key: %v", err)
	}
	if !bytes.Equal(plaintext, pt) {
		t.Fatal("round trip failed with loaded keys")
	}
}

func TestKeyExchange(t *testing.T) {
	privKey, pubKey, err := GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("GenerateRSAKeyPair: %v", err)
	}

	sessionKey, err := GenerateAESKey()
	if err != nil {
		t.Fatalf("GenerateAESKey: %v", err)
	}

	payload := []byte(`{"hostname":"DESKTOP-TEST","username":"admin","os":"windows"}`)

	encrypted, err := PackKeyExchange(pubKey, sessionKey, payload)
	if err != nil {
		t.Fatalf("PackKeyExchange: %v", err)
	}

	gotKey, gotPayload, err := UnpackKeyExchange(privKey, encrypted)
	if err != nil {
		t.Fatalf("UnpackKeyExchange: %v", err)
	}

	if !bytes.Equal(sessionKey, gotKey) {
		t.Fatal("session key mismatch after key exchange")
	}
	if !bytes.Equal(payload, gotPayload) {
		t.Fatal("payload mismatch after key exchange")
	}
}

func TestSessionKeyID(t *testing.T) {
	key1, _ := GenerateAESKey()
	key2, _ := GenerateAESKey()

	id1 := SessionKeyID(key1)
	id2 := SessionKeyID(key2)

	if id1 == id2 {
		t.Fatal("different keys should produce different IDs")
	}

	// Same key should produce same ID
	id1again := SessionKeyID(key1)
	if id1 != id1again {
		t.Fatal("same key should produce same ID")
	}
}

func TestBase64RoundTrip(t *testing.T) {
	data := []byte("phantom test data for encoding")
	encoded := Base64Encode(data)
	decoded, err := Base64Decode(encoded)
	if err != nil {
		t.Fatalf("Base64Decode: %v", err)
	}
	if !bytes.Equal(data, decoded) {
		t.Fatal("base64 round trip failed")
	}
}

func TestHexRoundTrip(t *testing.T) {
	data := []byte("phantom hex test")
	encoded := HexEncode(data)
	decoded, err := HexDecode(encoded)
	if err != nil {
		t.Fatalf("HexDecode: %v", err)
	}
	if !bytes.Equal(data, decoded) {
		t.Fatal("hex round trip failed")
	}
}
