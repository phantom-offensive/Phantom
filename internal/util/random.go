package util

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

var adjectives = []string{
	"silent", "dark", "cold", "red", "swift", "hidden", "ghost",
	"shadow", "iron", "steel", "black", "crimson", "frost", "amber",
	"toxic", "rogue", "savage", "grim", "feral", "hollow", "dire",
	"ashen", "obsidian", "cobalt", "azure", "phantom", "void", "blaze",
}

var nouns = []string{
	"falcon", "wolf", "raven", "tiger", "bear", "viper", "cobra",
	"hawk", "eagle", "panther", "spider", "scorpion", "dragon", "hydra",
	"jackal", "lynx", "mantis", "serpent", "fox", "shark", "owl",
	"cipher", "dagger", "blade", "storm", "wraith", "spectre", "reaper",
}

// GenerateAgentName creates a random adjective-noun pair (e.g., "silent-falcon").
func GenerateAgentName() string {
	adj := adjectives[randomInt(len(adjectives))]
	noun := nouns[randomInt(len(nouns))]
	return adj + "-" + noun
}

// RandomString generates a cryptographically secure random hex string of n bytes.
func RandomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// RandomID generates an 8-character random hex ID.
func RandomID() string {
	return RandomString(4)
}

func randomInt(max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}

// ShortID returns the first 8 chars of an ID for display.
func ShortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// PadRight pads a string to a minimum width.
func PadRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}
