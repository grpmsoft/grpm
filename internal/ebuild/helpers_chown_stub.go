//go:build !unix

// Package ebuild implements ebuild execution engine.
//
// This file provides stub chown implementation for non-Unix systems.
package ebuild

// parseOwnerGroup is a stub on non-Unix systems.
func parseOwnerGroup(_ string) (uid, gid int, err error) {
	return -1, -1, nil
}

// chownPath is a stub on non-Unix systems.
func chownPath(_ string, _, _ int, _ bool) error {
	return nil
}

// chownSupported returns false on non-Unix systems.
func chownSupported() bool {
	return false
}
