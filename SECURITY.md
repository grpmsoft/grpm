# Security Policy

## Supported Versions

GRPM is currently in initial release (v0.1.x).

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| < 0.1.0 | :x:                |

## Reporting a Vulnerability

**DO NOT** open a public GitHub issue for security vulnerabilities.

Instead, please report security issues via:

1. **Private Security Advisory** (preferred):
   https://github.com/grpmsoft/grpm/security/advisories/new

2. **GitHub Discussions** (for less critical issues):
   https://github.com/grpmsoft/grpm/discussions

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Potential impact
- Suggested fix (if any)

### Response Timeline

- **Initial Response**: Within 72 hours
- **Fix & Disclosure**: Coordinated with reporter

## Security Considerations

GRPM is a package manager that executes ebuilds and manages system packages. Users should be aware of:

1. **Ebuild Execution** - Ebuilds contain shell scripts that are executed during package installation. Only use trusted repositories.

2. **Privilege Escalation** - Package installation requires root privileges. GRPM supports privilege dropping (`userpriv`, `userfetch`) to minimize attack surface.

3. **Build Sandboxing** - GRPM uses Linux namespaces (mount, PID, network, IPC) to isolate package builds when available.

4. **GPG Verification** - Repository sync supports GPG signature verification. Always enable GPG verification for production systems.

5. **Binary Packages** - Binary packages (.gpkg.tar, .tbz2) should be verified before installation. GRPM supports package signing.

6. **CONFIG_PROTECT** - Configuration files are protected from overwrites. Review `._cfg0000_*` files before merging.

## Security Best Practices

- Always sync repositories with GPG verification enabled
- Use `--pretend` flag to review changes before installation
- Enable build sandboxing (`FEATURES="sandbox"`)
- Run builds as unprivileged user (`FEATURES="userpriv"`)
- Verify binary packages from binhosts
- Keep GRPM updated to latest version

## Security Contact

- **GitHub Security Advisory**: https://github.com/grpmsoft/grpm/security/advisories/new
- **Public Issues**: https://github.com/grpmsoft/grpm/issues

---

**Thank you for helping keep GRPM secure!**
