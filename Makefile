# Makefile for ModelScan SDK Development

.PHONY: all build test lint fix clean install coverage bench help

# Default target
all: fix test lint

# Help target
help:
	@echo "ModelScan SDK Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  make all       - Fix, test, and lint all SDKs"
	@echo "  make build     - Build all SDKs"
	@echo "  make test      - Run tests for all SDKs"
	@echo "  make lint      - Lint all SDKs"
	@echo "  make fix       - Auto-fix formatting and deps"
	@echo "  make coverage  - Generate test coverage report"
	@echo "  make bench     - Run benchmarks"
	@echo "  make clean     - Clean build artifacts"
	@echo "  make install   - Install dev tools"

# Build all SDKs
build:
	@echo "Building all SDKs..."
	@cd sdk && for dir in */; do \
		echo "Building $$dir..."; \
		cd $$dir && go build ./... && cd ..; \
	done

# Test all SDKs
test:
	@echo "Running tests..."
	@./test-all-sdks.sh

# Lint all SDKs
lint:
	@echo "Linting..."
	@./lint-all-sdks.sh

# Auto-fix formatting
fix:
	@echo "Auto-fixing..."
	@./fix-all-sdks.sh

# Generate coverage report
coverage:
	@echo "Generating coverage report..."
	@mkdir -p coverage
	@cd sdk && for dir in */; do \
		if [ -f "$$dir"/*_test.go ]; then \
			echo "Coverage for $$dir..."; \
			cd $$dir && go test -coverprofile=../../coverage/$$dir.out ./... 2>/dev/null || true; \
			cd ..; \
		fi; \
	done
	@echo "Coverage reports generated in ./coverage/"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	@cd sdk && for dir in */; do \
		if [ -f "$$dir"/*_test.go ]; then \
			echo "Benchmarking $$dir..."; \
			cd $$dir && go test -bench=. -benchmem ./... || true; \
			cd ..; \
		fi; \
	done

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@cd sdk && for dir in */; do \
		cd $$dir && go clean && cd ..; \
	done
	@rm -rf coverage/

# Install development tools
install:
	@echo "Installing development tools..."
	@echo "Note: golangci-lint and staticcheck require separate installation"
	@echo "  golangci-lint: https://golangci-lint.run/usage/install/"
	@echo "  staticcheck: go install honnef.co/go/tools/cmd/staticcheck@latest"

# Quick check (fast test)
quick:
	@echo "Quick check..."
	@./fix-all-sdks.sh
	@cd sdk && for dir in */; do \
		echo "Checking $$dir..."; \
		cd $$dir && go vet ./... && gofmt -l . && cd ..; \
	done
	@echo "✓ Quick check passed"

# CI/CD target (what CI should run)
ci: fix test lint
	@echo "✅ CI checks passed"
