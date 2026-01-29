# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-native tool for spinning up and orchestrating local Ethereum networks composed of Geth nodes. The project avoids external tooling like shell scripts, Docker Compose, or YAML files, providing pure Go orchestration for local Ethereum development and testing.

## Essential Commands

### Build and Development

**Go Version Requirements:**
- Minimum: Go 1.23.10+ for base functionality
- Consensus layer features: Go 1.24.0+ (required by Prysm v5.3.3)

```bash
make build          # Build the project (validates Go version)
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

**Logging and Error Messages:**
- **CRITICAL: All log messages and error messages MUST be lowercase only**
- No uppercase letters are allowed in log messages (`.Msg()`, `.Msgf()`)
- No uppercase letters are allowed in error messages (`fmt.Errorf()`, `errors.New()`)
- This applies to all logger calls (`logger.Info()`, `logger.Error()`, `logger.Warn()`, etc.)
- This applies to all test logging (`t.Log()`, `t.Fatal()`, `t.Error()`, etc.)
- Example: Use `logger.Info().Msg("node started")` not `logger.Info().Msg("Node started")`
- Example: Use `fmt.Errorf("failed to start node: %w", err)` not `fmt.Errorf("Failed to start node: %w", err)`

**Type Design - Avoid Primitive Obsession and Thin Wrappers:**
- **CRITICAL: Use Ethereum's native types instead of primitives or thin wrappers**
- NEVER define custom types for concepts already in go-ethereum
- Use `common.Hash` for 32-byte hashes, NOT `[32]byte`
- Use `common.Address` for Ethereum addresses, NOT `[20]byte`
- Use `*big.Int` for balances/amounts, NOT custom wrapper types
- Use go-ethereum's transaction types directly, NOT custom wrappers
- Example of CORRECT usage:
  ```go
  GenesisRoot common.Hash  // NOT [32]byte
  FeeRecipient common.Address  // NOT custom type
  ```
- Example of INCORRECT usage (DO NOT DO THIS):
  ```go
  type Hash [32]byte  // Unnecessary - use common.Hash
  type ExecutionHeader struct { BlockHash common.Hash }  // Thin wrapper - just use the fields directly
  ```
- Only create new types when adding meaningful business logic or domain constraints
- When in doubt, use the upstream Ethereum types directly

**Port Management Pattern:**
- Test utilities allocate unique ports using `unittest.NewPort()` to prevent conflicts
- Port allocation uses TCP listeners to find available ports
- Global tracking prevents race conditions in parallel tests

**Node Lifecycle Management:**
- Nodes must pass RPC readiness checks within 5 seconds of startup
- Context cancellation triggers graceful shutdown
- Data directories are managed with explicit cleanup in tests

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

**Test Package Naming:**
- **CRITICAL: All test files (ending in `_test.go`) MUST use `_test` package suffix**
- Test files should use black-box testing by importing the package under test
- Example: tests for `package prysm` should use `package prysm_test`
- Example: tests for `package node` should use `package node_test`
- This enforces testing the public API and prevents tight coupling to internals
- Only exception: when testing unexported functions, use the same package name

**Test Timeouts:**
- **CRITICAL: Use defined timeout constants instead of hardcoded values**
- For component lifecycle (Ready/Done): use `ReadyDoneTimeout` (10 seconds)
- For node startup: use `node.StartupTimeout` (5 seconds)
- For RPC operations: use `node.OperationTimeout` (5 seconds)
- Example: Use `time.After(prysm.ReadyDoneTimeout)` not `time.After(30 * time.Second)`
- This ensures consistent timeout behavior across all tests

**Testing Component Lifecycle:**
- **CRITICAL: Use `unittest` helpers instead of ad-hoc select statements**
- NEVER write manual select statements to check channels with `time.After`
- Available helpers in `internal/unittest/`:
  - `RequireCallMustReturnWithinTimeout(t, func(), timeout, msg)` - Fails if function doesn't return in time
  - `ChannelMustCloseWithinTimeout(t, chan, timeout, msg)` - Fails if channel doesn't close in time
- Example of CORRECT pattern:
  ```go
  client.Start(ctx)
  unittest.RequireCallMustReturnWithinTimeout(t, func() {
      <-client.Ready()
  }, node.StartupTimeout, "client ready")

  cancel()
  unittest.ChannelMustCloseWithinTimeout(t, client.Done(), node.StartupTimeout, "client done")
  ```
- Example of INCORRECT pattern (DO NOT USE):
  ```go
  select {
  case <-client.Ready():
      t.Log("ready")
  case <-time.After(timeout):
      t.Fatal("not ready")
  }
  ```
- **Note:** When implementing Component lifecycle patterns from `github.com/thep2p/skipgraph-go`, that dependency will be added and `skipgraphtest.RequireAllReady/RequireAllDone` helpers will become available.

**Directory Structure:**
```
internal/
  └── unittest/          # All test utilities
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

