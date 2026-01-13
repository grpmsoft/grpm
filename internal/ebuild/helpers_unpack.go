// Package ebuild implements ebuild execution engine.
//
// This file provides EAPI 8 archive extraction functions (unpack).
package ebuild

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

// Unpack extracts archives to WORKDIR.
//
// Usage: unpack file.tar.gz
// Usage: unpack ${A}
//
// Supported formats: .tar.gz, .tar.bz2, .tar.xz, .tar, .zip
// Pure Go implementation, no external commands.
func (h *Helpers) Unpack(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "unpack: no files specified"}
	}

	workDir := h.getWorkDir()
	if workDir == "" {
		if h.env != nil {
			workDir = h.env.WORKDIR
		}
		if workDir == "" {
			return &DieError{Message: "unpack: WORKDIR not set"}
		}
	}

	distDir := ""
	if h.env != nil {
		distDir = h.env.DISTDIR
	}
	if distDir == "" {
		distDir = os.Getenv("DISTDIR")
	}

	for _, file := range args {
		// Resolve file path - check DISTDIR first, then relative
		archivePath := file
		if !filepath.IsAbs(file) {
			if distDir != "" {
				candidate := filepath.Join(distDir, file)
				if _, err := os.Stat(candidate); err == nil {
					archivePath = candidate
				}
			}
			// If not found in DISTDIR, try relative to WORKDIR
			if archivePath == file {
				candidate := filepath.Join(workDir, file)
				if _, err := os.Stat(candidate); err == nil {
					archivePath = candidate
				}
			}
		}

		h.writeStdout(fmt.Sprintf(">>> Unpacking %s\n", filepath.Base(archivePath)))

		if err := h.unpackArchive(archivePath, workDir); err != nil {
			return &DieError{Message: fmt.Sprintf("unpack %s: %v", file, err)}
		}
	}

	return nil
}

// unpackArchive extracts a single archive to destination.
func (h *Helpers) unpackArchive(archivePath, destDir string) error {
	// Check file exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return fmt.Errorf("archive not found: %s", archivePath)
	}

	lowerPath := strings.ToLower(archivePath)

	switch {
	case strings.HasSuffix(lowerPath, ".tar.gz") || strings.HasSuffix(lowerPath, ".tgz"):
		return h.unpackTarGz(archivePath, destDir)
	case strings.HasSuffix(lowerPath, ".tar.bz2") || strings.HasSuffix(lowerPath, ".tbz2"):
		return h.unpackTarBz2(archivePath, destDir)
	case strings.HasSuffix(lowerPath, ".tar.xz") || strings.HasSuffix(lowerPath, ".txz"):
		return h.unpackTarXz(archivePath, destDir)
	case strings.HasSuffix(lowerPath, ".tar"):
		return h.unpackTar(archivePath, destDir)
	case strings.HasSuffix(lowerPath, ".zip"):
		return h.unpackZip(archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive format: %s", archivePath)
	}
}

// unpackTarGz extracts a .tar.gz archive.
func (h *Helpers) unpackTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gzReader.Close() }()

	return h.extractTar(tar.NewReader(gzReader), destDir)
}

// unpackTarBz2 extracts a .tar.bz2 archive.
func (h *Helpers) unpackTarBz2(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	bzReader := bzip2.NewReader(file)
	return h.extractTar(tar.NewReader(bzReader), destDir)
}

// unpackTarXz extracts a .tar.xz archive.
func (h *Helpers) unpackTarXz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	xzReader, err := xz.NewReader(file)
	if err != nil {
		return fmt.Errorf("xz reader: %w", err)
	}

	return h.extractTar(tar.NewReader(xzReader), destDir)
}

// unpackTar extracts a plain .tar archive.
func (h *Helpers) unpackTar(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	return h.extractTar(tar.NewReader(file), destDir)
}

