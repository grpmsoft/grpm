package tools

import (
	"fmt"
	"os"
	"strings"
)

// Checker provides build-time tool verification for packages.
//
// Checker integrates with the ebuild system to detect what tools are needed
// for building a package and whether they are available on the system.
type Checker struct {
	detector *Detector
	registry *Registry
}

// CheckResult contains the result of a tool availability check.
type CheckResult struct {
	// Required lists all tools required for the build.
	Required []*Tool

	// Available lists tools that were found on the system.
	Available []*Tool

	// Missing lists tools that are not available.
	Missing []*Tool

	// Eclasses lists the eclasses that were analyzed.
	Eclasses []string

	// CanBuild is true if all required (non-optional) tools are available.
	CanBuild bool
}

// NewChecker creates a new Checker with the default registry.
func NewChecker() *Checker {
	registry := NewDefaultRegistry()
	detector := NewDetector(registry)

	return &Checker{
		detector: detector,
		registry: registry,
	}
}

// NewCheckerWithRegistry creates a new Checker with a custom registry.
func NewCheckerWithRegistry(registry *Registry) *Checker {
	return &Checker{
		detector: NewDetector(registry),
		registry: registry,
	}
}

// CheckForEclasses checks tool availability for the given eclasses.
//
// Returns a CheckResult with details about required and missing tools.
func (c *Checker) CheckForEclasses(eclasses []string) *CheckResult {
	result := &CheckResult{
		Eclasses: eclasses,
		CanBuild: true,
	}

	// Collect all required tools
	seen := make(map[string]bool)
	for _, eclass := range eclasses {
		// Get tools from registry
		tools := c.registry.ByEclass(eclass)
		for _, tool := range tools {
			if seen[tool.Name] {
				continue
			}
			seen[tool.Name] = true
			result.Required = append(result.Required, tool)

			if c.detector.IsAvailable(tool.Name) {
				result.Available = append(result.Available, tool)
			} else if !tool.Optional {
				result.Missing = append(result.Missing, tool)
				result.CanBuild = false
			}
		}

		// Also check static map for eclasses not in registry
		if toolNames, ok := EclassToolMap()[eclass]; ok {
			for _, name := range toolNames {
				if seen[name] {
					continue
				}
				if tool := c.registry.Get(name); tool != nil {
					seen[name] = true
					result.Required = append(result.Required, tool)

					if c.detector.IsAvailable(tool.Name) {
						result.Available = append(result.Available, tool)
					} else if !tool.Optional {
						result.Missing = append(result.Missing, tool)
						result.CanBuild = false
					}
				}
			}
		}
	}

	return result
}

// CheckForEbuild checks tool availability for an ebuild file.
//
// It extracts the "inherit" statement from the ebuild and checks
// the required tools for those eclasses.
func (c *Checker) CheckForEbuild(ebuildPath string) (*CheckResult, error) {
	content, err := os.ReadFile(ebuildPath)
	if err != nil {
		return nil, fmt.Errorf("reading ebuild: %w", err)
	}

	eclasses := ExtractInherit(string(content))
	return c.CheckForEclasses(eclasses), nil
}

// FormatMissingTools formats the missing tools as a user-friendly error message.
func FormatMissingTools(missing []*Tool) string {
	if len(missing) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Missing required tools:\n")

	for _, tool := range missing {
		sb.WriteString(fmt.Sprintf("  - %s: %s\n", tool.Name, tool.Description))
		sb.WriteString(fmt.Sprintf("    Install: grpm install %s\n", tool.Package))
	}

	sb.WriteString("\nInstall all missing tools:\n")
	packages := make([]string, len(missing))
	for i, tool := range missing {
		packages[i] = tool.Package
	}
	sb.WriteString(fmt.Sprintf("  grpm install %s\n", strings.Join(packages, " ")))

	return sb.String()
}

// FormatCheckResult formats a CheckResult as a detailed report.
func FormatCheckResult(result *CheckResult) string {
	var sb strings.Builder

	sb.WriteString("Tool Check Result\n")
	sb.WriteString("=================\n\n")

	if len(result.Eclasses) > 0 {
		sb.WriteString(fmt.Sprintf("Eclasses: %s\n\n", strings.Join(result.Eclasses, ", ")))
	}

	sb.WriteString(fmt.Sprintf("Required tools: %d\n", len(result.Required)))
	sb.WriteString(fmt.Sprintf("Available: %d\n", len(result.Available)))
	sb.WriteString(fmt.Sprintf("Missing: %d\n\n", len(result.Missing)))

	if result.CanBuild {
		sb.WriteString("Status: READY TO BUILD\n")
	} else {
		sb.WriteString("Status: BLOCKED (missing required tools)\n\n")
		sb.WriteString(FormatMissingTools(result.Missing))
	}

	return sb.String()
}