## Core Design Principles

### Simplicity Over Abstraction

**CRITICAL: Question every abstraction. Less is more.**

The codebase prioritizes directness and simplicity over premature abstraction. Follow these principles:

**1. Avoid Thin Wrapper Functions**
- NEVER create functions that just return constants or wrap simple operations
- If a function doesn't add meaningful business logic or domain constraints, inline it
- Example of INCORRECT (thin wrapper - DO NOT DO THIS):
  ```go
  // Bad: Just returns a constant
  func MinGenesisActiveValidatorCount() uint64 {
      return 64
  }

  // Bad: Just wraps a simple calculation
  func DefaultGenesisTime() time.Time {
      return time.Now().Add(-30 * time.Second)
  }
  ```
- Example of CORRECT (inline the logic):
  ```go
  // Good: Use the value directly
  const minValidators = 64

  // Good: Inline simple calculations with explanatory comments
  cfg := consensus.Config{
      // Genesis time 30 seconds in past for immediate block production
      GenesisTime: time.Now().Add(-30 * time.Second),
  }
  ```

**2. Avoid Thin Wrapper Types**
- NEVER create structs that just wrap other types without adding business logic
- If a type doesn't add domain constraints or behavior, don't create it
- Example of INCORRECT (thin wrapper type - DO NOT DO THIS):
  ```go
  // Bad: Adds no value over using the fields directly
  type GenesisConfig struct {
      ChainID     uint64
      GenesisTime time.Time
      Validators  []ValidatorConfig
  }

  // Bad: Just wraps existing types
  type ValidatorConfig struct {
      PrivateKey string
  }
  ```
- Example of CORRECT (use existing types):
  ```go
  // Good: Use consensus.Config directly, add fields to existing type if needed
  validatorKeys, _ := prysm.GenerateValidatorKeys(2)
  cfg := consensus.Config{
      ChainID:       1337,
      GenesisTime:   time.Now().Add(-30 * time.Second),
      ValidatorKeys: validatorKeys,
  }
  ```

**3. Use Existing Ecosystem Types**
- ALWAYS prefer types from go-ethereum and other established libraries
- NEVER reinvent types that already exist in the ecosystem
- See "Type Design - Avoid Primitive Obsession" section above for Ethereum-specific types
- When integrating with libraries, use their types directly

**4. Don't Over-Validate Test Helpers**
- Test helper functions don't need defensive validation
- Trust Go's runtime - let invalid inputs panic naturally
- Example of INCORRECT (over-validation in test helper):
  ```go
  // Bad: Unnecessary validation in test helper
  func GenerateTestValidators(count int) ([]string, error) {
      if count <= 0 {
          return nil, fmt.Errorf("count must be positive")
      }
      if count > 1000 {
          return nil, fmt.Errorf("count too large")
      }
      // ...
  }
  ```
- Example of CORRECT (trust the caller):
  ```go
  // Good: Let Go handle invalid input naturally
  func GenerateTestValidators(count int) ([]string, error) {
      keys := make([]string, count)  // Panics on negative, returns empty on 0
      // ...
  }
  ```

**5. Prefer Direct Implementation Over Abstraction**
- Don't create abstractions "just in case" for future flexibility
- Write simple, direct code that solves the current problem
- Refactor to add abstractions only when you have 3+ concrete use cases
- Example: Instead of creating a `TimeProvider` interface "for testing", just use `time.Now()` directly

**6. Use Existing Tools Over Reimplementation**
- ALWAYS check if a tool/helper already exists before writing your own
- Leverage test utilities from `internal/unittest/` (e.g., `RequireCallMustReturnWithinTimeout`, `ChannelMustCloseWithinTimeout`)
- Don't reinvent patterns that are already solved in the codebase or dependencies

