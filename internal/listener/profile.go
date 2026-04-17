package listener

import (
	"os"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Profile defines a malleable communication profile.
type Profile struct {
	Name          string            `yaml:"name"`
	RegisterURI   string            `yaml:"register_uri"`
	CheckInURI    string            `yaml:"checkin_uri"`
	DecoyURIs     []string          `yaml:"decoy_uris"`
	UserAgent     string            `yaml:"user_agent"`
	Headers       map[string]string `yaml:"headers"`
	ContentType   string            `yaml:"content_type"`
	DecoyResponse string            `yaml:"decoy_response"`
	FrontDomain   string            `yaml:"front_domain"` // CDN domain for SNI-based domain fronting (e.g., "cdn.cloudflare.com")
	HostHeader    string            `yaml:"host_header"`  // Actual C2 Host header override for domain fronting
}

// ProfileFile wraps the YAML structure.
type ProfileFile struct {
	Profile Profile `yaml:"profile"`
}

// LoadProfile reads a profile from a YAML file.
func LoadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pf ProfileFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, err
	}

	// Set defaults
	if pf.Profile.ContentType == "" {
		pf.Profile.ContentType = "application/json"
	}
	if pf.Profile.RegisterURI == "" {
		pf.Profile.RegisterURI = "/api/v1/auth"
	}
	if pf.Profile.CheckInURI == "" {
		pf.Profile.CheckInURI = "/api/v1/status"
	}

	return &pf.Profile, nil
}

// DefaultProfile returns a basic default profile.
func DefaultProfile() *Profile {
	return &Profile{
		Name:        "default",
		RegisterURI: "/api/v1/auth",
		CheckInURI:  "/api/v1/status",
		DecoyURIs:   []string{"/api/v1/health"},
		UserAgent:   "",
		Headers: map[string]string{
			"Server":       "nginx/1.24.0",
			"Cache-Control": "no-cache, no-store",
		},
		ContentType:   "application/json",
		DecoyResponse: `{"status":"ok","version":"2.1.0"}`,
	}
}

// ResolveHeaders processes header templates (e.g., {{uuid}}).
func (p *Profile) ResolveHeaders() map[string]string {
	resolved := make(map[string]string, len(p.Headers))
	for k, v := range p.Headers {
		if strings.Contains(v, "{{uuid}}") {
			v = strings.ReplaceAll(v, "{{uuid}}", uuid.New().String())
		}
		resolved[k] = v
	}
	return resolved
}

// IsC2URI checks if a URI path matches a C2 communication endpoint.
func (p *Profile) IsC2URI(path string) bool {
	return path == p.RegisterURI || path == p.CheckInURI
}

// IsDecoyURI checks if a URI path matches a decoy endpoint.
func (p *Profile) IsDecoyURI(path string) bool {
	for _, uri := range p.DecoyURIs {
		if path == uri {
			return true
		}
	}
	return false
}
