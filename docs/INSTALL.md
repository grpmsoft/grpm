# GRPM Installation Guide

This guide covers installation of GRPM on Gentoo Linux and compatible distributions.

## Table of Contents

- [Requirements](#requirements)
- [Pre-built Binary Installation](#pre-built-binary-installation)
- [Building from Source](#building-from-source)
- [Post-Installation Setup](#post-installation-setup)
- [Daemon Configuration](#daemon-configuration)
- [Verification](#verification)
- [Uninstallation](#uninstallation)
- [Troubleshooting](#troubleshooting)

---

## Requirements

### System Requirements

- **Operating System**: Linux (Gentoo, Calculate Linux, Funtoo, or compatible)
- **Architecture**: x86_64 (amd64), ARM64, ARMv7, ARMv6, or i386
- **Filesystem**: Btrfs or ZFS recommended for snapshot support (optional)
- **Disk Space**: ~50 MB for binary, ~500 MB for build from source

### Runtime Dependencies

- `gpg` - Required for repository GPG verification
- `git` - Required for Git-based repository sync
- `rsync` - Optional, for rsync-based repository sync

### Build Dependencies (source installation only)

- Go 1.25 or later
- Git
- Make
- GCC (for race detector tests)

---

## Pre-built Binary Installation

### Download and Install

```bash
# Set version and architecture
VERSION="0.9.3"
ARCH="x86_64"  # Options: x86_64, arm64, armv7, armv6, i386

# Download binary
wget "https://github.com/grpmsoft/grpm/releases/download/v${VERSION}/grpm_${VERSION}_linux_${ARCH}.tar.gz"

# Download checksums
wget "https://github.com/grpmsoft/grpm/releases/download/v${VERSION}/checksums.txt"

# Verify checksum
sha256sum -c checksums.txt --ignore-missing

# Extract
tar -xzf "grpm_${VERSION}_linux_${ARCH}.tar.gz"

# Install binary
sudo install -m 0755 grpm /usr/bin/grpm

# Verify installation
grpm -V
```

### Available Architectures

| Architecture | Filename | Description |
|--------------|----------|-------------|
| x86_64 | `grpm_*_linux_x86_64.tar.gz` | 64-bit Intel/AMD (most common) |
| ARM64 | `grpm_*_linux_arm64.tar.gz` | 64-bit ARM (Apple Silicon, AWS Graviton) |
| ARMv7 | `grpm_*_linux_armv7.tar.gz` | 32-bit ARM with hardware float |
| ARMv6 | `grpm_*_linux_armv6.tar.gz` | Raspberry Pi Zero/1 |
| i386 | `grpm_*_linux_i386.tar.gz` | 32-bit Intel/AMD |

---

## Building from Source

### Standard Build

```bash
# Clone repository
git clone https://github.com/grpmsoft/grpm.git
cd grpm

# Build
make build

# Run tests (recommended)
make test

# Install to /usr/bin
sudo make install

# Verify
grpm -V
```

### Build Options

```bash
# Build with version information
make build VERSION=0.1.0

# Build for specific platform
GOOS=linux GOARCH=arm64 make build

# Build with race detector (development)
go build -race -o bin/grpm ./cmd/grpm

# Clean build
make clean && make build
```

### Development Build

```bash
# Full development workflow
make dev    # fmt, lint, test, build

# Individual steps
make fmt    # Format code
make lint   # Run linter
make test   # Run tests
make build  # Build binary
```

---

## Post-Installation Setup

### 1. Verify Portage Repository

GRPM uses the standard Gentoo repository location:

```bash
# Check repository exists
ls /var/db/repos/gentoo

# If not present, sync using traditional emerge
sudo emerge --sync

# Or use GRPM to sync
sudo grpm sync
```

### 2. Configure GPG Keys (for repository verification)

```bash
# Import Gentoo release keys
sudo wget -O /usr/share/openpgp-keys/gentoo-release.asc \
    https://qa-reports.gentoo.org/output/service-keys.gpg

# Or install via Portage
sudo emerge app-crypt/gentoo-keys
```

### 3. Set Up Binary Package Directory (optional)

```bash
# Create binary package cache directory
sudo mkdir -p /var/cache/binpkgs

# Set permissions
sudo chown root:portage /var/cache/binpkgs
sudo chmod 775 /var/cache/binpkgs
```

### 4. Configure Distfiles Directory

```bash
# Create distfiles directory if needed
sudo mkdir -p /var/cache/distfiles

# Set permissions
sudo chown root:portage /var/cache/distfiles
sudo chmod 775 /var/cache/distfiles
```

---

## Daemon Configuration

GRPM includes a daemon mode for background operations and API access.

### Manual Start

```bash
# Start daemon
sudo grpm daemon

# Check status
grpm status
```

### OpenRC Service

Create `/etc/init.d/grpmd`:

```bash
#!/sbin/openrc-run

name="GRPM Daemon"
description="Go Resource Package Manager Daemon"
command="/usr/bin/grpm"
command_args="daemon"
command_background=true
pidfile="/var/run/grpm.pid"

depend() {
    need localmount
    after bootmisc
}
```

Enable and start:

```bash
sudo chmod +x /etc/init.d/grpmd
sudo rc-update add grpmd default
sudo rc-service grpmd start
```

### Systemd Service

Create `/etc/systemd/system/grpmd.service`:

```ini
[Unit]
Description=GRPM Daemon
After=network.target

[Service]
Type=simple
ExecStart=/usr/bin/grpm daemon
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable grpmd
sudo systemctl start grpmd
```

---

## Verification

### Basic Functionality

```bash
# Version information
grpm -V

# Daemon status
grpm status

# Search for a package
grpm search hello

# Show package info
grpm info app-misc/hello

# Resolve dependencies (dry-run)
grpm resolve --pretend app-misc/hello
```

### Test Installation (safe, non-destructive)

```bash
# Use mock repository for testing
grpm resolve --mock sys-libs/zlib
grpm info --mock app-misc/hello
```

### Full Test (requires root)

```bash
# Test sync (creates backup first)
sudo grpm sync --pretend

# Test installation (dry-run)
sudo grpm install --pretend app-misc/hello

# Test emerge (dry-run)
sudo grpm emerge --pretend app-misc/hello
```

---

## Uninstallation

### Remove Binary

```bash
# Stop daemon if running
sudo rc-service grpmd stop 2>/dev/null
sudo systemctl stop grpmd 2>/dev/null

# Remove binary
sudo rm /usr/bin/grpm

# Remove service files (OpenRC)
sudo rm /etc/init.d/grpmd
sudo rc-update del grpmd default

# Remove service files (systemd)
sudo rm /etc/systemd/system/grpmd.service
sudo systemctl daemon-reload

# Remove PID file
sudo rm -f /var/run/grpm.pid /var/run/grpm.sock
```

### Clean Build Directory (if built from source)

```bash
cd /path/to/grpm
make clean
```

---

## Troubleshooting

### Common Issues

#### "Permission denied" errors

GRPM requires root privileges for package installation:

```bash
sudo grpm install <package>
```

#### "Repository not found" error

Ensure the Gentoo repository exists:

```bash
ls /var/db/repos/gentoo

# If missing, sync:
sudo grpm sync
# Or: sudo emerge --sync
```

#### GPG verification fails

Import Gentoo release keys:

```bash
sudo emerge app-crypt/gentoo-keys
```

Or skip verification (not recommended):

```bash
sudo grpm sync --skip-gpg-verify
```

#### Daemon connection failed

Check if daemon is running:

```bash
grpm status

# Start daemon if needed
sudo grpm daemon &
```

### Debug Mode

Enable verbose output for troubleshooting:

```bash
# Single -v for basic verbose
grpm -v sync

# Double -vv for more detail
grpm -vv resolve app-misc/hello

# Triple -vvv for maximum verbosity
grpm -vvv install --pretend app-misc/hello

# Environment variable
GRPM_VERBOSE=1 grpm sync
```

### Log Files

- **Daemon logs**: Check syslog or journal
- **Build logs**: `/var/tmp/portage/<category>/<package>/temp/build.log`

### Getting Help

- **GitHub Issues**: https://github.com/grpmsoft/grpm/issues
- **Documentation**: https://github.com/grpmsoft/grpm/tree/main/docs

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GRPM_VERBOSE` | Enable verbose output (1=on) | `0` |
| `GRPM_SOCKET` | Daemon socket path | `/var/run/grpm.sock` |

---

## See Also

- [CLI Reference](CLI_REFERENCE.md) - Complete command documentation
- [README](../README.md) - Project overview
- [CONTRIBUTING](../CONTRIBUTING.md) - Development guidelines