### When to Create Abstractions

Create new types/functions ONLY when they:
1. **Add domain constraints**: e.g., `type Port int` with validation that ensures valid port range
2. **Add business logic**: e.g., `type Balance struct` with methods for currency conversion
3. **Enforce invariants**: e.g., `type NonEmptyString string` that guarantees non-empty values
4. **Simplify complex operations**: e.g., a function that coordinates multiple steps with error handling

If your abstraction doesn't meet at least one of these criteria, don't create it.

### Complete Implementation Over Placeholder Code

**CRITICAL: NEVER implement features as stubs, placeholders, or TODOs.**

This is the most important development principle in this project:

**THE RULE: Implement each issue from 0% to 100% with ZERO placeholders.**

**Why placeholder-driven development is WRONG:**
1. **Impossible to evaluate**: Can't verify if the design actually works
2. **Hidden complexity**: Problems only surface when implementing later
3. **Immediate technical debt**: Creates work that should have been done now
4. **False progress**: Looks complete but nothing actually works
5. **Wasted abstractions**: Interfaces designed without real implementation often need redesign

**Anti-pattern (NEVER DO THIS):**
```go
// Bad: Issue #45 - "Implement Prysm Client"
type Client struct {
    // TODO(#46): Implement beacon node
    beaconNode interface{}

    // TODO(#47): Implement validator
    validator interface{}
}

func (c *Client) Start(ctx context.Context) error {
    // TODO(#48): Implement startup
    return fmt.Errorf("not yet implemented")
}

func (c *Client) GetSyncStatus() (SyncStatus, error) {
    // TODO(#49): Implement sync status
    return SyncStatus{}, fmt.Errorf("not yet implemented")
}
```

**Problems with the above:**
- ✗ Everything is a TODO - nothing works
- ✗ Can't test real integration
- ✗ Can't verify design decisions
- ✗ Interface might be wrong but won't know until #48
- ✗ Breaking changes in #46-49 will invalidate #45

**Correct approach (DO THIS):**
```go
// Good: Issue #45 - "Implement basic Prysm beacon node startup"
// Scope: Start beacon node, wait for ready, check sync status
// NOT in scope: Validators, checkpoint sync (separate issues)

type Client struct {
    beaconNode *beacon.Node  // Real type, not interface{}
    config     Config
    logger     zerolog.Logger
    // ... actual fields needed
}

func (c *Client) Start(ctx context.Context) error {
    // Real implementation that works
    node, err := beacon.New(c.config.BeaconConfig)
    if err != nil {
        return fmt.Errorf("failed to create beacon node: %w", err)
    }

    if err := node.Start(ctx); err != nil {
        return fmt.Errorf("failed to start beacon node: %w", err)
    }

    c.beaconNode = node
    return nil
}

func (c *Client) GetSyncStatus() (SyncStatus, error) {
    // Real implementation with real Prysm API calls
    status, err := c.beaconNode.SyncStatus(context.Background())
    if err != nil {
        return SyncStatus{}, fmt.Errorf("failed to get sync status: %w", err)
    }

    return SyncStatus{
        HeadSlot: status.HeadSlot,
        IsSyncing: status.IsSyncing,
    }, nil
}
```

**How to scope issues correctly:**

**WRONG (horizontal slicing - one layer at a time):**
- Issue #1: Create all interfaces and types ❌
- Issue #2: Add all stub methods ❌
- Issue #3: Implement configuration ❌
- Issue #4: Implement startup ❌
- Issue #5: Implement sync status ❌

**RIGHT (vertical slicing - one complete feature at a time):**
- Issue #1: Basic beacon node lifecycle (start, stop, ready) ✅
- Issue #2: Add sync status checking ✅
- Issue #3: Add validator support ✅
- Issue #4: Add checkpoint sync ✅

**Each issue must:**
- ✅ Have a complete, working implementation
- ✅ Include full test coverage for that feature
- ✅ Be deployable and demonstrable on its own
- ✅ Not depend on future issues to be useful
- ❌ Have ZERO TODOs in the main implementation (tests can reference future enhancements)
- ❌ Have ZERO placeholder methods that return "not implemented"
- ❌ Have ZERO empty interfaces waiting for future population

