# Contributor Guide


## Testing Instructions
- Find the CI pipeline in the `.github/workflows` directory.
- Fix any failing test before submitting a pull request `go test ./...` should pass with no errors.
- Add or update godoc comments for any new or modified functions, types, and packages.
- Ensure that all new code is covered by tests. Use `go test -cover` to check coverage.
- Add or update tests for the code you modify or add.

### Testing Best Practices

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

**Test Organization Convention:**
```
internal/unittest/          # All test utilities and mocks
  ├── mocks/               # Auto-generated mocks (mockery)
  │   ├── mock_Client.go
  │   └── mock_Launcher.go
  ├── port.go              # Port allocation
  ├── tempdir.go           # Directory helpers
  ├── logger.go            # Logger configuration
  └── ...                  # Other test utilities
```

**Interface Mocking:**
- ALWAYS use `mockery` to generate interface mocks - never write them manually
- Generated mocks live in `internal/unittest/mocks/` with package name `mocks`
- Generated mocks use `testify/mock` for expectation-based testing
- Pass `*testing.T` to mock constructors for automatic expectation verification
- Use `EXPECT()` method for fluent, readable test assertions
- Import mocks as: `"github.com/thep2p/go-eth-localnet/internal/unittest/mocks"`
- Reference as: `mocks.NewMockClient(t)`

**Test Helpers:**
- Use test utilities in `internal/unittest/` for common operations
- `NewTempDir()` - Creates temp directories with automatic cleanup via `t.Cleanup()`
- `NewPort()` - Thread-safe unique port allocation to prevent test conflicts
- `Logger()` - Test-specific logger configuration
- All test-related code (helpers + mocks) centralizes under `internal/unittest/`

**Component Lifecycle Testing:**
- Test components implement the `modules.Component` lifecycle pattern
- **CRITICAL: Use `skipgraphtest.RequireAllReady` and `skipgraphtest.RequireAllDone` instead of ad-hoc select statements**
- NEVER write manual select statements to check `Ready()` or `Done()` channels
- These helpers provide better error messages and consistent timeout handling
- The helpers use a default timeout internally, so no timeout parameter is needed
- Use `context.WithCancel()` for graceful shutdown testing
- Test context cancellation triggers proper component cleanup
- Example of CORRECT pattern:
  ```go
  client.Start(ctx)
  skipgraphtest.RequireAllReady(t, client)

  mockCtx.Cancel()
  skipgraphtest.RequireAllDone(t, client)
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
- For multiple components, pass them all to the helper:
  ```go
  skipgraphtest.RequireAllReady(t, client1, client2, client3)
  ```

**Avoiding Common Pitfalls:**
- NEVER use `select` with `time.After` directly - causes goroutine leaks in loops
- NEVER write ad-hoc select statements for component Ready/Done - use `skipgraphtest.RequireAllReady` and `skipgraphtest.RequireAllDone`
- ALWAYS use test helper functions for timeouts (e.g., waiting on channels with timeout)
- NEVER skip cleanup in tests - always use `t.Cleanup()` or `defer`
- ALWAYS test both success and failure paths

**Type Design - Avoid Primitive Obsession:**
- **CRITICAL: Use Ethereum's native types instead of primitives or thin wrappers**
- NEVER define custom types for concepts already in go-ethereum (`common.Hash`, `common.Address`, etc.)
- Use `common.Hash` for 32-byte hashes, NOT `[32]byte`
- Use `common.Address` for Ethereum addresses, NOT `[20]byte`
- Use `*big.Int` for balances/amounts, NOT custom wrapper types
- NEVER create thin wrapper structs that add no business logic
- Only create new types when they add meaningful domain constraints or behavior
- Example of CORRECT usage:
  ```go
  GenesisRoot common.Hash        // NOT [32]byte
  FeeRecipient common.Address    // NOT custom type
  Amount *big.Int                // NOT uint64 or custom wrapper
  ```
- Example of INCORRECT usage (ANTI-PATTERN):
  ```go
  type StateRoot [32]byte        // Bad: use common.Hash instead
  type ValidatorAddress [20]byte // Bad: use common.Address instead
  type WeiAmount uint64          // Bad: use *big.Int instead
  ```
- When integrating with Ethereum libraries, use their types directly

## Design Principles: Simplicity Over Abstraction

**CRITICAL: Question every abstraction before creating it.**

This codebase values directness and simplicity. Follow these principles:

### Avoid Thin Wrapper Functions
- **NEVER create functions that just return constants or wrap simple one-liners**
- If a function doesn't add meaningful business logic, inline it
- Ask: "Does this function add domain constraints, business logic, or simplify complexity?"
- If the answer is no, don't create it

**Anti-patterns (DO NOT DO THIS):**
```go
// Bad: Just returns a constant - use the constant directly
func MinGenesisActiveValidatorCount() uint64 {
    return 64
}

