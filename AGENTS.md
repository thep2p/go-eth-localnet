# Contributor Guide


## Testing Instructions
- Find the CI pipeline in the `.github/workflows` directory.
- Fix any failing test before submitting a pull request `go test ./...` should pass with no errors.
- Add or update godoc comments for any new or modified functions, types, and packages.
- Ensure that all new code is covered by tests. Use `go test -cover` to check coverage.
- Add or update tests for the code you modify or add.

### Testing Best Practices

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
- Verify `Start()`, `Ready()`, and `Done()` channels in tests
- Use `context.WithCancel()` for graceful shutdown testing
- Test context cancellation triggers proper component cleanup

**Avoiding Common Pitfalls:**
- NEVER use `select` with `time.After` directly - causes goroutine leaks in loops
- ALWAYS use test helper functions for timeouts (e.g., waiting on channels with timeout)
- NEVER skip cleanup in tests - always use `t.Cleanup()` or `defer`
- ALWAYS test both success and failure paths 

## Code Style
- Use `gofmt` to format your code before committing.
- Follow Go's idiomatic style and conventions.
- Use meaningful variable and function names.
- Keep functions small and focused on a single task.
- Use comments to explain complex logic or decisions.
- Use `godoc` comments for public functions, types, and packages.
- Use `go doc` to check your documentation.
- Title format for commit messages: `Short description (50 characters or less)`.
- Use the imperative mood in commit messages (e.g., "Fix bug" instead of "Fixed bug").
- Title format for pull requests: `Short description (50 characters or less)`.
- Don't add a PR description. The maintainers will handle that.
- Don't add any labels to the PR. The maintainers will handle that.
- Add `godoc` comments for any new tests you write, explaining what the test does and why it's necessary.
- Update `godoc` comments for any existing functions, test, types, or packages that you modify.
- Update the `README.md` file with any new features or changes you make.