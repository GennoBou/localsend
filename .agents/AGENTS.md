# Localsend Project Rules

## 1. Language & Comments
- All source code, API comments, and public Go Doc comments must be written in English.
- Exported functions, types, and constants must have Go Doc style comments (e.g., `// FunctionName does...`).

## 2. Code Style & Architecture
- Follow standard Go idioms and `go fmt` style guidelines.
- Prefer standard library packages unless third-party dependencies are strictly necessary. Do not add new packages to `go.mod` without explicit instruction.
- Follow the project's directory structure:
  - Private application code goes under `internal/`.
  - Reusable public library code goes under `pkg/`.
- Handle errors gracefully. Wrap errors with context using `fmt.Errorf("failed to ...: %w", err)` rather than returning them raw, unless passing them directly up the stack is appropriate.

## 3. Testing Policy
- Write unit tests for new features.
- Prefer using Table-Driven Tests for testing complex logic.
- Use the standard `testing` package along with `github.com/stretchr/testify` for assertions if already imported in the project.
