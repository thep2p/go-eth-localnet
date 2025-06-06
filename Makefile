# Desired Go version
GO_DESIRED_VERSION := 1.23.10

# Dynamically detect OS (e.g., darwin, linux) and architecture (amd64, arm64)
GO_OS := $(shell uname -s | tr A-Z a-z)
GO_ARCH := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/arm64/arm64/')

# Download URL for the Go package
GO_DOWNLOAD_URL := https://golang.org/dl/go$(GO_DESIRED_VERSION).$(GO_OS)-$(GO_ARCH).tar.gz

# Go installation directory and binary path
GO_INSTALL_DIR := /usr/local/go
GO_BIN := $(shell which go)

# Hook to check Go version before running any target
.PHONY: check-go-version
check-go-version:
	@if [ -x "$(GO_BIN)" ]; then \
		VERSION=$$($(GO_BIN) version | grep -o 'go[0-9]\+\(\.[0-9]\+\)*' | sed 's/go//'); \
		if [ "$$VERSION" != "$(GO_DESIRED_VERSION)" ]; then \
			echo "⚠️  Current Go version ($$VERSION) does not match desired version ($(GO_DESIRED_VERSION))."; \
			exit 1; \
		else \
			echo "✅ Go version $(GO_DESIRED_VERSION) is already installed."; \
		fi; \
	else \
		echo "❌ Go is not installed."; \
		exit 1; \
	fi

# Install tools target with a dependency on Go version check
.PHONY: install-tools
install-tools: check-go-version
	@echo "Installing other tools..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "🔧 Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	else \
		echo "✅ golangci-lint is already installed."; \
	fi
	@echo "✅ All tools installed successfully."

# Linting target with a dependency on Go version check
.PHONY: lint
lint: check-go-version tidy
	@golangci-lint run --config ./integration/golangci-lint.yml

.PHONY: tidy
tidy: check-go-version
	@echo "Running go mod tidy..."
	@go mod tidy