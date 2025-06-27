# Minimum Go version
GO_MIN_VERSION := 1.23.0
LINT_VERSION := v1.64.5

# Dynamically detect OS (e.g., darwin, linux) and architecture (amd64, arm64)
GO_OS := $(shell uname -s | tr A-Z a-z)
GO_ARCH := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/arm64/arm64/')

# Download URL for the Go package
GO_DOWNLOAD_URL := https://golang.org/dl/go$(GO_MIN_VERSION).$(GO_OS)-$(GO_ARCH).tar.gz

# Go installation directory and binary path
GO_INSTALL_DIR := /usr/local/go
GO_BIN := $(shell which go)

# Hook to check Go version before running any target
.PHONY: check-go-version
check-go-version:
	@if [ -x "$(GO_BIN)" ]; then \
		CURRENT=$$($(GO_BIN) version | grep -o 'go[0-9]\+\(\.[0-9]\+\)*' | sed 's/go//'); \
		DESIRED="$(GO_MIN_VERSION)"; \
		if [ "$$(printf '%s\n' "$$DESIRED" "$$CURRENT" | sort -V | head -n1)" != "$$DESIRED" ]; then \
			echo "⚠️  Current Go version ($$CURRENT) does not meet the minimum required version ($$DESIRED)."; \
			exit 1; \
		else \
			echo "✅ Current Go version ($$CURRENT) meets or exceeds the required version ($$DESIRED)."; \
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
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(LINT_VERSION); \
	else \
		VERSION=$$(golangci-lint --version --format "{{.Version}}"); \
		if [[ "$${VERSION}" != "$(LINT_VERSION)" ]]; then \
			echo "🔄 Updating/Downgrading golangci-lint to $(LINT_VERSION)..."; \
			go clean -i github.com/golangci/golangci-lint/cmd/golangci-lint; \
			go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(LINT_VERSION); \
		else \
			echo "✅ golangci-lint $(LINT_VERSION) is already installed."; \
		fi; \
	fi
	@echo "✅ All tools installed successfully."



# Linting target with a dependency on Go version check
.PHONY: lint-fix
lint: check-go-version tidy
	 @golangci-lint run --fix --config ./integration/golangci-lint.yml ./...

.PHONY: tidy
tidy: check-go-version
	@go mod tidy

.PHONY: build
build: check-go-version tidy
	@go build ./...

.PHONY: test
test: check-go-version tidy
	@go test -v ./...
