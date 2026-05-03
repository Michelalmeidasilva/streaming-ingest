#!/bin/bash

# Pre-commit hook for streaming-ingest
# Runs tests and security scans before allowing commits
# Install with: ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🔍 Running pre-commit checks..."
echo ""

# Run tests and check coverage
echo "🧪 Running unit tests with coverage..."
cd "$PROJECT_ROOT"
MIN_COVERAGE=80

if ! go test -v -coverprofile=coverage.out ./internal/... -timeout 30s; then
    echo "❌ Tests failed. Commit aborted."
    exit 1
fi

# Extract total coverage percentage
TOTAL_COVERAGE=$(go tool cover -func=coverage.out | grep total | grep -Eo '[0-9]+\.[0-9]+')

if [[ -z "$TOTAL_COVERAGE" ]]; then
    echo "❌ Could not determine coverage percentage. Commit aborted."
    exit 1
fi

# Check if coverage meets the minimum requirement
if (( $(echo "$TOTAL_COVERAGE < $MIN_COVERAGE" | bc -l) )); then
    echo "❌ Coverage too low: ${TOTAL_COVERAGE}% (Minimum: ${MIN_COVERAGE}%). Commit aborted."
    exit 1
fi

echo "✅ Unit tests passed with ${TOTAL_COVERAGE}% coverage"
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
