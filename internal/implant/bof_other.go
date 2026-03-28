//go:build !windows && !linux

package implant

import "fmt"

func executeBOFPlatform(bofData []byte, args []byte) ([]byte, error) {
	return nil, fmt.Errorf("BOF execution not supported on this platform")
}
