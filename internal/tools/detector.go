package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Detector checks for the availability of external tools on the system.
//
// Detector caches results to avoid repeated PATH lookups. The cache can be
// cleared with Reset() if the PATH changes or tools are installed.
//
// Detector is thread-safe for concurrent access.
type Detector struct {
	registry *Registry

	mu    sync.RWMutex
	cache map[string]*DetectionResult
}

// DetectionResult holds the result of detecting a tool.
type DetectionResult struct {
	// Available is true if the tool was found.
	Available bool

	// Path is the full path to the binary (if found).
	Path string

	// Version is the detected version (if available).
	Version string
}

// NewDetector creates a new Detector with the given registry.
func NewDetector(registry *Registry) *Detector {
	return &Detector{
		registry: registry,
		cache:    make(map[string]*DetectionResult),
	}
}

// IsAvailable checks if a tool is available on the system.
//
// Results are cached for performance.
func (d *Detector) IsAvailable(name string) bool {
	result := d.detect(name)
	return result.Available
}

// FindBinary searches for a tool's binary in PATH.
//
// Returns the full path and true if found, empty string and false otherwise.
func (d *Detector) FindBinary(name string) (string, bool) {
	result := d.detect(name)
	return result.Path, result.Available
}

// GetResult returns the full detection result for a tool.
func (d *Detector) GetResult(name string) *DetectionResult {
	return d.detect(name)
}

// detect performs tool detection with caching.
func (d *Detector) detect(name string) *DetectionResult {
	// Check cache first
	d.mu.RLock()
	if result, ok := d.cache[name]; ok {
		d.mu.RUnlock()
		return result
	}
	d.mu.RUnlock()

	// Not in cache, perform detection
	result := d.performDetection(name)

	// Cache result
	d.mu.Lock()
	d.cache[name] = result
	d.mu.Unlock()

	return result
}

// performDetection actually searches for the tool.
func (d *Detector) performDetection(name string) *DetectionResult {
	result := &DetectionResult{Available: false}

	// Get tool from registry for binary name
	tool := d.registry.Get(name)
	binary := name
	if tool != nil && tool.Binary != "" {
		binary = tool.Binary
	}

	// Search in PATH using exec.LookPath
	path, err := exec.LookPath(binary)
	if err == nil {
		result.Available = true
		result.Path = path
		return result
	}

	// Try platform-specific extensions on Windows
	if runtime.GOOS == "windows" {
		extensions := []string{".exe", ".cmd", ".bat", ".com"}
		for _, ext := range extensions {
			path, err = exec.LookPath(binary + ext)
			if err == nil {
				result.Available = true
				result.Path = path
				return result
			}
		}
	}

	return result
}

// CheckAll checks availability of all registered tools.
//
// Returns a map of tool name to availability status.
func (d *Detector) CheckAll() map[string]bool {
	tools := d.registry.All()
	result := make(map[string]bool, len(tools))

	for _, tool := range tools {
		result[tool.Name] = d.IsAvailable(tool.Name)
	}

	return result
}

// CheckCategory checks availability of all tools in a category.
//
// Returns a map of tool name to availability status.
func (d *Detector) CheckCategory(cat ToolCategory) map[string]bool {
	tools := d.registry.ByCategory(cat)
	result := make(map[string]bool, len(tools))

	for _, tool := range tools {
		result[tool.Name] = d.IsAvailable(tool.Name)
	}

	return result
}

// Missing returns all registered tools that are not available.
func (d *Detector) Missing() []*Tool {
	tools := d.registry.All()
	var missing []*Tool

	for _, tool := range tools {
		if !d.IsAvailable(tool.Name) {
			missing = append(missing, tool)
		}
	}

	return missing
}

// MissingForEclass returns tools required by an eclass that are not available.
//
// This is useful for pre-build checks to identify what needs to be installed.
func (d *Detector) MissingForEclass(eclass string) []*Tool {
	tools := d.registry.ByEclass(eclass)
	var missing []*Tool

	for _, tool := range tools {
		if !d.IsAvailable(tool.Name) && !tool.Optional {
			missing = append(missing, tool)
		}
	}

	return missing
}

// MissingForEclasses returns tools required by any of the eclasses that are not available.
func (d *Detector) MissingForEclasses(eclasses []string) []*Tool {
	seen := make(map[string]bool)
	var missing []*Tool

	for _, eclass := range eclasses {
		for _, tool := range d.MissingForEclass(eclass) {
			if !seen[tool.Name] {
				seen[tool.Name] = true
				missing = append(missing, tool)
			}
		}
	}

	return missing
}

// Available returns all registered tools that are available.
func (d *Detector) Available() []*Tool {
	tools := d.registry.All()
	var available []*Tool

	for _, tool := range tools {
		if d.IsAvailable(tool.Name) {
			available = append(available, tool)
		}
	}

	return available
}

// Reset clears the detection cache.
//
// Call this if PATH changes or tools are installed/removed.
func (d *Detector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache = make(map[string]*DetectionResult)
}

// Summary returns a summary of tool availability.
func (d *Detector) Summary() *DetectionSummary {
	tools := d.registry.All()
	summary := &DetectionSummary{
		Total:        len(tools),
		ByCategory:   make(map[ToolCategory]CategorySummary),
		MissingTools: make([]*Tool, 0),
	}

	for _, tool := range tools {
		if d.IsAvailable(tool.Name) {
			summary.Available++
		} else {
			summary.Missing++
			summary.MissingTools = append(summary.MissingTools, tool)
		}

		// Update category summaries
		for _, cat := range tool.Categories {
			cs := summary.ByCategory[cat]
			cs.Total++
			if d.IsAvailable(tool.Name) {
				cs.Available++
			} else {
				cs.Missing++
			}
			summary.ByCategory[cat] = cs
		}
	}

	return summary
}

// DetectionSummary provides an overview of tool detection results.
type DetectionSummary struct {
	Total        int
	Available    int
	Missing      int
	ByCategory   map[ToolCategory]CategorySummary
	MissingTools []*Tool
}

// CategorySummary provides detection summary for a category.
type CategorySummary struct {
	Total     int
	Available int
	Missing   int
}

// AvailabilityPercent returns the percentage of available tools.
func (s *DetectionSummary) AvailabilityPercent() float64 {
	if s.Total == 0 {
		return 100.0
	}
	return float64(s.Available) / float64(s.Total) * 100.0
}

// LookupPaths returns the current PATH directories.
//
// This is useful for debugging tool detection issues.
func LookupPaths() []string {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil
	}

	separator := string(os.PathListSeparator)
	paths := strings.Split(pathEnv, separator)

	// Filter out empty and non-existent paths
	var result []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			result = append(result, p)
		}
	}

	return result
}

// FindInPath searches for a binary in the given path directories.
//
// This is a lower-level function that doesn't use the registry.
func FindInPath(binary string, paths []string) (string, bool) {
	for _, dir := range paths {
		full := filepath.Join(dir, binary)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			return full, true
		}

		// Try with extensions on Windows
		if runtime.GOOS == "windows" {
			extensions := []string{".exe", ".cmd", ".bat", ".com"}
			for _, ext := range extensions {
				full := filepath.Join(dir, binary+ext)
				if info, err := os.Stat(full); err == nil && !info.IsDir() {
					return full, true
				}
			}
		}
	}

	return "", false
}
