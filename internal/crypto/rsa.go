package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
)

const RSAKeyBits = 2048

// GenerateRSAKeyPair generates a new RSA-2048 key pair.
func GenerateRSAKeyPair() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, RSAKeyBits)
	if err != nil {
		return nil, nil, err
	}
	return privKey, &privKey.PublicKey, nil
}

// RSAEncrypt encrypts data using RSA-OAEP with SHA-256.
func RSAEncrypt(pubKey *rsa.PublicKey, plaintext []byte) ([]byte, error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, plaintext, nil)
}

// RSADecrypt decrypts data using RSA-OAEP with SHA-256.
func RSADecrypt(privKey *rsa.PrivateKey, ciphertext []byte) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, privKey, ciphertext, nil)
}

// SavePrivateKey writes an RSA private key to a PEM file.
func SavePrivateKey(privKey *rsa.PrivateKey, path string) error {
	keyBytes := x509.MarshalPKCS1PrivateKey(privKey)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}
	return os.WriteFile(path, pem.EncodeToMemory(block), 0600)
}

// SavePublicKey writes an RSA public key to a PEM file.
func SavePublicKey(pubKey *rsa.PublicKey, path string) error {
	keyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return err
	}
	block := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: keyBytes,
	}
	return os.WriteFile(path, pem.EncodeToMemory(block), 0644)
}

// LoadPrivateKey reads an RSA private key from a PEM file.
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// LoadPublicKey reads an RSA public key from a PEM file.
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	return rsaPub, nil
}

// PublicKeyToBytes serializes an RSA public key to DER bytes.
func PublicKeyToBytes(pubKey *rsa.PublicKey) ([]byte, error) {
	return x509.MarshalPKIXPublicKey(pubKey)
}

// PublicKeyFromBytes deserializes an RSA public key from DER bytes.
func PublicKeyFromBytes(data []byte) (*rsa.PublicKey, error) {
	pub, err := x509.ParsePKIXPublicKey(data)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	return rsaPub, nil
}
