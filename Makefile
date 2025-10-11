.PHONY: help deadcode check-unused lint-all install-tools

help:
	@echo "workspaces - Make Commands"
	@echo ""
	@echo "Dead Code Detection:"
	@echo "  make deadcode        - Check for dead code (Go, TypeScript, Rust)"
	@echo "  make check-unused    - Check for unused dependencies"
	@echo "  make lint-all        - Run all linting and dead code checks"
	@echo ""
	@echo "Setup:"
	@echo "  make install-tools   - Install dead code detection tools"

install-tools:
	@echo "==> Installing dead code detection tools..."
	@echo "==> Installing Go tools..."
	go install golang.org/x/tools/cmd/deadcode@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	@echo "==> Installing npm dependencies..."
	cd app && npm install
	@echo "✓ Tools installed"

deadcode:
	@echo "==> Checking Go dead code..."
	@cd api && deadcode ./... || (echo "❌ Go dead code found" && exit 1)
	@echo "✓ No Go dead code"
	@echo ""
	@echo "==> Checking TypeScript dead code..."
	@cd app && npx knip || (echo "❌ TypeScript dead code found" && exit 1)
	@echo "✓ No TypeScript dead code"
	@echo ""
	@echo "==> Checking Rust warnings..."
	@cd app/src-tauri && cargo clippy -- -D warnings || (echo "❌ Rust warnings found" && exit 1)
	@echo "✓ No Rust warnings"
	@echo ""
	@echo "✓ All dead code checks passed"

check-unused:
	@echo "==> Checking unused npm dependencies..."
	@cd app && npx depcheck || echo "⚠ Check output above for unused dependencies"
	@echo ""
	@echo "==> Checking Go module consistency..."
	@cd api && go mod tidy && git diff --exit-code go.mod go.sum || (echo "⚠ Run 'cd api && go mod tidy' to fix" && exit 1)
	@echo "✓ Go modules are tidy"
	@echo ""
	@echo "==> Checking unused Rust dependencies (requires nightly)..."
	@cd app/src-tauri && cargo +nightly udeps 2>/dev/null || echo "⚠ Install cargo-udeps: cargo install cargo-udeps"

lint-all: deadcode check-unused
	@echo ""
	@echo "✓ All checks passed"
