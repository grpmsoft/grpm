// Package cache provides persistent storage for ebuild metadata to speed up
// repository queries and dependency resolution.
package cache

import (
	"time"
)

// Entry represents cached ebuild metadata.
// Contains all parsed ebuild variables needed for dependency resolution
// without re-parsing the ebuild file.
type Entry struct {
	// Package identification
	Category string
	Name     string
	Version  string

	// Core metadata fields from ebuild
	EAPI        string
	Slot        string
	SubSlot     string
	Keywords    []string
	IUSE        []string
	Use         []string
	License     string
	Description string
	Homepage    string

	// Dependencies (raw strings, parsed on demand)
	Depend  string
	RDepend string
	BDepend string
	PDepend string

	// SRC_URI entries
	SrcURI []string

	// Cache metadata
	EbuildMtime time.Time // Mtime of source ebuild file
	CachedAt    time.Time // When this entry was cached
}

// Key returns the unique cache key for this entry.
// Format: "category/name-version"
func (e *Entry) Key() string {
	return e.Category + "/" + e.Name + "-" + e.Version
}

// Atom returns the package atom without version.
// Format: "category/name"
func (e *Entry) Atom() string {
	return e.Category + "/" + e.Name
}

// IsValid checks if the cache entry is still valid based on ebuild mtime.
// Returns false if the ebuild has been modified since caching.
func (e *Entry) IsValid(currentMtime time.Time) bool {
	// Cache is valid if ebuild mtime matches
	return e.EbuildMtime.Equal(currentMtime)
}

// IsExpired checks if the cache entry is older than the given duration.
func (e *Entry) IsExpired(maxAge time.Duration) bool {
	return time.Since(e.CachedAt) > maxAge
}

// Stats holds cache statistics.
type Stats struct {
	Hits       int64 // Number of cache hits
	Misses     int64 // Number of cache misses
	Entries    int64 // Total entries in cache
	Size       int64 // Size in bytes (approximate)
	LastUpdate time.Time
}

// HitRate returns the cache hit rate as a percentage.
func (s *Stats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0.0
	}
	return float64(s.Hits) / float64(total) * 100.0
}
