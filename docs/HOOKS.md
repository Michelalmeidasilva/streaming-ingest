# Git Hooks for streaming-ingest

Automated checks that run before commits to ensure code quality and security.

## Available Hooks

### pre-commit
Runs before each commit to validate code quality and security:
- ✅ Execute all unit tests (`go test ./internal/...`)
- 🔒 Run Semgrep security scan
- ⏱️ Timeout: 30 seconds for tests

**Behavior:**
- Commit is **blocked** if tests fail
- Commit is **blocked** if security issues are found
- Commit is **allowed** if both checks pass

## Installation

### Automatic Setup
```bash
bash scripts/setup-hooks.sh
```

### Manual Setup
```bash
ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

## Usage

### Normal Commit (with hooks)
```bash
git commit -m "your message"
# Hooks will run automatically
```

### Skip Hooks (not recommended)
```bash
git commit --no-verify -m "your message"
```

## Troubleshooting

### Hook not running
1. Check if hook is executable:
   ```bash
   ls -l .git/hooks/pre-commit
   # Should have 'x' permission
   ```

2. Reinstall hook:
   ```bash
   bash scripts/setup-hooks.sh
   ```

### Semgrep not installed
The hook will skip Semgrep checks if not installed:
```bash
pip install semgrep
# or
brew install semgrep
```

### Tests timing out
Increase timeout in `scripts/pre-commit.sh`:
```bash
go test -v ./internal/... -timeout 60s  # Change from 30s to 60s
```

## What Each Check Does

### Unit Tests
- Runs all tests in `internal/` packages
- Verifies 81.7% code coverage
- Ensures no regressions

### Semgrep Security Scan
- Checks for hardcoded credentials
- Detects injection vulnerabilities
- Identifies OWASP Top 10 issues
- Validates secure coding practices

## Contributing

When adding new code:
1. Ensure tests pass locally: `go test -v ./internal/...`
2. Run Semgrep locally: `bash scripts/security-scan.sh`
3. Commit (hooks will validate automatically)

## Performance

Expected hook execution time:
- **Tests:** 15-20 seconds (cached)
- **Semgrep:** 2-3 seconds
- **Total:** ~20-25 seconds per commit

## Related Documentation

- [Testing Guide](README_TESTS.md) - How to run tests manually
- [Security Documentation](SECURITY.md) - Security scanning details
- [Repository Structure](../SPEC.md) - Project overview
