# Makefile Documentation

This document describes all available `make` targets for the Tabularis Redis Plugin project.

## Quick Start

```bash
# Show all available commands
make help

# Build the plugin
make build

# Run tests
make test

# Install plugin locally
make install
```

## Container Runtime Support

The Makefile automatically detects and uses either **Docker** or **Podman** for Redis server management:

- If `docker` is available, it will be used
- If `docker` is not found but `podman` is available, it will use `podman`
- If neither is found, Redis-related targets will show an error message

You can check which container runtime is being used:
```bash
make info
# Output shows: Container Cmd: docker (or podman)
```

## Command Categories

### General

| Command | Description |
|---------|-------------|
| `make help` | Display help message with all available targets |

### Development

| Command | Description |
|---------|-------------|
| `make build` | Build the plugin binary for current platform |
| `make build-all` | Cross-compile for all platforms (Linux, macOS, Windows - amd64/arm64) |
| `make install` | Build and install plugin to Tabularis plugins directory |
| `make install-force` | Clean, build, and install plugin (useful for fresh installs) |
| `make uninstall` | Remove plugin from Tabularis plugins directory |
| `make clean` | Remove build artifacts and temporary files |
| `make run-local` | Build and run plugin locally for manual testing |

**Example Workflow:**
```bash
# Make changes to code
make build          # Build the plugin
make install        # Install to Tabularis
# Restart Tabularis to use updated plugin
```

### Testing

| Command | Description |
|---------|-------------|
| `make test` | Run unit tests (alias for `test-unit`) |
| `make test-unit` | Run unit tests with race detection |
| `make test-coverage` | Run tests with coverage report (generates `coverage.html`) |
| `make test-e2e` | Run end-to-end tests (requires Docker/Podman) |
| `make test-pubsub` | Run Pub/Sub-specific tests |
| `make test-pubsub-comprehensive` | Run comprehensive Pub/Sub test suite |
| `make test-all` | Run all tests (unit, E2E, Pub/Sub) |

**Example:**
```bash
# Quick test during development
make test

# Full test suite before release
make test-all

# Generate coverage report
make test-coverage
open coverage.html  # View in browser
```

### Redis Server Management

| Command | Description |
|---------|-------------|
| `make redis-start` | Start Redis server in Docker/Podman container |
| `make redis-stop` | Stop Redis server |
| `make redis-restart` | Restart Redis server |
| `make redis-status` | Check Redis server status |
| `make redis-cli` | Connect to Redis CLI |
| `make redis-clean` | Stop and remove Redis container |
| `make redis-logs` | Show Redis server logs (follow mode) |

**Note:** These commands automatically use Docker or Podman, whichever is available.

**Example Workflow:**
```bash
# Start Redis for testing
make redis-start

# Seed with test data
make seed-redis

# Run tests
make test-e2e

# Check logs if needed
make redis-logs

# Clean up when done
make redis-clean
```

### Code Quality

| Command | Description |
|---------|-------------|
| `make lint` | Run golangci-lint (requires installation) |
| `make fmt` | Format code with gofmt |
| `make vet` | Run go vet |
| `make check` | Run all code quality checks (fmt, vet, lint, test-unit) |

**Pre-commit Workflow:**
```bash
# Before committing
make check
```

**Installing golangci-lint:**
```bash
# macOS
brew install golangci-lint

# Linux
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Or use make dev-setup
make dev-setup
```

### Dependencies

| Command | Description |
|---------|-------------|
| `make mod-download` | Download Go module dependencies |
| `make mod-tidy` | Tidy Go module dependencies |
| `make mod-verify` | Verify Go module dependencies |
| `make mod-update` | Update all dependencies to latest versions |

**Example:**
```bash
# After adding new imports
make mod-tidy

# Update all dependencies
make mod-update
make test  # Verify everything still works
```

### Release Management

| Command | Description |
|---------|-------------|
| `make release-patch` | Create a new patch release (v0.0.X) |
| `make release-minor` | Create a new minor release (v0.X.0) |
| `make release-major` | Create a new major release (vX.0.0) |
| `make tag-push` | Push current tag to trigger GitHub Actions release workflow |
| `make tag-list` | List all git tags |

**Release Workflow:**

```bash
# 1. Ensure working directory is clean
git status

# 2. Run full test suite
make test-all

# 3. Create release (choose appropriate version bump)
make release-patch   # For bug fixes (0.1.0 → 0.1.1)
make release-minor   # For new features (0.1.0 → 0.2.0)
make release-major   # For breaking changes (0.1.0 → 1.0.0)

# 4. Push to trigger GitHub Actions
git push origin main
make tag-push

# GitHub Actions will automatically:
# - Build for all platforms
# - Run tests
# - Create GitHub release
# - Upload binaries
```

