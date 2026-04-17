package cli

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/phantom-c2/phantom/internal/server"
)

// RunDiagnostics performs a full system health check and reports issues.
func RunDiagnostics(srv *server.Server) {
	fmt.Println()
	fmt.Printf("  %s%sPhantom C2 — Diagnostics%s\n", colorBold, colorPurple, colorReset)
	fmt.Printf("  %s══════════════════════════════════════════════════%s\n", colorDim, colorReset)
	fmt.Println()

	passed := 0
	failed := 0
	warnings := 0

	// ── System Info ──
	fmt.Printf("  %sSystem%s\n", colorYellow, colorReset)
	printDiag("OS", runtime.GOOS+"/"+runtime.GOARCH, "pass")
	printDiag("Go Version", runtime.Version(), "pass")
	printDiag("CPUs", fmt.Sprintf("%d", runtime.NumCPU()), "pass")

	wd, _ := os.Getwd()
	printDiag("Working Dir", wd, "pass")
	passed += 4
	fmt.Println()

	// ── Configuration Files ──
	fmt.Printf("  %sConfiguration%s\n", colorYellow, colorReset)

	configFiles := map[string]string{
		"Server Config":  "configs/server.yaml",
		"RSA Private Key": "configs/server.key",
		"RSA Public Key":  "configs/server.pub",
		"Default Profile": "configs/profiles/default.yaml",
	}

	for name, path := range configFiles {
		if _, err := os.Stat(path); err == nil {
			printDiag(name, path, "pass")
			passed++
		} else {
			printDiag(name, path+" — NOT FOUND", "fail")
			failed++
		}
	}

	// TLS certs (optional)
	if _, err := os.Stat("configs/server.crt"); err == nil {
		printDiag("TLS Certificate", "configs/server.crt", "pass")
		passed++
	} else {
		printDiag("TLS Certificate", "Not found (HTTPS listeners won't work)", "warn")
		warnings++
	}

	// Credentials
	if _, err := os.Stat("configs/.phantom_creds"); err == nil {
		printDiag("Operator Creds", "Configured", "pass")
		passed++
	} else {
		printDiag("Operator Creds", "Not set up (will prompt on startup)", "warn")
		warnings++
	}
	fmt.Println()

	// ── Database ──
	fmt.Printf("  %sDatabase%s\n", colorYellow, colorReset)
	if srv != nil && srv.DB != nil {
		printDiag("SQLite", "Connected", "pass")
		passed++

		agents, _ := srv.AgentMgr.List()
		printDiag("Agents in DB", fmt.Sprintf("%d", len(agents)), "pass")
		passed++
	} else {
		if _, err := os.Stat("data/phantom.db"); err == nil {
			printDiag("SQLite", "data/phantom.db exists", "pass")
			passed++
		} else {
			printDiag("SQLite", "No database yet (created on first run)", "warn")
			warnings++
		}
	}

	// Check data directory writable
	if err := os.MkdirAll("data", 0755); err == nil {
		printDiag("Data Dir", "data/ — writable", "pass")
		passed++
	} else {
		printDiag("Data Dir", "data/ — NOT WRITABLE", "fail")
		failed++
	}
	fmt.Println()

	// ── Network ──
	fmt.Printf("  %sNetwork%s\n", colorYellow, colorReset)

	ports := []struct {
		name string
		addr string
	}{
		{"HTTP (8080)", "0.0.0.0:8080"},
		{"HTTPS (443)", "0.0.0.0:443"},
		{"DNS (53)", "0.0.0.0:53"},
		{"Web UI (3000)", "0.0.0.0:3000"},
	}

	for _, p := range ports {
		ln, err := net.Listen("tcp", p.addr)
		if err != nil {
			if strings.Contains(err.Error(), "permission denied") {
				printDiag(p.name, "Permission denied (needs sudo for ports < 1024)", "warn")
				warnings++
			} else if strings.Contains(err.Error(), "address already in use") {
				printDiag(p.name, "Port already in use — check for other services", "fail")
				failed++
			} else {
				printDiag(p.name, err.Error(), "fail")
				failed++
			}
		} else {
			ln.Close()
			printDiag(p.name, "Available", "pass")
			passed++
		}
	}

	// Check outbound connectivity
	conn, err := net.DialTimeout("tcp", "8.8.8.8:53", 3*time.Second)
	if err == nil {
		conn.Close()
		printDiag("Outbound", "Internet reachable", "pass")
		passed++
	} else {
		printDiag("Outbound", "No internet (DNS listener may not work)", "warn")
		warnings++
	}
	fmt.Println()

	// ── Listeners ──
	if srv != nil {
		fmt.Printf("  %sListeners%s\n", colorYellow, colorReset)
		listeners := srv.ListenerMgr.List()
		if len(listeners) == 0 {
			printDiag("Listeners", "None configured", "warn")
			warnings++
		} else {
			for _, l := range listeners {
				status := "stopped"
				result := "warn"
				if l.IsRunning() {
					status = "running"
					result = "pass"
				}
				printDiag(l.GetName(), fmt.Sprintf("%s %s (%s)", l.GetType(), l.GetBindAddr(), status), result)
				if result == "pass" {
					passed++
				} else {
					warnings++
				}
			}
		}
		fmt.Println()
	}

	// ── Build Tools ──
	fmt.Printf("  %sBuild Tools%s\n", colorYellow, colorReset)

	goPath, err := exec.LookPath("go")
	if err == nil {
		printDiag("Go Compiler", goPath, "pass")
		passed++
	} else {
		printDiag("Go Compiler", "Not found (can't build agents from CLI)", "fail")
		failed++
	}

	garblePath, err := exec.LookPath("garble")
	if err == nil {
		printDiag("Garble", garblePath, "pass")
		passed++
	} else {
		printDiag("Garble", "Not installed (obfuscated builds unavailable)", "warn")
		warnings++
	}

	_, err = exec.LookPath("docker")
	if err == nil {
		printDiag("Docker", "Available", "pass")
		passed++
	} else {
		printDiag("Docker", "Not installed (Docker deployment unavailable)", "warn")
		warnings++
	}
	fmt.Println()

	// ── Directories ──
	fmt.Printf("  %sDirectories%s\n", colorYellow, colorReset)
	dirs := []string{"build/agents", "build/payloads", "logs", "reports", "data"}
	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
		if _, err := os.Stat(dir); err == nil {
			printDiag(dir, "OK", "pass")
			passed++
		} else {
			printDiag(dir, "Cannot create", "fail")
			failed++
		}
	}
	fmt.Println()

	// ── Summary ──
	fmt.Printf("  %s══════════════════════════════════════════════════%s\n", colorDim, colorReset)
	total := passed + failed + warnings
	fmt.Printf("  Results: %s%d passed%s  ", colorGreen, passed, colorReset)
	if failed > 0 {
		fmt.Printf("%s%d failed%s  ", colorRed, failed, colorReset)
	}
	if warnings > 0 {
		fmt.Printf("%s%d warnings%s", colorYellow, warnings, colorReset)
	}
	fmt.Printf("  (%d total checks)\n", total)

	if failed > 0 {
		fmt.Printf("\n  %sFix the failed checks above before running Phantom.%s\n", colorRed, colorReset)
		fmt.Printf("  %sRun 'make keygen' to generate missing RSA keys.%s\n", colorDim, colorReset)
		fmt.Printf("  %sRun 'bash scripts/generate_certs.sh' for TLS certs.%s\n", colorDim, colorReset)
	} else {
		fmt.Printf("\n  %s%sAll critical checks passed — Phantom is ready to run!%s\n", colorBold, colorGreen, colorReset)
	}
	fmt.Println()
}

func printDiag(name, value, result string) {
	icon := colorGreen + "✓" + colorReset
	if result == "fail" {
		icon = colorRed + "✗" + colorReset
	} else if result == "warn" {
		icon = colorYellow + "!" + colorReset
	}
	fmt.Printf("    %s %-20s %s\n", icon, name, value)
}
