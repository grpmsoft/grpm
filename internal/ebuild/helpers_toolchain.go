// Package ebuild implements ebuild execution engine.
//
// This file provides EAPI 8 toolchain functions (tc-getCC, tc-getCXX, etc.).
package ebuild

import (
	"os"
	"runtime"
)

// ============================================================================
// EAPI 8 Toolchain Functions
// ============================================================================

// TcGetCC prints the C compiler command.
//
// Usage: tc-getCC
//
// Returns CC from environment or default "gcc".
func (h *Helpers) TcGetCC(args []string) error {
	cc := h.getEnvOrDefault("CC", "gcc")
	h.writeStdout(cc)
	return nil
}

// TcGetCXX prints the C++ compiler command.
//
// Usage: tc-getCXX
//
// Returns CXX from environment or default "g++".
func (h *Helpers) TcGetCXX(args []string) error {
	cxx := h.getEnvOrDefault("CXX", "g++")
	h.writeStdout(cxx)
	return nil
}

// TcGetLD prints the linker command.
//
// Usage: tc-getLD
//
// Returns LD from environment or default "ld".
func (h *Helpers) TcGetLD(args []string) error {
	ld := h.getEnvOrDefault("LD", "ld")
	h.writeStdout(ld)
	return nil
}

// TcArch prints the target architecture.
//
// Usage: tc-arch
//
// Returns architecture name suitable for Portage KEYWORDS.
func (h *Helpers) TcArch(args []string) error {
	arch := h.detectArch()
	h.writeStdout(arch)
	return nil
}

// getEnvOrDefault gets an environment variable or returns default.
func (h *Helpers) getEnvOrDefault(key, defaultVal string) string {
	if h.env == nil {
		return defaultVal
	}

	// Check Environment struct fields
	switch key {
	case "CC":
		if h.env.CFLAGS != "" {
			// No CC field in Environment, use OS env
			if cc := os.Getenv("CC"); cc != "" {
				return cc
			}
		}
	case "CXX":
		if h.env.CXXFLAGS != "" {
			if cxx := os.Getenv("CXX"); cxx != "" {
				return cxx
			}
		}
	case "LD":
		if ld := os.Getenv("LD"); ld != "" {
			return ld
		}
	}

	return defaultVal
}

// detectArch detects the current architecture for Portage KEYWORDS.
func (h *Helpers) detectArch() string {
	// Map Go GOARCH to Gentoo KEYWORDS
	archMap := map[string]string{
		"amd64":   "amd64",
		"386":     "x86",
		"arm":     "arm",
		"arm64":   "arm64",
		"ppc64":   "ppc64",
		"ppc64le": "ppc64",
		"riscv64": "riscv",
		"s390x":   "s390",
		"mips":    "mips",
		"mips64":  "mips",
	}

	goarch := runtime.GOARCH
	if arch, ok := archMap[goarch]; ok {
		return arch
	}

	return goarch
}
