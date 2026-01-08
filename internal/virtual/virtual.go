// Package virtual implements virtual package handling for GRPM.
//
// Virtual packages in Gentoo are abstract packages that can be satisfied
// by one of several concrete provider packages. For example, virtual/jdk
// can be provided by dev-java/openjdk, dev-java/oracle-jdk-bin, etc.
//
// This package provides:
//   - Virtual package detection (IsVirtual)
//   - Provider resolution with preference support
//   - Configuration loading for default providers
//
// Example usage:
//
//	resolver := virtual.NewResolver()
//	resolver.SetDefault("virtual/jdk", "dev-java/openjdk")
//	provider, err := resolver.SelectProvider("virtual/jdk", available)
package virtual

import (
	"strings"
)

// Virtual represents a virtual package with its available providers.
//
// Virtual packages act as abstract dependencies that can be satisfied
// by one of several concrete provider packages.
type Virtual struct {
	// Name is the virtual package name (e.g., "virtual/jdk")
	Name string

	// Version is the virtual package version
	Version string

	// Providers is the list of available provider package names
	// e.g., ["dev-java/openjdk:17", "dev-java/openjdk-bin:17"]
	Providers []string
}

// NewVirtual creates a new Virtual package.
//
// The name should include the "virtual/" category prefix.
func NewVirtual(name, version string) *Virtual {
	return &Virtual{
		Name:      name,
		Version:   version,
		Providers: make([]string, 0),
	}
}

// AddProvider adds a provider to this virtual package.
func (v *Virtual) AddProvider(provider string) {
	v.Providers = append(v.Providers, provider)
}

// HasProviders returns true if this virtual has at least one provider.
func (v *Virtual) HasProviders() bool {
	return len(v.Providers) > 0
}

// IsVirtual checks if a package name is a virtual package.
//
// Virtual packages are identified by the "virtual/" category prefix.
//
// Examples:
//
//	IsVirtual("virtual/jdk")        // true
//	IsVirtual("dev-java/openjdk")   // false
//	IsVirtual("virtual/editor")     // true
func IsVirtual(pkgName string) bool {
	return strings.HasPrefix(pkgName, "virtual/")
}

// ExtractCategory extracts the category from a package name.
//
// Examples:
//
//	ExtractCategory("virtual/jdk")        // "virtual"
//	ExtractCategory("dev-java/openjdk")   // "dev-java"
func ExtractCategory(pkgName string) string {
	parts := strings.SplitN(pkgName, "/", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

// ExtractPackageName extracts the package name without category.
//
// Examples:
//
//	ExtractPackageName("virtual/jdk")        // "jdk"
//	ExtractPackageName("dev-java/openjdk")   // "openjdk"
func ExtractPackageName(pkgName string) string {
	parts := strings.SplitN(pkgName, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return pkgName
}

// StripSlot removes the slot suffix from a package atom.
//
// Examples:
//
//	StripSlot("dev-java/openjdk:17")     // "dev-java/openjdk"
//	StripSlot("sys-libs/zlib:0/1")       // "sys-libs/zlib"
//	StripSlot("dev-java/openjdk")        // "dev-java/openjdk"
func StripSlot(atom string) string {
	idx := strings.Index(atom, ":")
	if idx != -1 {
		return atom[:idx]
	}
	return atom
}

// ExtractSlot extracts the slot from a package atom.
//
// Examples:
//
//	ExtractSlot("dev-java/openjdk:17")   // "17"
//	ExtractSlot("sys-libs/zlib:0/1")     // "0/1"
//	ExtractSlot("dev-java/openjdk")      // ""
func ExtractSlot(atom string) string {
	idx := strings.Index(atom, ":")
	if idx != -1 {
		return atom[idx+1:]
	}
	return ""
}
