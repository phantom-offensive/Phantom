package webui

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/phantom-c2/phantom/internal/server"
)

// DiagCheck is a single health check result.
type DiagCheck struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Status string `json:"status"` // pass | warn | fail
}

// DiagGroup is a category of health checks.
type DiagGroup struct {
	Name   string      `json:"name"`
	Checks []DiagCheck `json:"checks"`
}

// DiagnosticsResult is the full structured diagnostics payload.
type DiagnosticsResult struct {
	Summary struct {
		Passed   int `json:"passed"`
		Failed   int `json:"failed"`
		Warnings int `json:"warnings"`
		Total    int `json:"total"`
	} `json:"summary"`
	Groups []DiagGroup `json:"groups"`
}

// handleDiagnostics runs the full system health check and returns it as JSON.
func (w *WebUI) handleDiagnostics(rw http.ResponseWriter, r *http.Request) {
	result := collectDiagnostics(w.server, w.bindAddr)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(result)
}

func collectDiagnostics(srv *server.Server, webUIBind string) DiagnosticsResult {
	var res DiagnosticsResult
	groups := []DiagGroup{}

	// ── System ──
	sys := DiagGroup{Name: "System"}
	wd, _ := os.Getwd()
	sys.Checks = []DiagCheck{
		{Name: "OS", Value: runtime.GOOS + "/" + runtime.GOARCH, Status: "pass"},
		{Name: "Go Version", Value: runtime.Version(), Status: "pass"},
		{Name: "CPUs", Value: fmt.Sprintf("%d", runtime.NumCPU()), Status: "pass"},
		{Name: "Working Dir", Value: wd, Status: "pass"},
	}
	groups = append(groups, sys)

	// ── Configuration ──
	cfg := DiagGroup{Name: "Configuration"}
	configFiles := []struct{ name, path string }{
		{"Server Config", "configs/server.yaml"},
		{"RSA Private Key", "configs/server.key"},
		{"RSA Public Key", "configs/server.pub"},
		{"Default Profile", "configs/profiles/default.yaml"},
	}
	for _, f := range configFiles {
		if _, err := os.Stat(f.path); err == nil {
			cfg.Checks = append(cfg.Checks, DiagCheck{f.name, f.path, "pass"})
		} else {
			cfg.Checks = append(cfg.Checks, DiagCheck{f.name, f.path + " — NOT FOUND", "fail"})
		}
	}
	if _, err := os.Stat("configs/server.crt"); err == nil {
		cfg.Checks = append(cfg.Checks, DiagCheck{"TLS Certificate", "configs/server.crt", "pass"})
	} else {
		cfg.Checks = append(cfg.Checks, DiagCheck{"TLS Certificate", "Not found (HTTPS listeners won't work)", "warn"})
	}
	if _, err := os.Stat("configs/.phantom_creds"); err == nil {
		cfg.Checks = append(cfg.Checks, DiagCheck{"Operator Creds", "Configured", "pass"})
	} else {
		cfg.Checks = append(cfg.Checks, DiagCheck{"Operator Creds", "Not set up (will prompt on startup)", "warn"})
	}
	groups = append(groups, cfg)

	// ── Database ──
	db := DiagGroup{Name: "Database"}
	if srv != nil && srv.DB != nil {
		db.Checks = append(db.Checks, DiagCheck{"SQLite", "Connected", "pass"})
		if agents, err := srv.AgentMgr.List(); err == nil {
			db.Checks = append(db.Checks, DiagCheck{"Agents in DB", fmt.Sprintf("%d", len(agents)), "pass"})
		}
	} else if _, err := os.Stat("data/phantom.db"); err == nil {
		db.Checks = append(db.Checks, DiagCheck{"SQLite", "data/phantom.db exists", "pass"})
	} else {
		db.Checks = append(db.Checks, DiagCheck{"SQLite", "No database yet (created on first run)", "warn"})
	}
	if err := os.MkdirAll("data", 0755); err == nil {
		db.Checks = append(db.Checks, DiagCheck{"Data Dir", "data/ — writable", "pass"})
	} else {
		db.Checks = append(db.Checks, DiagCheck{"Data Dir", "data/ — NOT WRITABLE", "fail"})
	}
	groups = append(groups, db)

	// ── Network ──
	netGrp := DiagGroup{Name: "Network"}

	// Build the set of ports Phantom itself is already listening on so we
	// do not report our own listeners as conflicts.
	phantomPorts := map[string]bool{}
	if srv != nil {
		for _, l := range srv.ListenerMgr.List() {
			if l.IsRunning() {
				addr := l.GetBindAddr()
				port := addr
				if strings.Contains(addr, ":") {
					if _, p, err := net.SplitHostPort(addr); err == nil {
						port = p
					}
				}
				phantomPorts[port] = true
			}
		}
	}
	if webUIBind != "" {
		port := webUIBind
		if strings.Contains(webUIBind, ":") {
			if _, p, err := net.SplitHostPort(webUIBind); err == nil {
				port = p
			}
		}
		phantomPorts[port] = true
	}

	ports := []struct{ name, addr, port string }{
		{"HTTP", "0.0.0.0:8080", "8080"},
		{"HTTPS", "0.0.0.0:443", "443"},
		{"DNS", "0.0.0.0:53", "53"},
		{"Web UI", "0.0.0.0:3000", "3000"},
	}
	for _, p := range ports {
		if phantomPorts[p.port] {
			netGrp.Checks = append(netGrp.Checks, DiagCheck{p.name + " (" + p.port + ")", "Phantom is listening", "pass"})
			continue
		}
		ln, err := net.Listen("tcp", p.addr)
		if err != nil {
			msg := err.Error()
			status := "warn"
			if strings.Contains(msg, "permission denied") {
				msg = "Permission denied (needs sudo for ports < 1024)"
			} else if strings.Contains(msg, "address already in use") {
				msg = "In use by another service"
			}
			netGrp.Checks = append(netGrp.Checks, DiagCheck{p.name + " (" + p.port + ")", msg, status})
		} else {
			ln.Close()
			netGrp.Checks = append(netGrp.Checks, DiagCheck{p.name + " (" + p.port + ")", "Available", "pass"})
		}
	}
	conn, err := net.DialTimeout("tcp", "8.8.8.8:53", 3*time.Second)
	if err == nil {
		conn.Close()
		netGrp.Checks = append(netGrp.Checks, DiagCheck{"Outbound", "Internet reachable", "pass"})
	} else {
		netGrp.Checks = append(netGrp.Checks, DiagCheck{"Outbound", "No internet (DNS listener may not work)", "warn"})
	}
	groups = append(groups, netGrp)

	// ── Listeners ──
	if srv != nil {
		ls := DiagGroup{Name: "Listeners"}
		listeners := srv.ListenerMgr.List()
		if len(listeners) == 0 {
			ls.Checks = append(ls.Checks, DiagCheck{"Listeners", "None configured", "warn"})
		} else {
			for _, l := range listeners {
				status := "stopped"
				result := "warn"
				if l.IsRunning() {
					status = "running"
					result = "pass"
				}
				ls.Checks = append(ls.Checks, DiagCheck{l.GetName(), fmt.Sprintf("%s %s (%s)", l.GetType(), l.GetBindAddr(), status), result})
			}
		}
		groups = append(groups, ls)
	}

	// ── Build Tools ──
	build := DiagGroup{Name: "Build Tools"}
	if goPath, err := exec.LookPath("go"); err == nil {
		build.Checks = append(build.Checks, DiagCheck{"Go Compiler", goPath, "pass"})
	} else {
		build.Checks = append(build.Checks, DiagCheck{"Go Compiler", "Not found (can't build agents)", "fail"})
	}
	if garblePath, err := exec.LookPath("garble"); err == nil {
		build.Checks = append(build.Checks, DiagCheck{"Garble", garblePath, "pass"})
	} else {
		build.Checks = append(build.Checks, DiagCheck{"Garble", "Not installed (obfuscated builds unavailable)", "warn"})
	}
	if _, err := exec.LookPath("docker"); err == nil {
		build.Checks = append(build.Checks, DiagCheck{"Docker", "Available", "pass"})
	} else {
		build.Checks = append(build.Checks, DiagCheck{"Docker", "Not installed (Docker deployment unavailable)", "warn"})
	}
	groups = append(groups, build)

	// ── Directories ──
	dirsGrp := DiagGroup{Name: "Directories"}
	for _, dir := range []string{"build/agents", "build/payloads", "logs", "reports", "data"} {
		os.MkdirAll(dir, 0755)
		if _, err := os.Stat(dir); err == nil {
			dirsGrp.Checks = append(dirsGrp.Checks, DiagCheck{dir, "OK", "pass"})
		} else {
			dirsGrp.Checks = append(dirsGrp.Checks, DiagCheck{dir, "Cannot create", "fail"})
		}
	}
	groups = append(groups, dirsGrp)

	// ── Summary ──
	for _, g := range groups {
		for _, c := range g.Checks {
			switch c.Status {
			case "pass":
				res.Summary.Passed++
			case "warn":
				res.Summary.Warnings++
			case "fail":
				res.Summary.Failed++
			}
		}
	}
	res.Summary.Total = res.Summary.Passed + res.Summary.Warnings + res.Summary.Failed
	res.Groups = groups
	return res
}
