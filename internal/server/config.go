package server

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the server configuration loaded from YAML.
type Config struct {
	Server   ServerConfig    `yaml:"server"`
	Listeners []ListenerConf `yaml:"listeners"`
	Logging  LoggingConfig   `yaml:"logging"`
}

// ServerConfig holds core server settings.
type ServerConfig struct {
	Bind          string `yaml:"bind"`
	Database      string `yaml:"database"`
	RSAPrivateKey string `yaml:"rsa_private_key"`
	RSAPublicKey  string `yaml:"rsa_public_key"`
	DefaultSleep  int    `yaml:"default_sleep"`
	DefaultJitter int    `yaml:"default_jitter"`
}

// ListenerConf holds listener configuration from the config file.
type ListenerConf struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	Bind    string `yaml:"bind"`
	Profile string `yaml:"profile"`
	TLSCert string `yaml:"tls_cert"`
	TLSKey  string `yaml:"tls_key"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

// LoadConfig reads and parses a YAML configuration file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Bind:          "0.0.0.0",
			Database:      "data/phantom.db",
			RSAPrivateKey: "configs/server.key",
			RSAPublicKey:  "configs/server.pub",
			DefaultSleep:  10,
			DefaultJitter: 20,
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
