#!/bin/bash

# Pre-commit hook for streaming-ingest
# Runs tests and security scans before allowing commits
# Install with: ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🔍 Running pre-commit checks..."
echo ""

# Run tests (skip integration tests - run with: make test-integration)
echo "🧪 Running unit tests..."
cd "$PROJECT_ROOT"
if ! go test -v ./internal/... -timeout 30s; then
    echo "❌ Tests failed. Commit aborted."
    exit 1
fi
echo "✅ Unit tests passed"
echo ""

# Run Semgrep security scan
echo "🔒 Running Semgrep security scan..."
if ! command -v semgrep &> /dev/null; then
    echo "⚠️  Semgrep not installed. Skipping security scan."
    echo "   Install with: pip install semgrep or brew install semgrep"
else
    if ! semgrep \
        --config=p/security-audit \
        --config=p/owasp-top-ten \
        --config=p/cwe-top-25 \
        --config=p/xss \
        --config=p/secrets \
        --config=.semgrep.yml \
        --quiet \
        .; then
        echo "❌ Security scan failed. Commit aborted."
        exit 1
    fi
    echo "✅ Security scan passed (0 findings)"
fi
echo ""

echo "✅ All pre-commit checks passed!"
exit 0
