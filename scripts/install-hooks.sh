#!/bin/bash
# Install git hooks for BGP4mesh project
# This script copies pre-commit hooks to .git/hooks/

set -e

echo "🔧 Installing Git hooks for BGP4mesh..."

# Get the project root directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"
HOOKS_DIR="$PROJECT_ROOT/.git/hooks"

# Check if we're in a git repository
if [ ! -d "$PROJECT_ROOT/.git" ]; then
    echo "❌ Error: Not in a git repository"
    echo "   Make sure you're running this from the BGP4mesh project directory"
    exit 1
fi

# Create hooks directory if it doesn't exist
mkdir -p "$HOOKS_DIR"

# Install pre-commit hook
echo "  → Installing pre-commit hook..."
cat > "$HOOKS_DIR/pre-commit" << 'EOF'
#!/bin/bash
# Pre-commit hook for BGP4mesh project
# Prevents commits with formatting issues, vet errors, or failing tests

set -e

echo "🔍 Running pre-commit checks..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track if we're in the daemon-go directory
DAEMON_GO_DIR="daemon-go"

# Check if daemon-go exists (we might be in a subdirectory)
if [ ! -d "$DAEMON_GO_DIR" ]; then
    # Try to find it from project root
    if [ -d "../daemon-go" ]; then
        DAEMON_GO_DIR="../daemon-go"
    elif [ -d "../../daemon-go" ]; then
        DAEMON_GO_DIR="../../daemon-go"
    else
        echo -e "${YELLOW}⚠️  daemon-go directory not found, skipping Go checks${NC}"
        exit 0
    fi
fi

# Change to daemon-go directory
cd "$DAEMON_GO_DIR"

# 1. Check Go formatting
echo -n "  → Checking Go formatting... "
UNFORMATTED=$(gofmt -s -l . 2>&1)
if [ -n "$UNFORMATTED" ]; then
    echo -e "${RED}✗${NC}"
    echo -e "${RED}Error: The following files are not properly formatted:${NC}"
    echo "$UNFORMATTED"
    echo ""
    echo -e "${YELLOW}Fix with: cd daemon-go && gofmt -s -w .${NC}"
    exit 1
fi
echo -e "${GREEN}✓${NC}"

# 2. Run go vet
echo -n "  → Running go vet... "
if ! make vet > /dev/null 2>&1; then
    echo -e "${RED}✗${NC}"
    echo -e "${RED}Error: go vet found issues${NC}"
    make vet
    exit 1
fi
echo -e "${GREEN}✓${NC}"

# 3. Run unit tests (quick)
echo -n "  → Running unit tests... "
if ! make test-unit > /dev/null 2>&1; then
    echo -e "${RED}✗${NC}"
    echo -e "${RED}Error: Unit tests failed${NC}"
    echo ""
    echo "Running tests with verbose output:"
    make test-unit
    exit 1
fi
echo -e "${GREEN}✓${NC}"

echo ""
echo -e "${GREEN}✅ All pre-commit checks passed!${NC}"
echo ""

exit 0
EOF

chmod +x "$HOOKS_DIR/pre-commit"

echo "  ✅ Pre-commit hook installed"
echo ""
echo "✨ Git hooks installed successfully!"
echo ""
echo "The pre-commit hook will now run automatically before each commit to:"
echo "  • Check Go code formatting (gofmt)"
echo "  • Run go vet for code issues"
echo "  • Run unit tests"
echo ""
echo "To skip the hook temporarily (not recommended), use:"
echo "  git commit --no-verify"
echo ""
