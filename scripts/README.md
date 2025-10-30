# Development Scripts

This directory contains utility scripts for BGP4mesh development.

## Available Scripts

### `install-hooks.sh`

Installs Git pre-commit hooks for automatic code quality checks.

**Usage:**
```bash
./scripts/install-hooks.sh
```

**What it does:**
- Copies pre-commit hook to `.git/hooks/pre-commit`
- Makes the hook executable
- Configures automatic checks before each commit

**Pre-commit checks:**
1. Go code formatting (`gofmt -s`)
2. Go static analysis (`go vet`)
3. Unit tests (`make test-unit`)

**First-time setup:**
```bash
# After cloning the repository
cd BGP4mesh
./scripts/install-hooks.sh
```

**Benefits:**
- ✅ Prevents CI failures due to formatting errors
- ✅ Catches issues locally before pushing
- ✅ Ensures consistent code quality
- ✅ Saves time by failing fast

## Notes

- Git hooks are not versioned (stored in `.git/hooks/`)
- Each developer needs to run `install-hooks.sh` once after cloning
- Hooks can be bypassed with `git commit --no-verify` (not recommended)

For more information, see the "Git Hooks & Pre-Commit Checks" section in `CLAUDE.md`.
