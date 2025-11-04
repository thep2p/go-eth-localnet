# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-native tool for spinning up and orchestrating local Ethereum networks composed of Geth nodes. The project avoids external tooling like shell scripts, Docker Compose, or YAML files, providing pure Go orchestration for local Ethereum development and testing.

## Essential Commands

### Build and Development
```bash
make build          # Build the project (validates Go version 1.23.10+)
make test           # Run all tests (requires solc installed)
make lint           # Run golangci-lint v1.64.5
make lint-fix       # Run linting with auto-fixes
make tidy           # Tidy go modules
make install-tools  # Install required tools (golangci-lint)
```

### Testing Individual Components
```bash
go test ./internal/node/... -v           # Test node management
go test ./internal/contracts/... -v      # Test contract compilation
go test -run TestSpecificFunction -v     # Run specific test
```

## Architecture and Key Patterns

### Core Architecture
The codebase follows a single-node development mode architecture with these key components:

1. **Node Manager** (`internal/node/manager.go`) - Orchestrates the Geth node lifecycle, handling startup, shutdown, and RPC readiness checks. Uses context-based cancellation for graceful shutdown.

2. **Launcher** (`internal/node/geth.go`) - Creates and configures Geth nodes with simulated beacon for block production. Embeds nodes in-process using go-ethereum library directly.

3. **Configuration Model** (`internal/model/config.go`) - Defines node configuration including ports, data directories, and network settings. Uses chain ID 1337 for local development.

### Important Implementation Details

**Port Management Pattern:**
- Test utilities allocate unique ports using `testutils.NewPort()` to prevent conflicts
- Port allocation uses TCP listeners to find available ports
- Global tracking prevents race conditions in parallel tests

**Node Lifecycle Management:**
- Nodes must pass RPC readiness checks within 5 seconds of startup
- Context cancellation triggers graceful shutdown
- Data directories are managed with explicit cleanup in tests

**Component Lifecycle Pattern (from skipgraph-go):**
- Components use `component.Manager` for lifecycle coordination
- Manager handles Start/Ready/Done signaling automatically
- Startup logic provided via `component.WithStartupLogic(func(ctx ThrowableContext) {})`
- Shutdown logic provided via `component.WithShutdownLogic(func() {})`
- Parent components wait for all children before signaling ready/done
- Each component can only be started once (enforced by panic)
- Example:
  ```go
  type MyComponent struct {
      *component.Manager
  }

  func NewMyComponent() *MyComponent {
      c := &MyComponent{}
      c.Manager = component.NewManager(logger,
          component.WithStartupLogic(c.initialize),
          component.WithShutdownLogic(c.cleanup),
      )
      return c
  }
  ```

**Smart Contract Integration:**
- Contracts are compiled using `solc` command directly (minimum version 0.8.30)
- Compilation results are parsed from combined JSON output
- Contract deployment tests validate the full workflow

### Testing Patterns

Tests follow a consistent pattern using helper functions:
```go
func startNode(t *testing.T, opts ...node.LaunchOption) (context.Context, context.CancelFunc, *node.Manager) {
    // Creates temp directory with automatic cleanup
    // Allocates unique port
    // Starts node with context cancellation
    // Returns manager for RPC operations
}
```

### Test Organization Convention

**Directory Structure:**
```
internal/
  └── unittest/          # All test utilities and mocks
      ├── mocks/        # Auto-generated mocks (via mockery)
      ├── port.go       # Port allocation helpers
      ├── tempdir.go    # Temporary directory helpers
      ├── logger.go     # Test logger configuration
      └── ...           # Other test helpers
```

**Key test utilities in `internal/unittest/`:**
- `NewTempDir()` - Creates directories with t.Cleanup() registration
- `NewPort()` - Thread-safe unique port allocation
- `Logger()` - Test-specific logger configuration
- RPC helpers for balance checks and transaction operations

**Generated mocks in `internal/unittest/mocks/`:**
- All interface mocks auto-generated via mockery
- Import as: `"github.com/thep2p/go-eth-localnet/internal/unittest/mocks"`
- Reference as: `mocks.NewMockClient(t)`

### Mocking with testify.Mock

**Generated Mocks**: Interface mocks are generated using `mockery` and `testify/mock` for behavior-driven testing.

**Mock Generation:**
```bash
# Install mockery
go install github.com/vektra/mockery/v2@latest

# Generate mocks (configured in .mockery.yaml)
mockery
```

**Using Generated Mocks:**
```go
func TestComponent(t *testing.T) {
    mockClient := mocks.NewMockClient(t)

    // Set expectations
    mockClient.EXPECT().
        BeaconEndpoint().
        Return("http://localhost:4000").
        Once()

    // Use mock in test
    endpoint := mockClient.BeaconEndpoint()
    require.Equal(t, "http://localhost:4000", endpoint)
}
```

**Mock Best Practices:**
- Use `mockery` to auto-generate mocks from interfaces - never write mocks manually
- Generated mocks live in `internal/unittest/mocks/` alongside other test helpers
- Test files use `package <package>_test` (e.g., `consensus_test`) to prevent circular imports
- Use `EXPECT()` method for fluent assertion syntax
- Always pass `*testing.T` to mock constructors for automatic verification
- Set expectations BEFORE calling the mocked methods
- Use `.Once()`, `.Times(n)`, or `.Maybe()` to specify call expectations
- Use `.Return()` to specify return values
- Use `.Run()` to execute custom logic during mock calls

**Avoiding Import Cycles:**
- Mocks live in `unittest/mocks` which imports packages being mocked
- Tests in `package <name>_test` can import both the package AND `unittest/mocks`
- This breaks the cycle: `unittest/mocks -> consensus <- consensus_test -> unittest/mocks`
- All test-related code (helpers + mocks) lives under `internal/unittest/`

## Development Workflow

**IMPORTANT: Follow all coding best practices listed in AGENTS.md**

1. **Before making changes:**
   - Check Go version meets minimum 1.23.10 requirement
   - Ensure `solc` is installed (version 0.8.30+)
   - Run `make tidy` to ensure dependencies are clean

2. **When modifying node management:**
   - Review `internal/node/manager.go` for lifecycle patterns
   - Maintain context-based cancellation patterns
   - Ensure RPC readiness checks are preserved

3. **When adding new features:**
   - Follow the LaunchOption pattern for configuration
   - Add corresponding tests with proper cleanup
   - Use testutils for port and directory management

4. **Before committing:**
   - Run `make lint` to ensure code quality
   - Run `make test` to verify all tests pass
   - Ensure new code follows existing patterns
   - Add or update godoc comments for new/modified functions
   - Use `gofmt` to format code before committing
   - Follow Go's idiomatic style and conventions

## Key Dependencies

- `github.com/ethereum/go-ethereum v1.15.11` - Core Ethereum implementation
- `github.com/rs/zerolog v1.34.0` - Structured logging throughout
- `github.com/stretchr/testify v1.10.0` - Test assertions and requirements

## Current Limitations

- Multi-node support is available for testing and development
- Production-grade multi-node orchestration features are still in development
- Uses simulated beacon instead of full consensus layer
- Docker mode not yet implemented (in-process only)