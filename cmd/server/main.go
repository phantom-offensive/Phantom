package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/phantom-c2/phantom/internal/cli"
	"github.com/phantom-c2/phantom/internal/server"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "configs/server.yaml", "Path to server configuration file")
	flag.Parse()

	// Print banner
	cli.PrintBanner(version)

	// Load configuration
	cli.Info("Loading configuration from %s", *configPath)
	cfg, err := server.LoadConfig(*configPath)
	if err != nil {
		cli.Error("Failed to load config: %v", err)
		fmt.Println()
		cli.Info("Run 'make keygen' first to generate RSA keys")
		os.Exit(1)
	}

	// Initialize server
	cli.Info("Initializing server...")
	srv, err := server.New(cfg)
	if err != nil {
		cli.Error("Failed to initialize server: %v", err)
		os.Exit(1)
	}

	// Setup listeners from config
	cli.Info("Setting up listeners...")
	if err := srv.SetupListeners(); err != nil {
		cli.Error("Failed to setup listeners: %v", err)
		os.Exit(1)
	}

	// Auto-start HTTP listeners
	for _, lc := range cfg.Listeners {
		if lc.Type == "http" {
			if err := srv.StartListener(lc.Name); err != nil {
				cli.Warn("Could not auto-start listener %s: %v", lc.Name, err)
			} else {
				cli.Success("Listener '%s' started on %s (%s)", lc.Name, lc.Bind, lc.Type)
			}
		}
	}

	cli.Success("Phantom C2 server ready")
	cli.Info("Type 'help' for available commands")

	// Start interactive CLI shell
	shell := cli.NewShell(srv)
	shell.Run()
}
