.PHONY: install-tools
install-tools:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "🔧 Installing golangci-lint globally..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	else \
		echo "✅ golangci-lint is already installed"; \
	fi

.PHONY: lint
lint:
	@golangci-lint run --config ./integration/golangci-lint.yml
