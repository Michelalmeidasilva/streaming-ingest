#!/bin/bash

# Setup script to install git hooks
# Usage: bash scripts/setup-hooks.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
GIT_HOOKS_DIR="$PROJECT_ROOT/.git/hooks"

echo "📝 Setting up git hooks..."
echo ""

# Check if .git/hooks exists
if [[ ! -d "$GIT_HOOKS_DIR" ]]; then
    echo "❌ Error: Not in a git repository. Run from project root." >&2
    exit 1
fi

# Install pre-commit hook
if [[ -f "$GIT_HOOKS_DIR/pre-commit" ]]; then
    echo "⚠️  pre-commit hook already exists"
    read -p "Overwrite? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Skipping pre-commit hook installation"
    else
        ln -sf "../../scripts/pre-commit.sh" "$GIT_HOOKS_DIR/pre-commit"
        chmod +x "$GIT_HOOKS_DIR/pre-commit"
        echo "✅ pre-commit hook installed"
    fi
else
    ln -sf "../../scripts/pre-commit.sh" "$GIT_HOOKS_DIR/pre-commit"
    chmod +x "$GIT_HOOKS_DIR/pre-commit"
    echo "✅ pre-commit hook installed"
fi

echo ""
echo "🎉 Git hooks setup complete!"
echo ""
echo "Available hooks:"
echo "  - pre-commit: Runs tests and Semgrep security scan before commits"
echo ""
echo "To skip hooks (not recommended):"
echo "  git commit --no-verify"
