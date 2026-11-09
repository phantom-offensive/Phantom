#!/bin/bash
# Phantom C2 — Caddy Redirector Setup (Docker)
# Usage: ./setup.sh [C2_DOMAIN] [C2_UPSTREAM]

set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DOMAIN=${1:-}
UPSTREAM=${2:-172.17.0.1:8080}

echo "[*] Phantom C2 Docker Redirector Setup (Caddy)"

# Bootstrap .env from the example if missing
if [ ! -f "$SCRIPT_DIR/.env" ]; then
  if [ -z "$DOMAIN" ]; then
    echo "[!] No .env found. Usage: ./setup.sh your-domain.com [upstream]"
    exit 1
  fi
  printf 'C2_DOMAIN=%s\nC2_UPSTREAM=%s\n' "$DOMAIN" "$UPSTREAM" > "$SCRIPT_DIR/.env"
  echo "[+] Created .env (C2_DOMAIN=$DOMAIN, C2_UPSTREAM=$UPSTREAM)"
else
  echo "[+] Using existing .env"
fi

# Create the decoy site directory if missing
mkdir -p "$SCRIPT_DIR/site"

echo "[*] Starting the redirector container..."
cd "$SCRIPT_DIR"
docker compose down 2>/dev/null || true
docker compose up -d

echo ""
echo "[+] Redirector up — Caddy will obtain the Let's Encrypt certificate for www.$DOMAIN automatically."
echo "[+] C2 profile paths (/api/v1/*, /cdn-cgi/*, /common/oauth2/*, /app/*) proxy to $UPSTREAM."
echo "[+] All other requests get the static decoy from ./site."
echo ""
echo "[*] Test chain (C2 must be reachable at the upstream):"
echo "    curl -v https://www.$DOMAIN/api/v1/auth"
echo "    curl -v https://www.$DOMAIN/       (decoy)"
echo ""
echo "[*] Container logs:"
docker compose logs --tail=20