// Bad: Wraps a simple calculation - inline it where needed
func DefaultGenesisTime() time.Time {
    return time.Now().Add(-30 * time.Second)
}

// Bad: Unnecessary getter - just access the field
func (c *Config) GetChainID() uint64 {
    return c.ChainID
}
```

**Correct patterns:**
```go
// Good: Use constants directly
const minGenesisValidators = 64

// Good: Inline simple logic with explanatory comments
cfg := consensus.Config{
    // Genesis time 30 seconds in past ensures immediate block production
    GenesisTime: time.Now().Add(-30 * time.Second),
}

// Good: Direct field access
if cfg.ChainID == 1337 {
    // ...
}
```

### Avoid Thin Wrapper Types
- **NEVER create structs that just group fields without adding behavior**
- If a type doesn't enforce invariants or add domain logic, don't create it
- Use existing types directly or add fields to existing structs

**Anti-patterns (DO NOT DO THIS):**
```go
// Bad: Adds no value - just use consensus.Config directly
type GenesisConfig struct {
    ChainID     uint64
    GenesisTime time.Time
    Validators  []ValidatorConfig
}

// Bad: Thin wrapper with no behavior
type ValidatorConfig struct {
    PrivateKey string
}

// Bad: Wraps existing type with no added value
type ExecutionHeader struct {
    BlockHash   common.Hash
    BlockNumber uint64
    Timestamp   uint64
}
```

**Correct patterns:**
```go
// Good: Add fields to existing consensus.Config instead
type Config struct {
    ChainID       uint64
    GenesisTime   time.Time
    ValidatorKeys []string  // Simple slice, no wrapper needed
    // ...
}

// Good: Type with domain constraints and validation
type Port int

func (p Port) Validate() error {
    if p < 1024 || p > 65535 {
        return fmt.Errorf("invalid port: %d", p)
    }
    return nil
}
```

### Don't Over-Validate Test Helpers
- Test helper functions should NOT have defensive validation
- Trust Go's runtime to handle invalid inputs naturally
- Only validate at public API boundaries, not in test utilities

**Anti-pattern (DO NOT DO THIS):**
```go
// Bad: Unnecessary validation in test helper
func GenerateTestValidators(count int) ([]string, error) {
    if count <= 0 {
        return nil, fmt.Errorf("count must be positive")
    }
    if count > 1000 {
        return nil, fmt.Errorf("count too large")
    }
    keys := make([]string, count)
    // ...
}
```

**Correct pattern:**
```go
// Good: Trust the caller, let Go handle edge cases
func GenerateTestValidators(count int) ([]string, error) {
    keys := make([]string, count)  // Panics on negative, empty slice on 0
    for i := 0; i < count; i++ {
        keys[i] = fmt.Sprintf("test-key-%d", i)
    }
    return keys, nil
}
```

### Use Existing Tools and Utilities
- **ALWAYS check if functionality already exists before implementing**
- Prefer ecosystem libraries over custom implementations
- Leverage test utilities from dependencies
- Don't reinvent patterns already solved in the codebase

**Examples:**
- Use `skipgraphtest.RequireAllReady` instead of manual select statements
- Use `common.Hash` from go-ethereum instead of `[32]byte`
- Use `testutils.NewPort()` instead of hardcoding port numbers

### When to Create Abstractions

Create new types/functions ONLY when they:
1. **Add domain constraints**: Validation logic that enforces business rules
2. **Add business logic**: Methods that perform meaningful operations
3. **Enforce invariants**: Guarantee certain properties always hold
4. **Simplify complexity**: Coordinate multiple steps with proper error handling

**Test your abstraction:**
- Does it add meaningful behavior beyond the wrapped type? ✅ Keep it
- Does it just group fields or wrap a simple operation? ❌ Remove it
- Could you inline it without losing clarity? ❌ Remove it
- Does it make the code harder to understand? ❌ Remove it

## Complete Implementation Over Placeholder Code

**CRITICAL: This is the most important principle in this project.**

### The Golden Rule

**Implement each issue/feature from 0% to 100% completion with ZERO placeholders, stubs, or TODOs in core functionality.**

### Why Placeholder Development is Unacceptable

Implementing features as stubs/placeholders/TODOs is the **worst anti-pattern** in software development:

1. **Impossible to evaluate**: Reviewers can't verify if the design actually works
2. **Hidden complexity**: Real problems only surface during later implementation when they're harder to fix
3. **Immediate technical debt**: Creates work that should have been done now
4. **False progress**: Code looks complete but nothing actually functions
5. **Wasted effort**: Abstractions designed without real implementation usually need complete redesign
6. **Blocks evaluation**: Can't test real integration or validate architectural decisions

### Anti-Pattern: Horizontal Slicing (NEVER DO THIS)

**WRONG approach - Issue #45: "Implement Prysm Client"**
```go
// Bad: Everything is a placeholder
type Client struct {
    // TODO(#46): Implement beacon node
    beaconNode interface{}

    // TODO(#47): Implement validator
    validator interface{}
}

