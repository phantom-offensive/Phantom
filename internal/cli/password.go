package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// ReadPassword reads a password from stdin with no echo (hidden input).
func ReadPassword(prompt string) string {
	fmt.Print(prompt)

	fd := int(syscall.Stdin)
	if term.IsTerminal(fd) {
		password, err := term.ReadPassword(fd)
		fmt.Println()
		if err == nil {
			return string(password)
		}
	}

	// Fallback
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

// ReopenStdin reopens /dev/tty as stdin to fix terminal state after
// term.ReadPassword corrupts the file descriptor on some systems (WSL).
// Must be called after all password prompts, before starting readline/liner.
func ReopenStdin() {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return // Not on Linux/macOS or no TTY available
	}

	// Replace os.Stdin with the fresh TTY
	syscall.Dup2(int(tty.Fd()), int(os.Stdin.Fd()))
	tty.Close()
}
