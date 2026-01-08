// Package solver provides SAT-based dependency resolution for GRPM.
//
// This file implements autounmask functionality for automatically generating
// package.use and package.accept_keywords entries to resolve conflicts.
// Inspired by Portage's autounmask feature.
package solver

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// AutounmaskConfig contains configuration for autounmask operations.
type AutounmaskConfig struct {
	// ConfigRoot is the base configuration directory (default: /etc/portage)
	ConfigRoot string

	// Write determines if changes should be written to disk
	Write bool

	// Continue determines if emerge should continue after writing changes
	Continue bool

	// UseOnly limits autounmask to USE flag changes only (no keyword changes)
	UseOnly bool
}

// DefaultAutounmaskConfig returns the default autounmask configuration.
func DefaultAutounmaskConfig() *AutounmaskConfig {
	return &AutounmaskConfig{
		ConfigRoot: "/etc/portage",
		Write:      false,
		Continue:   false,
		UseOnly:    false,
	}
}

// AutounmaskEntry represents a single entry for package.use or related files.
type AutounmaskEntry struct {
	// Atom is the package atom (e.g., "sys-libs/zlib")
	Atom string

	// Changes contains flag changes (for package.use)
	Changes []string

	// Keyword contains keyword to accept (for package.accept_keywords)
	Keyword string

	// Reason explains why this entry was generated
	Reason string

	// File indicates which file this entry belongs to
	File AutounmaskFile

	// SourceCollision is the collision that generated this entry
	SourceCollision *SlotCollision
}

// AutounmaskFile indicates which portage config file to modify.
type AutounmaskFile int

const (
	// FilePackageUse is for USE flag changes (/etc/portage/package.use)
	FilePackageUse AutounmaskFile = iota

	// FilePackageAcceptKeywords is for keyword changes (/etc/portage/package.accept_keywords)
	FilePackageAcceptKeywords

	// FilePackageUnmask is for unmasking (/etc/portage/package.unmask)
	FilePackageUnmask

	// FilePackageLicense is for license acceptance (/etc/portage/package.license)
	FilePackageLicense
)

// String returns the filename for this file type.
func (f AutounmaskFile) String() string {
	switch f {
	case FilePackageUse:
		return "package.use"
	case FilePackageAcceptKeywords:
		return "package.accept_keywords"
	case FilePackageUnmask:
		return "package.unmask"
	case FilePackageLicense:
		return "package.license"
	default:
		return "unknown"
	}
}

// FilePath returns the full path for this file type.
func (f AutounmaskFile) FilePath(configRoot string) string {
	return filepath.Join(configRoot, f.String())
}

// AutounmaskWriter generates and optionally writes autounmask entries.
type AutounmaskWriter struct {
	config  *AutounmaskConfig
	entries []*AutounmaskEntry
}

// NewAutounmaskWriter creates a new autounmask writer.
func NewAutounmaskWriter(config *AutounmaskConfig) *AutounmaskWriter {
	if config == nil {
		config = DefaultAutounmaskConfig()
	}
	return &AutounmaskWriter{
		config:  config,
		entries: make([]*AutounmaskEntry, 0),
	}
}

// AddUseChange adds a USE flag change entry.
func (w *AutounmaskWriter) AddUseChange(atom string, flags []string, reason string, collision *SlotCollision) {
	w.entries = append(w.entries, &AutounmaskEntry{
		Atom:            atom,
		Changes:         flags,
		Reason:          reason,
		File:            FilePackageUse,
		SourceCollision: collision,
	})
}

// AddKeywordChange adds a keyword acceptance entry.
func (w *AutounmaskWriter) AddKeywordChange(atom, keyword, reason string, collision *SlotCollision) {
	w.entries = append(w.entries, &AutounmaskEntry{
		Atom:            atom,
		Keyword:         keyword,
		Reason:          reason,
		File:            FilePackageAcceptKeywords,
		SourceCollision: collision,
	})
}

// AddUnmask adds a package unmask entry.
func (w *AutounmaskWriter) AddUnmask(atom, reason string, collision *SlotCollision) {
	w.entries = append(w.entries, &AutounmaskEntry{
		Atom:            atom,
		Reason:          reason,
		File:            FilePackageUnmask,
		SourceCollision: collision,
	})
}

// AddFromSolution adds entries from a collision solution.
func (w *AutounmaskWriter) AddFromSolution(solution *CollisionSolution, collision *SlotCollision) {
	for _, useChange := range solution.UseChanges {
		var flags []string
		for flag, enabled := range useChange.FlagChanges {
			if enabled {
				flags = append(flags, flag)
			} else {
				flags = append(flags, "-"+flag)
			}
		}
		sort.Strings(flags)

		w.AddUseChange(
			useChange.Package.Name,
			flags,
			useChange.Reason,
			collision,
		)
	}
}

// GetEntries returns all collected entries.
func (w *AutounmaskWriter) GetEntries() []*AutounmaskEntry {
	return w.entries
}

// GetEntriesByFile returns entries grouped by target file.
func (w *AutounmaskWriter) GetEntriesByFile() map[AutounmaskFile][]*AutounmaskEntry {
	result := make(map[AutounmaskFile][]*AutounmaskEntry)
	for _, entry := range w.entries {
		result[entry.File] = append(result[entry.File], entry)
	}
	return result
}