func (c *Client) Start(ctx context.Context) error {
    // TODO(#48): Implement startup logic
    return fmt.Errorf("not yet implemented")
}

func (c *Client) Stop() error {
    // TODO(#49): Implement shutdown
    return fmt.Errorf("not yet implemented")
}

func (c *Client) GetSyncStatus() (SyncStatus, error) {
    // TODO(#50): Implement sync status check
    return SyncStatus{}, fmt.Errorf("not yet implemented")
}

// Tests also have placeholders
func TestClientStart(t *testing.T) {
    t.Skip("Skipping until #48 is implemented")
}
```

**Why this is WRONG:**
- ✗ Nothing works - entire PR is just structure
- ✗ Can't test if the design is correct
- ✗ Can't verify it integrates with Prysm
- ✗ Issues #46-50 might reveal Client interface is wrong
- ✗ Reviewer has no idea if this will actually work
- ✗ No value delivered - just promises of future work

### Correct Pattern: Vertical Slicing (DO THIS)

**RIGHT approach - Issue #45: "Implement basic Prysm beacon node lifecycle"**

**Scope:** Start beacon node, verify Ready state, check sync status, stop gracefully
**NOT in scope:** Validators, checkpoint sync, P2P configuration (separate issues)

```go
// Good: Complete implementation for defined scope
type Client struct {
    beaconNode *beacon.Node  // Real type from Prysm
    config     Config
    logger     zerolog.Logger
    ready      chan struct{}
    done       chan struct{}
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
    close(c.ready)

    go c.waitForShutdown(ctx)
    return nil
}

func (c *Client) Stop() error {
    // Real implementation
    return c.beaconNode.Shutdown()
}

func (c *Client) GetSyncStatus() (SyncStatus, error) {
    // Real implementation with actual Prysm API calls
    status, err := c.beaconNode.SyncStatus(context.Background())
    if err != nil {
        return SyncStatus{}, fmt.Errorf("failed to get sync status: %w", err)
    }

    return SyncStatus{
        HeadSlot:  status.HeadSlot,
        IsSyncing: status.IsSyncing,
    }, nil
}

// Tests work and demonstrate the feature
func TestClientLifecycle(t *testing.T) {
    client := setupTestClient(t)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    client.Start(ctx)
    skipgraphtest.RequireAllReady(t, client)

    // Actually works!
    status, err := client.GetSyncStatus()
    require.NoError(t, err)
    require.NotNil(t, status)

    cancel()
    skipgraphtest.RequireAllDone(t, client)
}
```

**Why this is RIGHT:**
- ✅ Everything works end-to-end for the defined scope
- ✅ Can demo the feature functioning
- ✅ Tests prove it integrates with Prysm correctly
- ✅ Reviewer can evaluate if the design is sound
- ✅ Delivers real value - beacon node lifecycle works
- ✅ Next issue can build on solid foundation

### Correct Issue Breakdown

**WRONG (horizontal slicing):**
```
Issue #1: Create all types and interfaces ❌
Issue #2: Add all method stubs ❌
Issue #3: Implement configuration ❌
Issue #4: Implement beacon node ❌
Issue #5: Implement validators ❌
Issue #6: Implement sync status ❌
```

**RIGHT (vertical slicing):**
```
Issue #1: Basic beacon node lifecycle (start, ready, stop, sync status) ✅
  - Scope: Minimal working beacon node
  - Delivers: Can start/stop Prysm beacon node

Issue #2: Add validator support ✅
  - Scope: Load validators, sign attestations
  - Delivers: Beacon node can run validators

Issue #3: Add checkpoint sync ✅
  - Scope: Bootstrap from checkpoint
  - Delivers: Fast sync capability

Issue #4: Add P2P peer configuration ✅
  - Scope: Bootnodes, static peers
  - Delivers: Network connectivity control
