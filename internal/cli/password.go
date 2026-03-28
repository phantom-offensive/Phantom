package cli

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/term"
)

// ReadPassword reads a password from stdin with asterisk masking.
// Falls back to plain text if terminal raw mode is not available.
func ReadPassword(prompt string) string {
	fmt.Print(prompt)

	// Try using x/term for proper password masking
	fd := int(syscall.Stdin)
	if term.IsTerminal(fd) {
		password, err := term.ReadPassword(fd)
		fmt.Println() // New line after password entry
		if err == nil {
			return string(password)
		}
	}

	// Fallback: read with asterisk display (manual raw mode)
	password := readPasswordManual()
	fmt.Println()
	return password
}

// readPasswordManual reads password character by character, displaying * for each.
func readPasswordManual() string {
	// Try to set terminal to raw mode
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Can't do raw mode — fall back to plain text
		var pass string
		fmt.Scanln(&pass)
		return pass
	}
	defer term.Restore(fd, oldState)

	var password []byte
	buf := make([]byte, 1)

	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}

		char := buf[0]

		switch char {
		case '\n', '\r': // Enter
			return string(password)
		case 127, '\b': // Backspace
			if len(password) > 0 {
				password = password[:len(password)-1]
				fmt.Print("\b \b") // Erase the asterisk
			}
		case 3: // Ctrl+C
			fmt.Println()
			os.Exit(0)
		default:
			if char >= 32 && char < 127 { // Printable character
				password = append(password, char)
				fmt.Print("*")
			}
		}
	}

	return string(password)
}
