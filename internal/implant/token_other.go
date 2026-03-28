//go:build !windows

package implant

import "fmt"

func ExecuteTokenCommand(args []string) ([]byte, error) {
	return nil, fmt.Errorf("token manipulation requires Windows")
}