**Acceptable use of TODOs:**
```go
// Acceptable: Future enhancement in tests
func TestBasicBeaconNode(t *testing.T) {
    // Test basic beacon node startup and sync status
    // Works completely as-is

    // TODO: Add test for checkpoint sync once #47 is implemented
}

// Acceptable: Known optimization for later
func (c *Client) fetchPeers() []Peer {
    // Working implementation using simple approach
    peers := c.node.GetPeers()

    // TODO: Optimize with caching once we have metrics showing it's needed
    return peers
}
```

**Unacceptable use of TODOs:**
```go
// WRONG: Core functionality placeholder
func (c *Client) Start(ctx context.Context) error {
    // TODO(#123): Implement this
    return fmt.Errorf("not implemented")
}

// WRONG: Essential method that doesn't work
func (c *Client) GetSyncStatus() (SyncStatus, error) {
    // TODO(#124): Call Prysm API
    return SyncStatus{}, nil
}
```

**Before submitting a PR, ask:**
1. Can I demo this feature working end-to-end? If no, it's not done.
2. Do all public methods have real implementations? If no, scope is too big.
3. Can someone review and understand what this does? If no, too many placeholders.
4. Would I deploy this to production for its intended scope? If no, it's incomplete.

**If the scope is too large:**
- ✅ Break into smaller, complete features
- ✅ Implement subset of functionality fully
- ❌ DON'T implement everything as stubs

### Pull Request and Testing Requirements

**CRITICAL: Pull requests MUST NOT contain:**

1. **Skipped Tests** - All tests must run and pass
   - ❌ WRONG: `t.Skip("Skipping until #123 is implemented")`
   - ❌ WRONG: `// TODO: Add test for feature X when #124 is done`
   - ✅ RIGHT: Only include tests that run and verify working functionality
   - ✅ RIGHT: If functionality isn't complete, don't add the test yet

2. **Stub Functions** - All functions must have real implementations
   - ❌ WRONG: Functions that return `nil` with TODO comments
   - ❌ WRONG: Functions that return `fmt.Errorf("not implemented")`
   - ❌ WRONG: Empty functions with only logging statements
   - ✅ RIGHT: Functions that do real work and can be tested

3. **Placeholder Fields** - All struct fields must be used
   - ❌ WRONG: `//nolint:unused // Will be used in #123`
   - ❌ WRONG: Fields declared as `interface{}` waiting for future types
   - ❌ WRONG: Fields that are never initialized or accessed
   - ✅ RIGHT: Only declare fields that are actually used in the PR

4. **Half-Implemented Features** - Features must be complete or not included
   - ❌ WRONG: Adding 5 methods where 4 are stubs and 1 works
   - ❌ WRONG: Creating infrastructure for future functionality
   - ❌ WRONG: "Configuration foundation" that configures nothing
   - ✅ RIGHT: One complete method that works end-to-end
   - ✅ RIGHT: Complete, testable, demonstrable functionality

**How to handle incomplete work:**

If you can't complete a feature in one PR:
1. **Option A (Recommended):** Scope down to what you CAN complete
   - Example: Instead of "Add Prysm client" → "Add Prysm genesis state generation"
   - Merge only the working parts, implement the rest in follow-up PRs

2. **Option B:** Document planned structure in issue description
   - Put code examples in issue #123 description as guidance
   - Don't commit the placeholder code to the main branch
   - Implement it fully when you actually work on issue #123

