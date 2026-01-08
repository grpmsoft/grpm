package binpkg

import (
	"fmt"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// SelectionStrategy determines how to choose between binary and source packages.
type SelectionStrategy int

const (
	// StrategyPreferBinary prefers binary packages when available and compatible
	StrategyPreferBinary SelectionStrategy = iota

	// StrategyPreferSource prefers building from source
	StrategyPreferSource

	// StrategyBinaryOnly only uses binary packages (fails if not available)
	StrategyBinaryOnly

	// StrategySourceOnly always builds from source
	StrategySourceOnly

	// StrategyAuto automatically decides based on various factors
	StrategyAuto
)

// String returns string representation of selection strategy.
func (s SelectionStrategy) String() string {
	switch s {
	case StrategyPreferBinary:
		return "prefer-binary"
	case StrategyPreferSource:
		return "prefer-source"
	case StrategyBinaryOnly:
		return "binary-only"
	case StrategySourceOnly:
		return "source-only"
	case StrategyAuto:
		return "auto"
	default:
		return "unknown"
	}
}

// SelectorOptions configures package selection behavior.
type SelectorOptions struct {
	// Strategy is the selection strategy
	Strategy SelectionStrategy

	// RequiredUSE are the USE flags that must be present
	RequiredUSE []string

	// MaxAge is the maximum age for binary packages (0 = no limit)
	MaxAge time.Duration

	// MinCoverage is minimum USE flag match percentage (0.0-1.0)
	// A value of 1.0 means all USE flags must match exactly
	MinCoverage float64

	// AllowUntrusted allows packages without valid signatures
	AllowUntrusted bool

	// PreferLocal prefers local binhost over remote
	PreferLocal bool
}

// DefaultSelectorOptions returns default selector options.
func DefaultSelectorOptions() SelectorOptions {
	return SelectorOptions{
		Strategy:       StrategyPreferBinary,
		MaxAge:         30 * 24 * time.Hour, // 30 days
		MinCoverage:    0.9,                 // 90% USE flag match
		AllowUntrusted: true,                // Allow unsigned packages
		PreferLocal:    true,
	}
}

// Selector selects between binary and source packages.
type Selector struct {
	// Options configures selection behavior
	Options SelectorOptions

	// Binhosts is the list of available binhosts
	Binhosts []*Binhost
}

// NewSelector creates a new package selector.
func NewSelector(opts SelectorOptions) *Selector {
	return &Selector{
		Options:  opts,
		Binhosts: []*Binhost{},
	}
}

// AddBinhost adds a binhost to the selector.
func (s *Selector) AddBinhost(binhost *Binhost) {
	s.Binhosts = append(s.Binhosts, binhost)
}

// SelectionResult represents the result of package selection.
type SelectionResult struct {
	// UseBinary indicates whether to use binary package
	UseBinary bool

	// BinaryPackage is the selected binary package (if UseBinary is true)
	BinaryPackage *BinaryPackage

	// Binhost is the binhost containing the package (if UseBinary is true)
	Binhost *Binhost

	// Reason explains why this selection was made
	Reason string

	// Score is the compatibility score (0.0-1.0)
	Score float64
}

// Select chooses between binary and source for the given package.
//
// Returns SelectionResult indicating whether to use binary or source.
func (s *Selector) Select(p *pkg.Package) (*SelectionResult, error) {
	// Handle strategy-specific logic
	switch s.Options.Strategy {
	case StrategySourceOnly:
		return &SelectionResult{
			UseBinary: false,
			Reason:    "strategy is source-only",
			Score:     0.0,
		}, nil

	case StrategyBinaryOnly:
		// Must find binary package
		result, err := s.findBestBinary(p)
		if err != nil || result == nil {
			return nil, fmt.Errorf("binary-only strategy but no compatible binary found")
		}
		return result, nil

	case StrategyPreferBinary:
		// Try binary first, fall back to source
		result, err := s.findBestBinary(p)
		if err != nil || result == nil {
			return &SelectionResult{
				UseBinary: false,
				Reason:    "no compatible binary found",
				Score:     0.0,
			}, nil
		}
		return result, nil

	case StrategyPreferSource:
		// Check if binary is significantly better
		result, err := s.findBestBinary(p)
		if err != nil || result == nil || result.Score < 0.95 {
			return &SelectionResult{
				UseBinary: false,
				Reason:    "prefer source unless binary is excellent match",
				Score:     0.0,
			}, nil
		}
		return result, nil

	case StrategyAuto:
		// Intelligent selection based on multiple factors
		return s.autoSelect(p)

	default:
		return nil, fmt.Errorf("unknown selection strategy: %d", s.Options.Strategy)
	}
}

// findBestBinary finds the best matching binary package from all binhosts.
func (s *Selector) findBestBinary(p *pkg.Package) (*SelectionResult, error) {
	var bestMatch *SelectionResult

	// Search all binhosts
	for _, binhost := range s.Binhosts {
		// Find packages matching atom
		candidates := binhost.Find(p.Name)

		// Score each candidate
		for _, candidate := range candidates {
			score := s.scorePackage(candidate, p)

			// Check if it meets minimum requirements
			if score < s.Options.MinCoverage {
				continue
			}

			// Check signature if required
			if !s.Options.AllowUntrusted && candidate.Signature == nil {
				continue
			}

			// Check age
			if s.Options.MaxAge > 0 && !candidate.IsFresh(s.Options.MaxAge) {
				continue
			}

			// Update best match
			if bestMatch == nil || score > bestMatch.Score {
				bestMatch = &SelectionResult{
					UseBinary:     true,
					BinaryPackage: candidate,
					Binhost:       binhost,
					Reason:        fmt.Sprintf("compatible binary found (score: %.2f)", score),
					Score:         score,
				}
			}
		}
	}

	return bestMatch, nil
}

// scorePackage calculates compatibility score between binary package and requirements.
//
// Returns a score from 0.0 (incompatible) to 1.0 (perfect match).
func (s *Selector) scorePackage(binPkg *BinaryPackage, p *pkg.Package) float64 {
	if binPkg.BuildInfo == nil {
		return 0.0
	}

	// Check USE flag compatibility
	if !binPkg.IsCompatible(s.Options.RequiredUSE) {
		return 0.0
	}

	// Calculate USE flag coverage
	// Score = (matching flags) / (total required flags)
	if len(s.Options.RequiredUSE) == 0 {
		return 1.0 // No specific requirements
	}

	buildUSE := make(map[string]bool)
	for _, flag := range binPkg.BuildInfo.USE {
		buildUSE[flag] = true
	}

	matchingFlags := 0
	for _, requiredFlag := range s.Options.RequiredUSE {
		// Skip negative flags for scoring
		flag := requiredFlag
		if flag[0] == '-' {
			flag = flag[1:]
		}

		if buildUSE[flag] {
			matchingFlags++
		}
	}

	coverage := float64(matchingFlags) / float64(len(s.Options.RequiredUSE))

	// Adjust score based on age (fresher packages get higher score)
	if s.Options.MaxAge > 0 {
		age := time.Since(binPkg.BuildInfo.BuildDate)
		agePenalty := float64(age) / float64(s.Options.MaxAge)
		if agePenalty > 1.0 {
			agePenalty = 1.0
		}
		coverage *= (1.0 - agePenalty*0.2) // Up to 20% penalty for age
	}

	return coverage
}

// autoSelect intelligently selects between binary and source.
//
// Considers factors like:
//   - USE flag compatibility
//   - Package age
//   - Build time estimate
//   - Available resources
func (s *Selector) autoSelect(p *pkg.Package) (*SelectionResult, error) {
	// Try to find binary
	result, err := s.findBestBinary(p)
	if err != nil {
		return nil, err
	}

	// No binary found - use source
	if result == nil {
		return &SelectionResult{
			UseBinary: false,
			Reason:    "no binary package available",
			Score:     0.0,
		}, nil
	}

	// Perfect match - use binary
	if result.Score >= 0.95 {
		return result, nil
	}

	// Good match - use binary
	if result.Score >= s.Options.MinCoverage {
		return result, nil
	}

	// Poor match - use source
	return &SelectionResult{
		UseBinary: false,
		Reason:    fmt.Sprintf("binary score too low (%.2f < %.2f)", result.Score, s.Options.MinCoverage),
		Score:     0.0,
	}, nil
}

// SyncAll synchronizes all binhosts.
func (s *Selector) SyncAll() error {
	for _, binhost := range s.Binhosts {
		if err := binhost.Sync(); err != nil {
			return fmt.Errorf("failed to sync binhost %s: %w", binhost.URI, err)
		}
	}
	return nil
}

// Stats returns statistics about available binary packages.
type Stats struct {
	TotalBinhosts  int
	TotalPackages  int
	LocalPackages  int
	RemotePackages int
	OldestPackage  time.Time
	NewestPackage  time.Time
	AverageAge     time.Duration
}

// GetStats returns statistics about available binary packages.
func (s *Selector) GetStats() Stats {
	stats := Stats{
		TotalBinhosts: len(s.Binhosts),
	}

	var totalAge time.Duration
	packageCount := 0

	for _, binhost := range s.Binhosts {
		stats.TotalPackages += len(binhost.Packages)

		if binhost.Type == BinhostLocal {
			stats.LocalPackages += len(binhost.Packages)
		} else {
			stats.RemotePackages += len(binhost.Packages)
		}

		for _, pkg := range binhost.Packages {
			if pkg.BuildInfo == nil {
				continue
			}

			buildDate := pkg.BuildInfo.BuildDate
			packageCount++

			totalAge += time.Since(buildDate)

			if stats.OldestPackage.IsZero() || buildDate.Before(stats.OldestPackage) {
				stats.OldestPackage = buildDate
			}

			if stats.NewestPackage.IsZero() || buildDate.After(stats.NewestPackage) {
				stats.NewestPackage = buildDate
			}
		}
	}

	if packageCount > 0 {
		stats.AverageAge = totalAge / time.Duration(packageCount)
	}

	return stats
}
