//go:build linux

package implant

func executeBOFPlatform(bofData []byte, args []byte) ([]byte, error) {
	return executeBOFLinux(bofData, args)
}
