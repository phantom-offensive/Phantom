package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/phantom-c2/phantom/internal/crypto"
)

func main() {
	outDir := flag.String("out", "configs", "Output directory for key files")
	flag.Parse()

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════╗")
	fmt.Println("  ║   Phantom C2 — RSA Key Generator     ║")
	fmt.Println("  ╚══════════════════════════════════════╝")
	fmt.Println()

	privPath := filepath.Join(*outDir, "server.key")
	pubPath := filepath.Join(*outDir, "server.pub")

	// Check if keys already exist
	if _, err := os.Stat(privPath); err == nil {
		fmt.Printf("  [!] Key already exists: %s\n", privPath)
		fmt.Print("  [?] Overwrite? (y/N): ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("  [-] Aborted.")
			return
		}
	}

	fmt.Printf("  [*] Generating RSA-%d keypair...\n", crypto.RSAKeyBits)

	privKey, pubKey, err := crypto.GenerateRSAKeyPair()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [!] Error: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "  [!] Error creating directory: %v\n", err)
		os.Exit(1)
	}

	if err := crypto.SavePrivateKey(privKey, privPath); err != nil {
		fmt.Fprintf(os.Stderr, "  [!] Error saving private key: %v\n", err)
		os.Exit(1)
	}

	if err := crypto.SavePublicKey(pubKey, pubPath); err != nil {
		fmt.Fprintf(os.Stderr, "  [!] Error saving public key: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  [+] Private key: %s\n", privPath)
	fmt.Printf("  [+] Public key:  %s\n", pubPath)
	fmt.Println()
	fmt.Println("  [*] Embed the public key into agents at compile time.")
	fmt.Println("  [*] Keep the private key secure — it decrypts all agent traffic.")
	fmt.Println()
}
