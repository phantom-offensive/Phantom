//go:build windows

package implant

func executeBOFPlatform(bofData []byte, args []byte) ([]byte, error) {
	return executeBOFWindows(bofData, args)
}
