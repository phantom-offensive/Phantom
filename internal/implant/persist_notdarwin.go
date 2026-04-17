//go:build !darwin

package implant

// InstallPersistenceDarwin is a stub for non-macOS builds.
// The real implementation lives in persist_darwin.go.
func InstallPersistenceDarwin(method, execPath string) ([]byte, error) {
	return []byte("[-] macOS persistence is only available on darwin targets"), nil
}

// RemovePersistenceDarwin is a stub for non-macOS builds.
func RemovePersistenceDarwin() ([]byte, error) {
	return []byte("[-] macOS persistence is only available on darwin targets"), nil
}