// ExtractInherit extracts inherited eclasses from ebuild content.
//
// Parses "inherit eclass1 eclass2 ..." statements, handling line continuations.
func ExtractInherit(content string) []string {
	var inherits []string

	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Check for inherit statement
		if strings.HasPrefix(line, "inherit ") {
			eclasses := strings.TrimPrefix(line, "inherit ")

			// Handle line continuation
			for strings.HasSuffix(eclasses, "\\") {
				eclasses = strings.TrimSuffix(eclasses, "\\")
				i++
				if i < len(lines) {
					eclasses += " " + strings.TrimSpace(lines[i])
				}
			}

			// Split into individual eclasses
			for _, ec := range strings.Fields(eclasses) {
				ec = strings.TrimSpace(ec)
				if ec != "" && ec != "\\" {
					inherits = append(inherits, ec)
				}
			}
		}
	}

	return inherits
}

// CheckForEclassesOnly checks tool availability ONLY for the specified eclasses.
//
// Unlike CheckForEclasses, this does NOT query the registry's ByEclass cache,
// which can include tools registered for broad eclasses like toolchain-funcs.
// Instead, it uses ONLY the explicit EclassToolMap() mapping.
//
// This is the preferred method for per-package tool checking, as it ensures
// a package like sys-libs/glibc doesn't require Rust, Java, or Ruby just because
// those tools are registered in the global registry.
//
// Parameters:
//   - eclasses: List of eclass names inherited by the package
//
// Returns a CheckResult with details about required and missing tools.
func (c *Checker) CheckForEclassesOnly(eclasses []string) *CheckResult {
	result := &CheckResult{
		Eclasses: eclasses,
		CanBuild: true,
	}

	// Collect tools from static EclassToolMap ONLY
	seen := make(map[string]bool)
	eclassMap := EclassToolMap()

	for _, eclass := range eclasses {
		// Only use static map - skip registry lookup
		toolNames, ok := eclassMap[eclass]
		if !ok {
			// Eclass has no tool requirements - this is fine
			continue
		}

		for _, name := range toolNames {
			if seen[name] {
				continue
			}
			seen[name] = true

			// Get tool info from registry (for Package and Description)
			tool := c.registry.Get(name)
			if tool == nil {
				// Create synthetic entry for unknown tool
				tool = &Tool{
					Name:        name,
					Binary:      name,
					Package:     "unknown",
					Description: "Required by " + eclass,
				}
			}

			result.Required = append(result.Required, tool)

			if c.detector.IsAvailable(tool.Name) {
				result.Available = append(result.Available, tool)
			} else if !tool.Optional {
				result.Missing = append(result.Missing, tool)
				result.CanBuild = false
			}
		}
	}

	return result
}

// MustHaveTools checks if all specified tools are available.
//
// Returns nil if all tools are available, or an error with details about missing tools.
func (c *Checker) MustHaveTools(names ...string) error {
	var missing []*Tool

	for _, name := range names {
		if !c.detector.IsAvailable(name) {
			if tool := c.registry.Get(name); tool != nil {
				missing = append(missing, tool)
			} else {
				// Create synthetic tool entry for unknown tool
				missing = append(missing, &Tool{
					Name:        name,
					Binary:      name,
					Package:     "unknown",
					Description: "Unknown tool",
				})
			}
		}
	}

	if len(missing) > 0 {
		return &MissingToolsError{Missing: missing}
	}

	return nil
}

// MissingToolsError is returned when required tools are not available.
type MissingToolsError struct {
	Missing []*Tool
}

// Error implements the error interface.
func (e *MissingToolsError) Error() string {
	if len(e.Missing) == 1 {
		return fmt.Sprintf("missing required tool: %s", e.Missing[0].Name)
	}

	names := make([]string, len(e.Missing))
	for i, tool := range e.Missing {
		names[i] = tool.Name
	}
	return fmt.Sprintf("missing %d required tools: %s", len(e.Missing), strings.Join(names, ", "))
}

// GetMissing returns the list of missing tools.
func (e *MissingToolsError) GetMissing() []*Tool {
	return e.Missing
}

// InstallHint returns a command to install missing tools.
func (e *MissingToolsError) InstallHint() string {
	packages := make([]string, len(e.Missing))
	for i, tool := range e.Missing {
		packages[i] = tool.Package
	}
	return fmt.Sprintf("grpm install %s", strings.Join(packages, " "))
}
