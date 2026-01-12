package tools

import (
	"fmt"
	"strings"
	"sync"
)

// ToolCategory represents a category of external tools.
type ToolCategory string

// Tool categories for organization.
const (
	CategoryCompiler      ToolCategory = "compilers"
	CategoryBuildSystem   ToolCategory = "build-systems"
	CategoryLanguage      ToolCategory = "languages"
	CategoryUtility       ToolCategory = "utilities"
	CategoryCompression   ToolCategory = "compression"
	CategoryDocumentation ToolCategory = "documentation"
	CategoryVCS           ToolCategory = "vcs"
)

// AllCategories returns all defined tool categories.
func AllCategories() []ToolCategory {
	return []ToolCategory{
		CategoryCompiler,
		CategoryBuildSystem,
		CategoryLanguage,
		CategoryUtility,
		CategoryCompression,
		CategoryDocumentation,
		CategoryVCS,
	}
}

// Tool represents an external tool that may be required for building packages.
//
// Tool is a value object - once created, it should be treated as immutable.
// All fields are exported for easy serialization but should not be modified.
type Tool struct {
	// Name is the canonical name of the tool (e.g., "cmake").
	Name string

	// Binary is the executable name to search for (e.g., "cmake").
	// May differ from Name for tools with different binary names.
	Binary string

	// Package is the Gentoo package that provides this tool (e.g., "dev-build/cmake").
	Package string

	// Description is a brief description of what the tool does.
	Description string

	// Categories lists the categories this tool belongs to.
	Categories []ToolCategory

	// RequiredBy lists eclasses that require this tool.
	RequiredBy []string

	// Optional indicates if this tool is optional (nice to have but not required).
	Optional bool
}

// NewTool creates a new Tool with required fields.
//
// Parameters:
//   - name: Tool name (used as key in registry)
//   - binary: Executable to search in PATH
//   - pkg: Gentoo package to install (category/name format)
//   - description: Brief description
func NewTool(name, binary, pkg, description string) *Tool {
	return &Tool{
		Name:        name,
		Binary:      binary,
		Package:     pkg,
		Description: description,
		Categories:  make([]ToolCategory, 0),
		RequiredBy:  make([]string, 0),
	}
}

// WithCategories adds categories to the tool.
func (t *Tool) WithCategories(cats ...ToolCategory) *Tool {
	t.Categories = append(t.Categories, cats...)
	return t
}

// WithRequiredBy adds eclasses that require this tool.
func (t *Tool) WithRequiredBy(eclasses ...string) *Tool {
	t.RequiredBy = append(t.RequiredBy, eclasses...)
	return t
}

// WithOptional marks the tool as optional.
func (t *Tool) WithOptional() *Tool {
	t.Optional = true
	return t
}

// String returns a human-readable representation of the tool.
func (t *Tool) String() string {
	return fmt.Sprintf("%s (%s)", t.Name, t.Package)
}

// InstallHint returns a hint for installing this tool.
func (t *Tool) InstallHint() string {
	return fmt.Sprintf("Run: grpm install %s", t.Package)
}

// HasCategory checks if the tool belongs to the given category.
func (t *Tool) HasCategory(cat ToolCategory) bool {
	for _, c := range t.Categories {
		if c == cat {
			return true
		}
	}
	return false
}

// IsRequiredByEclass checks if this tool is required by the given eclass.
func (t *Tool) IsRequiredByEclass(eclass string) bool {
	for _, e := range t.RequiredBy {
		if e == eclass {
			return true
		}
	}
	return false
}

// Registry holds a collection of known external tools.
//
// Registry is thread-safe for concurrent access.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*Tool

	// byCategory caches tools by category for fast lookup
	byCategory map[ToolCategory][]*Tool

	// byEclass caches tools by eclass for fast lookup
	byEclass map[string][]*Tool
}

// NewRegistry creates a new empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools:      make(map[string]*Tool),
		byCategory: make(map[ToolCategory][]*Tool),
		byEclass:   make(map[string][]*Tool),
	}
}

// Register adds a tool to the registry.
//
// If a tool with the same name already exists, it will be replaced.
func (r *Registry) Register(tool *Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove from caches if replacing
	if existing, ok := r.tools[tool.Name]; ok {
		r.removeFromCachesLocked(existing)
	}

	r.tools[tool.Name] = tool
	r.addToCachesLocked(tool)
}

// removeFromCachesLocked removes a tool from category and eclass caches.
// Must be called with lock held.
func (r *Registry) removeFromCachesLocked(tool *Tool) {
	// Remove from category caches
	for _, cat := range tool.Categories {
		tools := r.byCategory[cat]
		for i, t := range tools {
			if t.Name == tool.Name {
				r.byCategory[cat] = append(tools[:i], tools[i+1:]...)
				break
			}
		}
	}

	// Remove from eclass caches
	for _, eclass := range tool.RequiredBy {
		tools := r.byEclass[eclass]
		for i, t := range tools {
			if t.Name == tool.Name {
				r.byEclass[eclass] = append(tools[:i], tools[i+1:]...)
				break
			}
		}
	}
}

// addToCachesLocked adds a tool to category and eclass caches.
// Must be called with lock held.
func (r *Registry) addToCachesLocked(tool *Tool) {
	// Add to category caches
	for _, cat := range tool.Categories {
		r.byCategory[cat] = append(r.byCategory[cat], tool)
	}

	// Add to eclass caches
	for _, eclass := range tool.RequiredBy {
		r.byEclass[eclass] = append(r.byEclass[eclass], tool)
	}
}

// Get retrieves a tool by name.
//
// Returns nil if the tool is not registered.
func (r *Registry) Get(name string) *Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// Has checks if a tool is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

// All returns all registered tools.
//
// The returned slice is a copy and can be safely modified.
func (r *Registry) All() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// ByCategory returns all tools in the given category.
//
// The returned slice is a copy and can be safely modified.
func (r *Registry) ByCategory(cat ToolCategory) []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := r.byCategory[cat]
	result := make([]*Tool, len(tools))
	copy(result, tools)
	return result
}

// ByEclass returns all tools required by the given eclass.
//
// The returned slice is a copy and can be safely modified.
func (r *Registry) ByEclass(eclass string) []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Normalize eclass name (remove .eclass suffix if present)
	eclass = strings.TrimSuffix(eclass, ".eclass")

	tools := r.byEclass[eclass]
	result := make([]*Tool, len(tools))
	copy(result, tools)
	return result
}

// Count returns the number of registered tools.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// Categories returns all categories that have registered tools.
func (r *Registry) Categories() []ToolCategory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ToolCategory, 0, len(r.byCategory))
	for cat := range r.byCategory {
		if len(r.byCategory[cat]) > 0 {
			result = append(result, cat)
		}
	}
	return result
}

// Eclasses returns all eclasses that have registered tool requirements.
func (r *Registry) Eclasses() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0, len(r.byEclass))
	for eclass := range r.byEclass {
		if len(r.byEclass[eclass]) > 0 {
			result = append(result, eclass)
		}
	}
	return result
}