```

### Requirements for Every PR

Before submitting, verify:

**✅ MUST have:**
1. Complete working implementation for the defined scope
2. Full test coverage that actually runs and passes
3. Can demo the feature working end-to-end
4. All public methods have real implementations
5. Tests use real integrations, not mocks for everything
6. Documentation explains what works NOW

**❌ MUST NOT have:**
1. TODOs in core functionality implementation
2. Methods that return "not implemented" errors
3. Empty interface{} fields waiting for future population
4. Tests that skip with "waiting for issue #X"
5. Stub methods that do nothing
6. Placeholder types with no real fields

### Acceptable Use of TODOs

**✅ Acceptable (future enhancements):**
```go
// OK: Working implementation, noting future optimization
func (c *Client) fetchPeers() []Peer {
    // Works correctly now
    peers := c.node.GetPeers()

    // TODO: Add caching when metrics show it's needed
    return peers
}

// OK: Test works, noting future test case
func TestClientBasics(t *testing.T) {
    // Complete test that works
    // ...

    // TODO: Add test for checkpoint sync once #47 lands
}
```

**❌ Unacceptable (core functionality):**
```go
// WRONG: Core method doesn't work
func (c *Client) Start(ctx context.Context) error {
    // TODO(#123): Implement beacon node startup
    return fmt.Errorf("not implemented")
}

// WRONG: Essential functionality missing
func (c *Client) GetValidators() ([]Validator, error) {
    // TODO(#124): Query validator status from Prysm
    return nil, nil
}
```

### How to Handle Large Scope

**If you realize the scope is too large:**

✅ **DO:** Break into smaller complete features
- Issue #1: Beacon node only (validators in #2)
- Issue #2: Add validators
- Each issue delivers complete working functionality

❌ **DON'T:** Implement everything as stubs
- Issue #1: All stubs and TODOs for everything
- Issue #2-5: Fill in the TODOs
- Nothing works until issue #5

### PR Review Questions

**Before requesting review, ask yourself:**

1. **Can I demo this feature working?** If no → it's not ready
2. **Do all public methods work?** If no → scope too large, reduce it
3. **Can reviewer understand what this does?** If no → too many TODOs
4. **Would I deploy this for its intended scope?** If no → it's incomplete
5. **Can tests run without skipping?** If no → implementation incomplete

**If any answer is "no", the PR is not ready.**

### Examples from This Project

**Previous anti-pattern (what NOT to do):**
- Created `Client`, `Launcher` types with all methods as stubs
- Added TODOs for #45, #46, #47, #48, #49
- Nothing worked, everything deferred to future issues
- Impossible to evaluate if the design was correct
- Created unnecessary abstractions (GenesisConfig, ValidatorConfig)

**Correct approach (what TO do):**
- Issue #1: Implement basic launcher with real Prysm integration
- Issue #2: Add Client lifecycle with working start/stop
- Issue #3: Add validator loading that actually works
- Each issue delivers demonstrable value

## Code Style
- Use `gofmt` to format your code before committing.
- Follow Go's idiomatic style and conventions.
- Use meaningful variable and function names.
- Keep functions small and focused on a single task.
- Use comments to explain complex logic or decisions.
- Use `godoc` comments for public functions, types, and packages.
- Use `go doc` to check your documentation.

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

- **CRITICAL: All log messages and error messages MUST be lowercase only**
  - No uppercase letters in log messages (`.Msg()`, `.Msgf()`)
  - No uppercase letters in error messages (`fmt.Errorf()`, `errors.New()`)
  - Applies to all logger calls (`logger.Info()`, `logger.Error()`, `logger.Warn()`, etc.)
  - Applies to all test logging (`t.Log()`, `t.Fatal()`, `t.Error()`, etc.)
  - Example: Use `logger.Info().Msg("node started")` not `logger.Info().Msg("Node started")`
  - Example: Use `fmt.Errorf("failed to start node: %w", err)` not `fmt.Errorf("Failed to start node: %w", err)`
- Title format for commit messages: `Short description (50 characters or less)`.
- Use the imperative mood in commit messages (e.g., "Fix bug" instead of "Fixed bug").
- Title format for pull requests: `Short description (50 characters or less)`.
- Don't add a PR description. The maintainers will handle that.
- Don't add any labels to the PR. The maintainers will handle that.
- Add `godoc` comments for any new tests you write, explaining what the test does and why it's necessary.
- Update `godoc` comments for any existing functions, test, types, or packages that you modify.
- Update the `README.md` file with any new features or changes you make.