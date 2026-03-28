package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// savedTermState holds the original terminal state before any password prompts.
// It's used to restore the terminal after all auth is complete.
var savedTermState *term.State

// SaveTerminalState saves the current terminal state before password prompts.
func SaveTerminalState() {
	fd := int(syscall.Stdin)
	if term.IsTerminal(fd) {
		state, err := term.GetState(fd)
		if err == nil {
			savedTermState = state
		}
	}
}

// RestoreTerminalState restores the terminal to its original state.
// Call this after all password prompts are done, before starting liner/readline.
func RestoreTerminalState() {
	if savedTermState != nil {
		fd := int(syscall.Stdin)
		term.Restore(fd, savedTermState)
		savedTermState = nil
	}
}

// ReadPassword reads a password from stdin showing * for each character.
// Uses golang.org/x/term which properly saves and restores terminal state.
func ReadPassword(prompt string) string {
	fmt.Print(prompt)

	fd := int(syscall.Stdin)
	if term.IsTerminal(fd) {
		password, err := term.ReadPassword(fd)
		fmt.Println() // New line after password
		if err == nil {
			return string(password)
		}
	}

	// Fallback for non-terminal (piped input, etc.)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}
