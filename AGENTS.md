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

## Code Style
- Use `gofmt` to format your code before committing.
- Follow Go's idiomatic style and conventions.
- Use meaningful variable and function names.
- Keep functions small and focused on a single task.
- Use comments to explain complex logic or decisions.
- Use `godoc` comments for public functions, types, and packages.
- Use `go doc` to check your documentation.
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