#!/bin/bash

# Security scanning script using Semgrep
# Usage: ./scripts/security-scan.sh [options]
#
# Options:
#   --json      Output JSON report
#   --sarif     Output SARIF report
#   --config    Use custom config file (.semgrep.yml)
#   --help      Show this help message

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

OUTPUT_FORMAT="text"
EXTRA_ARGS=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --json)
            OUTPUT_FORMAT="json"
            shift
            ;;
        --sarif)
            OUTPUT_FORMAT="sarif"
            shift
            ;;
        --config)
            EXTRA_ARGS="--config=$PROJECT_ROOT/.semgrep.yml"
            shift
            ;;
        --help)
            grep "^#" "$0" | grep -v "^#!/" | sed 's/^# //'
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Check if Semgrep is installed
if ! command -v semgrep &> /dev/null; then
    echo "❌ Semgrep not installed"
    echo ""
    echo "Install Semgrep:"
    echo "  pip install semgrep"
    echo "  # or"
    echo "  brew install semgrep"
    exit 1
fi

echo "🔍 Running Semgrep security analysis..."
echo ""

cd "$PROJECT_ROOT"

# Build Semgrep command
SEMGREP_CMD="semgrep \
  --config=p/security-audit \
  --config=p/owasp-top-ten \
  --config=p/cwe-top-25 \
  --config=p/xss \
  --config=p/secrets"

if [[ -n "$EXTRA_ARGS" ]]; then
    SEMGREP_CMD="$SEMGREP_CMD $EXTRA_ARGS"
fi

case $OUTPUT_FORMAT in
    json)
        SEMGREP_CMD="$SEMGREP_CMD --json --output=semgrep-report.json"
        echo "Output: semgrep-report.json"
        ;;
    sarif)
        SEMGREP_CMD="$SEMGREP_CMD --sarif --output=semgrep-report.sarif"
        echo "Output: semgrep-report.sarif"
        ;;
    *)
        SEMGREP_CMD="$SEMGREP_CMD --verbose"
        ;;
esac

echo "Command: $SEMGREP_CMD"
echo ""

# Run Semgrep
eval "$SEMGREP_CMD" || true

echo ""
echo "✅ Security scan complete"
