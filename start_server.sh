#!/bin/bash
# Start the Phantom server from the repo root (dev convenience script).
cd "$(dirname "$0")"
exec ./build/phantom-server --config configs/server.yaml --headless --mode both
