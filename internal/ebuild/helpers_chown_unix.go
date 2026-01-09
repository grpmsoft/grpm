//go:build unix

// Package ebuild implements ebuild execution engine.
//
// This file provides Unix-specific chown implementation for fowners helper.
package ebuild

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// parseOwnerGroup parses owner:group string into UID and GID.
//
// Formats supported:
//   - "owner:group" - lookup both by name
//   - "owner" - lookup owner, keep group unchanged (-1)
//   - ":group" - keep owner unchanged (-1), lookup group
//   - "1000:1000" - numeric UID:GID
//
// Per Portage behavior: unknown users/groups cause die().
func parseOwnerGroup(ownerGroup string) (uid, gid int, err error) {
	uid = -1 // -1 means "don't change" in os.Chown
	gid = -1

	parts := strings.SplitN(ownerGroup, ":", 2)
	owner := parts[0]
	group := ""
	if len(parts) > 1 {
		group = parts[1]
	}

	// Parse owner
	if owner != "" {
		uid, err = lookupUID(owner)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid user '%s': %w", owner, err)
		}
	}

	// Parse group
	if group != "" {
		gid, err = lookupGID(group)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid group '%s': %w", group, err)
		}
	}

	return uid, gid, nil
}

// lookupUID returns UID for username or parses numeric UID.
func lookupUID(name string) (int, error) {
	// Try numeric first
	if uid, err := strconv.Atoi(name); err == nil {
		return uid, nil
	}

	// Lookup by name
	u, err := user.Lookup(name)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(u.Uid)
}

// lookupGID returns GID for group name or parses numeric GID.
func lookupGID(name string) (int, error) {
	// Try numeric first
	if gid, err := strconv.Atoi(name); err == nil {
		return gid, nil
	}

	// Lookup by name
	g, err := user.LookupGroup(name)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(g.Gid)
}

// chownPath changes ownership of a path, optionally recursively.
//
// Uses os.Lchown for symlinks (doesn't follow) and os.Chown for regular files.
// This matches Portage's fowners behavior.
func chownPath(path string, uid, gid int, recursive bool) error {
	if !recursive {
		return os.Lchown(path, uid, gid)
	}

	return filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Use Lchown to not follow symlinks
		return os.Lchown(p, uid, gid)
	})
}

// chownSupported returns true on Unix systems.
func chownSupported() bool {
	return true
}
