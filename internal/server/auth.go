package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const (
	credFile      = "configs/.phantom_creds"
	tokenExpiry   = 24 * time.Hour
	tokenLength   = 32
)

// Credentials holds the operator login credentials.
type Credentials struct {
	Username string `json:"username"`
	PassHash string `json:"pass_hash"` // SHA-256(password + salt)
	Salt     string `json:"salt"`
	Created  string `json:"created"`
}

// AuthToken represents an active session token.
type AuthToken struct {
	Token     string
	Username  string
	ExpiresAt time.Time
}

// AuthManager handles operator authentication for both CLI and Web UI.
type AuthManager struct {
	creds       *Credentials
	tokens      map[string]*AuthToken // token -> session
}

// NewAuthManager creates a new authentication manager.
func NewAuthManager() *AuthManager {
	return &AuthManager{
		tokens: make(map[string]*AuthToken),
	}
}

// IsSetup returns true if credentials have been configured.
func (am *AuthManager) IsSetup() bool {
	_, err := os.Stat(credFile)
	return err == nil
}

// Setup creates initial credentials (first-run).
func (am *AuthManager) Setup(username, password string) error {
	salt := generateSalt()
	hash := hashPassword(password, salt)

	creds := &Credentials{
		Username: username,
		PassHash: hash,
		Salt:     salt,
		Created:  time.Now().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	os.MkdirAll("configs", 0755)
	if err := os.WriteFile(credFile, data, 0600); err != nil {
		return err
	}

	am.creds = creds
	return nil
}

// LoadCredentials reads stored credentials.
func (am *AuthManager) LoadCredentials() error {
	data, err := os.ReadFile(credFile)
	if err != nil {
		return err
	}

	am.creds = &Credentials{}
	return json.Unmarshal(data, am.creds)
}

// Authenticate validates username and password, returns a session token.
func (am *AuthManager) Authenticate(username, password string) (string, error) {
	if am.creds == nil {
		if err := am.LoadCredentials(); err != nil {
			return "", fmt.Errorf("no credentials configured")
		}
	}

	if username != am.creds.Username {
		return "", fmt.Errorf("invalid credentials")
	}

	hash := hashPassword(password, am.creds.Salt)
	if hash != am.creds.PassHash {
		return "", fmt.Errorf("invalid credentials")
	}

	// Generate session token
	token := generateToken()
	am.tokens[token] = &AuthToken{
		Token:     token,
		Username:  username,
		ExpiresAt: time.Now().Add(tokenExpiry),
	}

	return token, nil
}

// ValidateToken checks if a session token is valid.
func (am *AuthManager) ValidateToken(token string) bool {
	session, ok := am.tokens[token]
	if !ok {
		return false
	}
	if time.Now().After(session.ExpiresAt) {
		delete(am.tokens, token)
		return false
	}
	return true
}

// GetUsername returns the configured username.
func (am *AuthManager) GetUsername() string {
	if am.creds != nil {
		return am.creds.Username
	}
	return ""
}

// ── Helpers ──

func hashPassword(password, salt string) string {
	h := sha256.Sum256([]byte(password + salt))
	return hex.EncodeToString(h[:])
}

func generateSalt() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateToken() string {
	b := make([]byte, tokenLength)
	rand.Read(b)
	return hex.EncodeToString(b)
}