**What the release script does:**
1. Checks that git working directory is clean
2. Increments version based on bump type
3. Updates `manifest.json` with new version
4. Commits the manifest change
5. Creates an annotated git tag
6. Provides instructions for pushing

### Development Setup

| Command | Description |
|---------|-------------|
| `make dev-setup` | Setup development environment (install tools, download deps) |
| `make seed-redis` | Seed Redis with test data (requires Redis running) |

**First-time Setup:**
```bash
# Clone repository
git clone https://github.com/gzamboni/tabularis-redis-plugin-go.git
cd tabularis-redis-plugin-go

# Setup development environment
make dev-setup

# Start Redis
make redis-start

# Build and test
make build
make test

# Install locally
make install
```

### Information

| Command | Description |
|---------|-------------|
| `make version` | Show current version, commit, and build time |
| `make info` | Show project information (binary name, plugin dir, etc.) |

## Platform-Specific Notes

### macOS

The Makefile automatically detects macOS and uses the correct plugin directory:
```
~/Library/Application Support/com.debba.tabularis/plugins/redis
```

### Linux

Plugin directory:
```
~/.local/share/tabularis/plugins/redis
```

### Windows

Plugin directory:
```
%APPDATA%\com.debba.tabularis\plugins\redis
```

**Note:** On Windows, use `make` from Git Bash, WSL, or install GNU Make.

## Environment Variables

The Makefile uses these variables (can be overridden):

| Variable | Default | Description |
|----------|---------|-------------|
| `BINARY_NAME` | `tabularis-redis-plugin-go` | Name of the binary |
| `REDIS_PORT` | `6379` | Redis server port |
| `REDIS_CONTAINER` | `tabularis-redis-test` | Docker container name |
| `REDIS_IMAGE` | `redis:7-alpine` | Docker image for Redis |

**Override example:**
```bash
# Use different Redis port
make redis-start REDIS_PORT=6380

# Use different container name
make redis-start REDIS_CONTAINER=my-redis
```

## Common Workflows

### Daily Development

```bash
# 1. Start Redis
make redis-start

# 2. Make code changes
# ... edit files ...

# 3. Test changes
make test

# 4. Install locally to test in Tabularis
make install

# 5. Restart Tabularis and test

# 6. Clean up
make redis-stop
```

### Before Committing

```bash
# Format, lint, and test
make check

# If all passes, commit
git add .
git commit -m "feat: add new feature"
```

### Preparing a Release

```bash
# 1. Ensure everything is committed
git status

# 2. Run full test suite
make test-all

# 3. Create release
make release-minor

# 4. Push
git push origin main
make tag-push

# 5. Monitor GitHub Actions
# https://github.com/gzamboni/tabularis-redis-plugin-go/actions
```

### Troubleshooting

```bash
# Clean everything and rebuild
make clean
make build

# Check Redis status
make redis-status

# View Redis logs
make redis-logs

# Connect to Redis CLI to inspect data
make redis-cli
> KEYS *
> GET mykey
> exit

# Reinstall plugin
make install-force
```

## Tips

1. **Use tab completion:** Type `make` and press Tab to see available targets
2. **Chain commands:** `make clean build test install`
3. **Parallel execution:** `make -j4 test-unit test-e2e` (run tests in parallel)
4. **Dry run:** `make -n build` (show what would be executed)
5. **Silent mode:** `make -s build` (suppress make output)

## Troubleshooting

### "make: command not found"

**macOS:**
```bash
xcode-select --install
```

**Linux:**
```bash
sudo apt-get install build-essential  # Debian/Ubuntu
sudo yum install make                 # RHEL/CentOS
```

### "docker: command not found" or "podman: command not found"

The Makefile automatically detects and uses either Docker or Podman. Install one of them:

**Docker:**
```bash
# macOS
brew install --cask docker

# Linux
# Follow: https://docs.docker.com/engine/install/
```

**Podman (Docker alternative):**
```bash
# macOS
brew install podman
podman machine init
podman machine start

# Linux (Fedora/RHEL)
sudo dnf install podman

# Linux (Debian/Ubuntu)
sudo apt-get install podman
```

After installation, verify with:
```bash
make info  # Shows which container runtime is detected
```

### Permission denied when installing

The plugin directory might not exist or have wrong permissions:
```bash
# Create directory manually
mkdir -p "$(make info | grep 'Plugin Dir' | cut -d: -f2 | xargs)"

# Or run with sudo (not recommended)
sudo make install
```

### Tests fail with "connection refused"

Redis is not running:
```bash
make redis-start
make redis-status
```

## See Also

- [`README.md`](../README.md) - User documentation
- [`AGENTS.md`](../AGENTS.md) - AI agent guidance
- [`docs/QUICK_START_TESTING.md`](QUICK_START_TESTING.md) - Testing guide
- [`.goreleaser.yaml`](../.goreleaser.yaml) - Release configuration
