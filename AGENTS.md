# Contributor Guide


## Testing Instructions
- Find the CI pipeline in the `.github/workflows` directory.
- Fix any failing test before submitting a pull request `go test ./...` should pass with no errors.
- Add or update godoc comments for any new or modified functions, types, and packages.
- Ensure that all new code is covered by tests. Use `go test -cover` to check coverage.
- Add or update tests for the code you modify or add. 

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