3. **NEVER Option C:** Don't merge half-complete code hoping to "fill it in later"
   - This creates technical debt immediately
   - Makes codebase confusing (what works vs what doesn't?)
   - Violates the core principle of this project

**Test Coverage Requirements:**

Every PR must have:
- ✅ Tests for all new functions/methods
- ✅ All tests passing (no skips, no failures)
- ✅ Tests that verify actual behavior, not just lifecycle
- ❌ No tests marked with `t.Skip()` for future features
- ❌ No test stubs that don't actually test anything

**Example of Good vs Bad PR:**

❌ **BAD PR - "Add Prysm Client Foundation"**
```
Files:
- client.go (5 stub methods with TODOs)
- client_test.go (tests pass but methods return nil)
- integration_test.go (all tests skipped)
Result: 400 LOC, 0% functional
```

✅ **GOOD PR - "Add Prysm Genesis State Generation"**
```
Files:
- genesis.go (GenerateGenesisState, DeriveGenesisRoot - both complete)
- genesis_test.go (comprehensive tests, all passing)
Result: 460 LOC, 100% functional
```

## Development Workflow

**IMPORTANT: Follow all coding best practices listed in AGENTS.md**

1. **Before making changes:**
   - Check Go version meets minimum requirement (1.23.10+ for base, 1.24.0+ for consensus layer)
   - Ensure `solc` is installed (version 0.8.30+)
   - Run `make tidy` to ensure dependencies are clean

2. **When modifying node management:**
   - Review `internal/node/manager.go` for lifecycle patterns
   - Maintain context-based cancellation patterns
   - Ensure RPC readiness checks are preserved

3. **When adding new features:**
   - Follow the LaunchOption pattern for configuration
   - Add corresponding tests with proper cleanup
   - Use unittest for port and directory management

4. **Before committing:**
   - Run `make lint` to ensure code quality
   - Run `make test` to verify all tests pass
   - Ensure new code follows existing patterns
   - Add or update godoc comments for new/modified functions (see Function Documentation Guidelines below)
   - Use `gofmt` to format code before committing
   - Follow Go's idiomatic style and conventions

### Function Documentation Guidelines

**CRITICAL: Every function and method must have succinct documentation that:**

1. **Explains what the function does** - One sentence describing purpose
2. **Documents parameters** - What each parameter represents
3. **Documents return values** - What is returned
4. **Documents error handling** - Critical requirement:
   - If function returns error, classify errors as either:
     - **Recoverable** - errors that can be logged and retried
     - **Critical** - errors that should crash the process
   - For functions with both recoverable and critical errors:
     - Define recoverable errors as **typed errors** (using `errors.New` or custom error types)
     - Document which specific errors are recoverable
     - Document that any other error is critical

**Documentation Length Rule:**
- Documentation should NOT exceed 30% of the function's line count
- Keep documentation succinct and focused on what developers need to know

**Examples:**

```go
// Good: Clear error classification
var (
    ErrInvalidPort = errors.New("invalid port number")
    ErrPortInUse   = errors.New("port already in use")
)

// StartServer starts the HTTP server on the specified port.
//
// The port parameter specifies which port to listen on (must be > 0).
//
// Returns the running server instance or an error.
//
// Recoverable errors:
// - ErrInvalidPort: port validation failed, can retry with different port
// - ErrPortInUse: port is already bound, can retry with different port
//
// Any other error is critical and indicates a system-level failure.
func StartServer(port int) (*Server, error) {
    if port <= 0 {
        return nil, ErrInvalidPort
    }
    // ... implementation
}

// Good: All errors are critical
// ParseConfig parses the configuration file and returns the config.
//
// The configPath parameter specifies the path to the configuration file.
//
// Returns an error if the file cannot be read or parsed. All errors
// are critical and indicate the application cannot start.
func ParseConfig(configPath string) (*Config, error) {
    // ... implementation
}

// Good: No errors possible
// GetPort returns the configured port number.
func GetPort() int {
    return 8080
}
```

**Bad Examples:**

```go
// Bad: No error classification
// Start starts the component.
func (c *Component) Start() error {
    // ... what errors are recoverable? critical?
}

// Bad: Documentation too long (exceeds 30% of function body)
// This function does something really important and here's a long
// explanation of every single detail about how it works internally
// including implementation details that should be in code comments
// and historical context about why we made certain decisions...
func ShortFunction() error {
    return nil
}
```

## Key Dependencies

- `github.com/ethereum/go-ethereum v1.15.11` - Core Ethereum implementation
- `github.com/prysmaticlabs/prysm/v5 v5.3.3` - Consensus layer (beacon chain) implementation
- `github.com/go-playground/validator/v10 v10.25.0` - Configuration validation
- `github.com/rs/zerolog v1.34.0` - Structured logging throughout
- `github.com/stretchr/testify v1.10.0` - Test assertions and requirements

**Note:** Prysm v5.3.3 requires Go 1.24.0+ as a minimum version.

## Current Limitations

- Multi-node support is available for testing and development
- Production-grade multi-node orchestration features are still in development
- Uses simulated beacon instead of full consensus layer
- Docker mode not yet implemented (in-process only)