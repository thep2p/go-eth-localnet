# =========================
# Configurable Variables
# =========================

IS_CI := $(shell [ -n "$$CI" ] && echo "true" || echo "false")
GO_MIN_VERSION := 1.23.10
LINT_VERSION := v1.64.5
SOLC_VERSION := 0.8.21

GO_OS := $(shell uname -s | tr A-Z a-z)
GO_ARCH := $(shell uname -m | sed 's/x86_64/amd64/;s/arm64/arm64/;s/aarch64/arm64/')
GO_DOWNLOAD_URL := https://golang.org/dl/go$(GO_MIN_VERSION).$(GO_OS)-$(GO_ARCH).tar.gz
GO_INSTALL_DIR := /usr/local/go
GO_BIN := $(shell which go)

# =========================
# Platform-specific solc URL
# =========================

ifeq ($(GO_OS),linux)
  ifeq ($(GO_ARCH),amd64)
    SOLC_URL := https://github.com/ethereum/solidity/releases/download/v$(SOLC_VERSION)/solc-static-linux
  else ifeq ($(GO_ARCH),arm64)
    SOLC_URL := https://github.com/ethereum/solidity/releases/download/v$(SOLC_VERSION)/solc-static-linux-arm64
  else
    $(error Unsupported Linux architecture: $(GO_ARCH))
  endif
endif

# =========================
# Targets
# =========================

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

.PHONY: install-lint
install-lint: check-go-version
	@echo "Installing other tools..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "🔧 Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(LINT_VERSION); \
	else \
		VERSION=$$(golangci-lint --version --format "{{.Version}}"); \
		if [ "$${VERSION}" != "$(LINT_VERSION)" ]; then \
			echo "🔄 Updating/Downgrading golangci-lint to $(LINT_VERSION)..."; \
			go clean -i github.com/golangci/golangci-lint/cmd/golangci-lint; \
			go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(LINT_VERSION); \
		else \
			echo "✅ golangci-lint $(LINT_VERSION) is already installed."; \
		fi; \
	fi
	@echo "✅ All tools installed successfully."

.PHONY: install-tools
install-tools: check-go-version install-lint

.PHONY: lint
lint: check-go-version tidy
	@golangci-lint run --config ./integration/golangci-lint.yml ./...
	@echo "✅ Linting completed"

.PHONY: lint-fix
lint-fix: check-go-version tidy
	@golangci-lint run --fix --config ./integration/golangci-lint.yml ./...
	@echo "✅ Linting (with fix) completed"

.PHONY: tidy
tidy: check-go-version
	@go mod tidy

.PHONY: build
build: check-go-version tidy
	@go build ./...

.PHONY: test
test: check-go-version tidy
	@go test -v ./...
	@echo "✅ All tests passed"
