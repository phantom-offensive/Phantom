package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/phantom-c2/phantom/internal/cli"
	"github.com/phantom-c2/phantom/internal/server"
	"github.com/phantom-c2/phantom/internal/webui"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "configs/server.yaml", "Path to server configuration file")
	mode := flag.String("mode", "", "Interface mode: cli, web, or both (default: asks on startup)")
	webAddr := flag.String("web-addr", "0.0.0.0:3000", "Web UI bind address")
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

	// Determine interface mode
	selectedMode := *mode
	if selectedMode == "" {
		selectedMode = promptMode()
	}

	switch selectedMode {
	case "cli", "1":
		cli.Info("Starting CLI interface...")
		cli.Info("Type 'help' for available commands")
		cli.Info("Tip: type 'webui' to also start the Web UI")
		fmt.Println()
		shell := cli.NewShell(srv)
		shell.Run()

	case "web", "2":
		cli.Info("Starting Web UI only at http://%s", *webAddr)
		cli.Info("Open this URL in your browser to manage Phantom")
		cli.Info("Press Ctrl+C to stop")
		fmt.Println()

		ui := webui.New(srv, *webAddr)
		go func() {
			if err := ui.Start(); err != nil {
				cli.Error("Web UI error: %v", err)
				os.Exit(1)
			}
		}()

		// Wait for Ctrl+C
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		fmt.Println()
		cli.Info("Shutting down...")
		srv.Shutdown()

	case "both", "3":
		cli.Info("Starting both CLI and Web UI")

		// Start Web UI in background
		ui := webui.New(srv, *webAddr)
		go func() {
			if err := ui.Start(); err != nil {
				cli.Error("Web UI error: %v", err)
			}
		}()
		cli.Success("Web UI started at http://%s", *webAddr)
		cli.Info("Type 'help' for available CLI commands")
		fmt.Println()

		// Start CLI in foreground
		shell := cli.NewShell(srv)
		shell.Run()

	default:
		cli.Error("Unknown mode: %s (use: cli, web, both)", selectedMode)
		os.Exit(1)
	}
}

// promptMode asks the user which interface they want to use.
func promptMode() string {
	fmt.Println()
	fmt.Printf("  %s%sSelect Interface Mode:%s\n", cli.ColorBold, cli.ColorPurple, cli.ColorReset)
	fmt.Printf("  %s─────────────────────────────────%s\n", cli.ColorDim, cli.ColorReset)
	fmt.Printf("  %s[1]%s CLI          Command-line interface (terminal)\n", cli.ColorCyan, cli.ColorReset)
	fmt.Printf("  %s[2]%s Web UI       Browser-based dashboard (http://localhost:3000)\n", cli.ColorCyan, cli.ColorReset)
	fmt.Printf("  %s[3]%s Both         CLI + Web UI running together\n", cli.ColorCyan, cli.ColorReset)
	fmt.Println()
	fmt.Printf("  %sChoice [1/2/3]:%s ", cli.ColorPurple, cli.ColorReset)

	var choice string
	fmt.Scanln(&choice)

	switch choice {
	case "1", "cli":
		return "cli"
	case "2", "web":
		return "web"
	case "3", "both":
		return "both"
	default:
		return "cli" // Default to CLI
	}
}
