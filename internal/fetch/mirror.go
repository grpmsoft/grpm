package fetch

import (
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
)

// DefaultMirrors contains the default Gentoo mirror list.
//
// These mirrors are used when GENTOO_MIRRORS is not configured.
// The list includes geographically diverse mirrors for good availability.
var DefaultMirrors = []string{
	"https://distfiles.gentoo.org/",
	"https://gentoo.osuosl.org/distfiles/",
	"https://mirrors.rit.edu/gentoo/distfiles/",
}

// MirrorStats tracks success/failure statistics for a mirror.
type MirrorStats struct {
	// Successes is the number of successful downloads from this mirror
	Successes int

	// Failures is the number of failed downloads from this mirror
	Failures int
}

// Score returns a score for mirror prioritization.
//
// Higher score = better mirror. Formula: successes - (failures * 2)
// Failed mirrors are penalized more heavily than successful ones are rewarded.
func (s MirrorStats) Score() int {
	return s.Successes - (s.Failures * 2)
}

// MirrorSelector manages a list of mirrors and tracks their reliability.
//
// MirrorSelector is thread-safe and can be used concurrently.
// It tracks success/failure statistics to prioritize reliable mirrors.
type MirrorSelector struct {
	mu      sync.RWMutex
	mirrors []string
	stats   map[string]*MirrorStats
}

// NewMirrorSelector creates a new MirrorSelector with the given mirrors.
//
// If mirrors is empty, DefaultMirrors will be used.
// Mirror URLs are normalized (trailing slash added if missing).
func NewMirrorSelector(mirrors []string) *MirrorSelector {
	if len(mirrors) == 0 {
		mirrors = DefaultMirrors
	}

	// Normalize mirror URLs
	normalized := make([]string, 0, len(mirrors))
	for _, m := range mirrors {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		// Ensure trailing slash
		if !strings.HasSuffix(m, "/") {
			m += "/"
		}
		normalized = append(normalized, m)
	}

	stats := make(map[string]*MirrorStats, len(normalized))
	for _, m := range normalized {
		stats[m] = &MirrorStats{}
	}

	return &MirrorSelector{
		mirrors: normalized,
		stats:   stats,
	}
}

// GetMirrors returns the current list of mirrors ordered by reliability.
//
// Mirrors with higher success rates are returned first.
func (m *MirrorSelector) GetMirrors() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Copy mirrors slice
	mirrors := make([]string, len(m.mirrors))
	copy(mirrors, m.mirrors)

	// Sort by score (higher score first)
	sort.Slice(mirrors, func(i, j int) bool {
		return m.stats[mirrors[i]].Score() > m.stats[mirrors[j]].Score()
	})

	return mirrors
}

// GetURIs returns download URIs for a filename from all mirrors.
//
// The URIs are returned in order of mirror reliability (best first).
// The filename is appended to each mirror's distfiles path.
//
// Example:
//
//	selector.GetURIs("hello-2.10.tar.gz")
//	// Returns: ["https://distfiles.gentoo.org/distfiles/hello-2.10.tar.gz", ...]
func (m *MirrorSelector) GetURIs(filename string) []string {
	mirrors := m.GetMirrors()
	uris := make([]string, 0, len(mirrors))

	for _, mirror := range mirrors {
		uri := buildDistfileURI(mirror, filename)
		if uri != "" {
			uris = append(uris, uri)
		}
	}

	return uris
}

// ReportSuccess records a successful download from a mirror.
//
// This improves the mirror's priority in future selections.
func (m *MirrorSelector) ReportSuccess(mirror string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mirror = normalizeMirror(mirror)
	if stats, ok := m.stats[mirror]; ok {
		stats.Successes++
	}
}

// ReportFailure records a failed download from a mirror.
//
// This decreases the mirror's priority in future selections.
func (m *MirrorSelector) ReportFailure(mirror string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mirror = normalizeMirror(mirror)
	if stats, ok := m.stats[mirror]; ok {
		stats.Failures++
	}
}

// GetStats returns the current statistics for a mirror.
//
// Returns nil if the mirror is not in the selector.
func (m *MirrorSelector) GetStats(mirror string) *MirrorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mirror = normalizeMirror(mirror)
	if stats, ok := m.stats[mirror]; ok {
		// Return a copy to prevent external mutation
		return &MirrorStats{
			Successes: stats.Successes,
			Failures:  stats.Failures,
		}
	}
	return nil
}

// AddMirror adds a new mirror to the selector.
//
// If the mirror already exists, this is a no-op.
func (m *MirrorSelector) AddMirror(mirror string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mirror = normalizeMirror(mirror)
	if mirror == "" {
		return
	}

	// Check if already exists
	for _, existing := range m.mirrors {
		if existing == mirror {
			return
		}
	}

	m.mirrors = append(m.mirrors, mirror)
	m.stats[mirror] = &MirrorStats{}
}

// Len returns the number of mirrors in the selector.
func (m *MirrorSelector) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.mirrors)
}

// buildDistfileURI constructs the full URI for a distfile.
//
// Handles mirror URLs that may or may not already include "distfiles/".
func buildDistfileURI(mirror, filename string) string {
	// Parse the mirror URL to check its path
	u, err := url.Parse(mirror)
	if err != nil {
		return ""
	}

	// Check if mirror path already ends with "distfiles/"
	mirrorPath := u.Path
	if !strings.HasSuffix(mirrorPath, "distfiles/") && !strings.HasSuffix(mirrorPath, "distfiles") {
		// Append distfiles/ to the path
		mirrorPath = path.Join(mirrorPath, "distfiles")
	}

	// Build the final URI
	u.Path = path.Join(mirrorPath, filename)
	return u.String()
}

// normalizeMirror normalizes a mirror URL.
func normalizeMirror(mirror string) string {
	mirror = strings.TrimSpace(mirror)
	if mirror == "" {
		return ""
	}
	if !strings.HasSuffix(mirror, "/") {
		mirror += "/"
	}
	return mirror
}

// ExtractMirrorBase extracts the base mirror URL from a full distfile URI.
//
// This is used for reporting success/failure back to the selector.
//
// Example:
//
//	ExtractMirrorBase("https://distfiles.gentoo.org/distfiles/hello.tar.gz")
//	// Returns: "https://distfiles.gentoo.org/"
func ExtractMirrorBase(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}

	// Return scheme + host with trailing slash
	return u.Scheme + "://" + u.Host + "/"
}
