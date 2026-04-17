//go:build !darwin

package implant

// HarvestCredsDarwin is a stub for non-macOS builds.
// The real implementation lives in creds_darwin.go.
func HarvestCredsDarwin(args []string) ([]byte, error) {
	return []byte("[-] macOS credential harvesting is only available on darwin targets"), nil
}
