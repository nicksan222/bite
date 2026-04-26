#!/usr/bin/env bash
set -euo pipefail

# The prebuilt devcontainers/go image pins GOTOOLCHAIN=local, which blocks
# `go install …@latest` whenever an upstream tool bumps its required Go past
# the image's bundled version. Flip it to auto so each tool fetches the
# toolchain it needs.
export GOTOOLCHAIN=auto

echo "==> Installing sqlite3..."
sudo apt-get update
sudo apt-get install -y --no-install-recommends sqlite3
sudo rm -rf /var/lib/apt/lists/*

echo "==> Installing Go tools (sqlc, goose, golangci-lint, goreleaser)..."
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go install github.com/pressly/goose/v3/cmd/goose@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/goreleaser/goreleaser/v2@latest

if [ -f go.mod ]; then
  echo "==> Resolving Go modules (go mod tidy)..."
  go mod tidy
fi

if [ -f sqlc.yaml ]; then
  echo "==> Generating sqlc code..."
  sqlc generate || echo "(sqlc generate failed; run manually after fixing schema)"
fi

echo
echo "Done. Try:"
echo "  cp .env.example .env && \$EDITOR .env"
echo "  go run ./cmd/bite           # opens chat TUI"
echo "  go run ./cmd/bite ask 'hi'  # one-shot"