// GenerateContent generates the content string for all entries.
func (w *AutounmaskWriter) GenerateContent() map[AutounmaskFile]string {
	result := make(map[AutounmaskFile]string)
	byFile := w.GetEntriesByFile()

	for fileType, entries := range byFile {
		var sb strings.Builder

		// Header comment
		sb.WriteString(fmt.Sprintf("# Generated by GRPM autounmask on %s\n",
			time.Now().Format("2006-01-02 15:04:05")))
		sb.WriteString("# This file was automatically generated to resolve package conflicts.\n\n")

		// Sort entries by atom for consistent output
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Atom < entries[j].Atom
		})

		for _, entry := range entries {
			// Add comment with reason
			if entry.Reason != "" {
				sb.WriteString(fmt.Sprintf("# %s\n", entry.Reason))
			}

			// Add entry line
			switch fileType {
			case FilePackageUse:
				sb.WriteString(fmt.Sprintf("%s %s\n",
					entry.Atom, strings.Join(entry.Changes, " ")))
			case FilePackageAcceptKeywords:
				sb.WriteString(fmt.Sprintf("%s %s\n", entry.Atom, entry.Keyword))
			case FilePackageUnmask:
				sb.WriteString(fmt.Sprintf("%s\n", entry.Atom))
			case FilePackageLicense:
				sb.WriteString(fmt.Sprintf("%s %s\n",
					entry.Atom, strings.Join(entry.Changes, " ")))
			}
		}

		result[fileType] = sb.String()
	}

	return result
}

// Write writes all entries to their respective files.
func (w *AutounmaskWriter) Write() error {
	if !w.config.Write {
		return nil
	}

	contents := w.GenerateContent()

	for fileType, content := range contents {
		path := fileType.FilePath(w.config.ConfigRoot)

		// Ensure directory exists
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		// Check if file is a directory (package.use/grpm style)
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			// Write to package.use/grpm file instead
			path = filepath.Join(path, "grpm")
		}

		// Append to existing file or create new
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", path, err)
		}

		_, writeErr := f.WriteString("\n" + content)
		closeErr := f.Close()

		if writeErr != nil {
			return fmt.Errorf("failed to write to %s: %w", path, writeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close %s: %w", path, closeErr)
		}
	}

	return nil
}

// FormatPreview generates a preview of what would be written.
func (w *AutounmaskWriter) FormatPreview() string {
	var sb strings.Builder

	contents := w.GenerateContent()
	if len(contents) == 0 {
		return "No autounmask changes required.\n"
	}

	sb.WriteString("\nThe following changes would be made:\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")

	// Sort file types for consistent output
	fileTypes := make([]AutounmaskFile, 0, len(contents))
	for ft := range contents {
		fileTypes = append(fileTypes, ft)
	}
	sort.Slice(fileTypes, func(i, j int) bool {
		return int(fileTypes[i]) < int(fileTypes[j])
	})

	for _, fileType := range fileTypes {
		content := contents[fileType]
		path := fileType.FilePath(w.config.ConfigRoot)

		sb.WriteString(fmt.Sprintf(">>> %s:\n", path))
		sb.WriteString(strings.Repeat("-", 40) + "\n")

		// Indent content
		for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
			sb.WriteString("    " + line + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// Clear removes all collected entries.
func (w *AutounmaskWriter) Clear() {
	w.entries = make([]*AutounmaskEntry, 0)
}

// HasEntries returns true if there are any entries.
func (w *AutounmaskWriter) HasEntries() bool {
	return len(w.entries) > 0
}

// EntryCount returns the number of entries.
func (w *AutounmaskWriter) EntryCount() int {
	return len(w.entries)
}

// InteractiveAutounmask provides interactive autounmask functionality.
type InteractiveAutounmask struct {
	writer   *AutounmaskWriter
	resolver *CollisionResolver
}

// NewInteractiveAutounmask creates a new interactive autounmask handler.
func NewInteractiveAutounmask(resolver *CollisionResolver, config *AutounmaskConfig) *InteractiveAutounmask {
	return &InteractiveAutounmask{
		writer:   NewAutounmaskWriter(config),
		resolver: resolver,
	}
}

// ProcessCollisions processes all collisions and generates autounmask entries.
func (ia *InteractiveAutounmask) ProcessCollisions(collisions []*SlotCollision) error {
	for _, collision := range collisions {
		solutions := ia.resolver.ResolveCollision(collision)
		if len(solutions) == 0 {
			continue // No solution found
		}

		// Use the first (best) solution
		ia.writer.AddFromSolution(solutions[0], collision)
	}

	return nil
}

// GetWriter returns the underlying autounmask writer.
func (ia *InteractiveAutounmask) GetWriter() *AutounmaskWriter {
	return ia.writer
}

// FormatReport generates a complete report with collisions and solutions.
func (ia *InteractiveAutounmask) FormatReport(collisions []*SlotCollision) string {
	var sb strings.Builder

	// Show collision report
	sb.WriteString(GenerateConflictReport(collisions))

	// Show autounmask preview
	if ia.writer.HasEntries() {
		sb.WriteString("\nAutounmask changes:\n")
		sb.WriteString(ia.writer.FormatPreview())

		sb.WriteString("\nUse --autounmask-write to apply these changes automatically.\n")
	}

	return sb.String()
}

// ApplyAutounmask applies the collected autounmask entries.
func (ia *InteractiveAutounmask) ApplyAutounmask() error {
	ia.writer.config.Write = true
	return ia.writer.Write()
}