// extractTar extracts files from a tar reader to destDir.
//
// This function preserves original file timestamps from the archive,
// which is critical for build systems like automake that use timestamps
// to determine if files need regeneration.
func (h *Helpers) extractTar(tarReader *tar.Reader, destDir string) error {
	// Track directories to set their timestamps after all files are extracted.
	// We must do this in reverse order (deepest first) to avoid parent directory
	// timestamp updates when extracting children.
	type dirEntry struct {
		path    string
		modTime int64 // Unix timestamp
		mode    int64
	}
	var directories []dirEntry

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		// Sanitize path to prevent directory traversal attacks
		target := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("invalid path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
			// Queue directory for timestamp restoration after extraction
			directories = append(directories, dirEntry{
				path:    target,
				modTime: header.ModTime.Unix(),
				mode:    header.Mode,
			})

		case tar.TypeReg:
			if err := h.extractTarFile(tarReader, target, header); err != nil {
				return err
			}

		case tar.TypeSymlink:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}
			// Remove existing symlink if any
			_ = os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("symlink %s: %w", target, err)
			}
			// Note: Symlink timestamps cannot be set portably in Go.
			// This is acceptable as symlinks are rarely used for build dependencies.

		case tar.TypeLink:
			// Hard link
			linkTarget := filepath.Join(destDir, header.Linkname)
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}
			_ = os.Remove(target)
			if err := os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("hardlink %s: %w", target, err)
			}
			// Hard links share inode with target, so timestamp is shared

		default:
			// Skip other types (devices, etc.)
			continue
		}
	}

	// Restore directory timestamps in reverse order (deepest directories first).
	// This is necessary because extracting files into a directory updates its mtime.
	for i := len(directories) - 1; i >= 0; i-- {
		dir := directories[i]
		modTime := unixToTime(dir.modTime)
		if err := os.Chtimes(dir.path, modTime, modTime); err != nil {
			// Log warning but don't fail - directory timestamp is less critical
			h.writeStderr(fmt.Sprintf("!!! Warning: failed to set timestamp on %s: %v\n", dir.path, err))
		}
	}

	return nil
}

// unixToTime converts Unix timestamp to time.Time.
func unixToTime(unix int64) (t time.Time) {
	return time.Unix(unix, 0)
}

// extractTarFile extracts a single regular file from tar.
//
// This function preserves the original modification time from the tar header,
// which is essential for build systems (automake, cmake) that use file timestamps
// to determine rebuild dependencies.
func (h *Helpers) extractTarFile(tarReader *tar.Reader, target string, header *tar.Header) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}

	outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}

	if _, err := io.Copy(outFile, tarReader); err != nil {
		_ = outFile.Close()
		return fmt.Errorf("write %s: %w", target, err)
	}

	// Close file before setting timestamps
	if err := outFile.Close(); err != nil {
		return fmt.Errorf("close %s: %w", target, err)
	}

	// Preserve original modification time from tar archive.
	// This is CRITICAL for automake-based packages that check timestamps
	// to decide if files like configure, Makefile.in need regeneration.
	modTime := header.ModTime
	if err := os.Chtimes(target, modTime, modTime); err != nil {
		return fmt.Errorf("set timestamp %s: %w", target, err)
	}

	return nil
}

// unpackZip extracts a .zip archive.
//
// This function preserves original file timestamps from the archive,
// which is critical for build systems that use timestamps for dependency tracking.
func (h *Helpers) unpackZip(archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	// Track directories to set their timestamps after all files are extracted
	type dirEntry struct {
		path    string
		modTime time.Time
	}
	var directories []dirEntry

	for _, f := range reader.File {
		target := filepath.Join(destDir, f.Name)

		// Sanitize path to prevent directory traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("invalid path: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
			// Queue directory for timestamp restoration after extraction
			directories = append(directories, dirEntry{
				path:    target,
				modTime: f.Modified,
			})
			continue
		}

		if err := h.extractZipFile(f, target); err != nil {
			return err
		}
	}

	// Restore directory timestamps in reverse order (deepest directories first)
	for i := len(directories) - 1; i >= 0; i-- {
		dir := directories[i]
		if err := os.Chtimes(dir.path, dir.modTime, dir.modTime); err != nil {
			h.writeStderr(fmt.Sprintf("!!! Warning: failed to set timestamp on %s: %v\n", dir.path, err))
		}
	}

	return nil
}

// extractZipFile extracts a single file from zip.
//
// This function preserves the original modification time from the zip header,
// which is essential for build systems that use file timestamps for dependency tracking.
func (h *Helpers) extractZipFile(f *zip.File, target string) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}

	src, err := f.Open()
	if err != nil {
		return fmt.Errorf("open zip entry: %w", err)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return fmt.Errorf("write %s: %w", target, err)
	}

	// Close file before setting timestamps
	if err := dst.Close(); err != nil {
		return fmt.Errorf("close %s: %w", target, err)
	}

	// Preserve original modification time from zip archive
	modTime := f.Modified
	if err := os.Chtimes(target, modTime, modTime); err != nil {
		return fmt.Errorf("set timestamp %s: %w", target, err)
	}

	return nil
}
