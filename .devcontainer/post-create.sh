#!/usr/bin/env bash
set -euo pipefail

# The prebuilt devcontainers/go image pins GOTOOLCHAIN=local, which blocks
# `go install …@latest` whenever an upstream tool bumps its required Go past
# the image's bundled version. Flip it to auto so each tool fetches the
# toolchain it needs.
export GOTOOLCHAIN=auto

echo "==> Installing system packages..."
sudo apt-get update
# sqlite3 — local DB CLI for poking at bite.db.
# zstd    — required by Ollama's official install script.
# ffmpeg  — bite extracts video keyframes via ffmpeg for analyze_meal.
# curl    — used by the Ollama installer; usually present, ensure it.
sudo apt-get install -y --no-install-recommends \
  sqlite3 \
  zstd \
  ffmpeg \
  curl
sudo rm -rf /var/lib/apt/lists/*

if ! command -v ollama >/dev/null 2>&1; then
  echo "==> Installing Ollama..."
  curl -fsSL https://ollama.com/install.sh | sh
fi

echo "==> Installing Go tools (sqlc, goose, golangci-lint, goreleaser, air)..."
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go install github.com/pressly/goose/v3/cmd/goose@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/goreleaser/goreleaser/v2@latest
go install github.com/air-verse/air@latest

if [ -f go.mod ]; then
  echo "==> Resolving Go modules (go mod tidy)..."
  go mod tidy
fi

if [ -f sqlc.yaml ]; then
  echo "==> Generating sqlc code..."
  sqlc generate || echo "(sqlc generate failed; run manually after fixing schema)"
fi

if command -v nvidia-smi >/dev/null 2>&1; then
  echo "==> NVIDIA GPU detected — Ollama will use it automatically:"
  nvidia-smi --query-gpu=name,memory.total --format=csv,noheader || true
fi

echo
echo "Done. Try:"
echo "  cp .env.example .env && \$EDITOR .env"
echo "  go run ./cmd/bite           # opens chat TUI"
echo "  go run ./cmd/bite ask 'hi'  # one-shot"
echo "  go run ./cmd/bite web       # serves the dashboard at http://127.0.0.1:8787"
echo "  make web-dev                # serves the dashboard with live reload (air)"
