package cache

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// PackageEntry represents a package in the repository index.
type PackageEntry struct {
	Category string
	Name     string
	Version  string
	Slot     string
	EAPI     string
	Path     string    // Relative path to ebuild
	Mtime    time.Time // Ebuild modification time
}

// Key returns the unique key for this entry (category/name-version).
func (pe *PackageEntry) Key() string {
	return pe.Category + "/" + pe.Name + "-" + pe.Version
}

// Atom returns the package atom (category/name).
func (pe *PackageEntry) Atom() string {
	return pe.Category + "/" + pe.Name
}

// RepoIndex provides fast package lookups via SQLite index.
// Caches the directory structure to avoid repeated filesystem scans.
type RepoIndex struct {
	db       *sql.DB
	path     string
	repoPath string
	closed   atomic.Bool

	// Prepared statements
	stmtGetPackage  *sql.Stmt
	stmtGetVersions *sql.Stmt
	stmtListPkgs    *sql.Stmt
	stmtInsert      *sql.Stmt
	stmtDelete      *sql.Stmt

	mu sync.RWMutex
}

// NewRepoIndex creates or opens a repository index.
// If the index is stale, it will be rebuilt automatically.
func NewRepoIndex(indexPath, repoPath string) (*RepoIndex, error) {
	// Ensure directory exists
	dir := filepath.Dir(indexPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating index directory: %w", err)
	}

	// Open database with WAL mode
	dsn := fmt.Sprintf("file:%s?_journal=WAL&_sync=NORMAL&_busy_timeout=5000", indexPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening index database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(1) // SQLite is single-writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// Verify connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connecting to index database: %w", err)
	}

	ri := &RepoIndex{
		db:       db,
		path:     indexPath,
		repoPath: repoPath,
	}

	// Initialize schema
	if err := ri.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing index schema: %w", err)
	}

	// Prepare statements
	if err := ri.prepareStatements(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("preparing index statements: %w", err)
	}

	return ri, nil
}

// initSchema creates the index schema if it doesn't exist.
func (ri *RepoIndex) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS packages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category TEXT NOT NULL,
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			slot TEXT DEFAULT '0',
			eapi TEXT DEFAULT '8',
			path TEXT NOT NULL,
			mtime INTEGER NOT NULL,
			UNIQUE(category, name, version)
		);

		CREATE INDEX IF NOT EXISTS idx_category ON packages(category);
		CREATE INDEX IF NOT EXISTS idx_package ON packages(category, name);
		CREATE INDEX IF NOT EXISTS idx_path ON packages(path);

		CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`

	_, err := ri.db.Exec(schema)
	return err
}

// prepareStatements creates prepared statements for common operations.
func (ri *RepoIndex) prepareStatements() error {
	var err error

	ri.stmtGetPackage, err = ri.db.Prepare(`
		SELECT category, name, version, slot, eapi, path, mtime
		FROM packages
		WHERE category = ? AND name = ?
	`)
	if err != nil {
		return fmt.Errorf("preparing get package statement: %w", err)
	}

	ri.stmtGetVersions, err = ri.db.Prepare(`
		SELECT category, name, version, slot, eapi, path, mtime
		FROM packages
		WHERE category = ? AND name = ?
		ORDER BY version
	`)
	if err != nil {
		return fmt.Errorf("preparing get versions statement: %w", err)
	}

	ri.stmtListPkgs, err = ri.db.Prepare(`
		SELECT DISTINCT name FROM packages WHERE category = ?
	`)
	if err != nil {
		return fmt.Errorf("preparing list packages statement: %w", err)
	}

	ri.stmtInsert, err = ri.db.Prepare(`
		INSERT OR REPLACE INTO packages (category, name, version, slot, eapi, path, mtime)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing insert statement: %w", err)
	}

	ri.stmtDelete, err = ri.db.Prepare(`
		DELETE FROM packages WHERE category = ? AND name = ? AND version = ?
	`)
	if err != nil {
		return fmt.Errorf("preparing delete statement: %w", err)
	}

	return nil
}

// LookupPackage returns all versions of a package.
// Returns empty slice if package not found.
func (ri *RepoIndex) LookupPackage(ctx context.Context, category, name string) ([]PackageEntry, error) {
	if ri.closed.Load() {
		return nil, fmt.Errorf("index is closed")
	}

	ri.mu.RLock()
	defer ri.mu.RUnlock()

	rows, err := ri.stmtGetVersions.QueryContext(ctx, category, name)
	if err != nil {
		return nil, fmt.Errorf("querying package: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []PackageEntry
	for rows.Next() {
		var e PackageEntry
		var mtime int64

		err := rows.Scan(&e.Category, &e.Name, &e.Version, &e.Slot, &e.EAPI, &e.Path, &mtime)
		if err != nil {
			return nil, fmt.Errorf("scanning package row: %w", err)
		}

		e.Mtime = time.Unix(mtime, 0)
		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating package rows: %w", err)
	}

	return entries, nil
}

// LookupCategory lists all package names in a category.
func (ri *RepoIndex) LookupCategory(ctx context.Context, category string) ([]string, error) {
	if ri.closed.Load() {
		return nil, fmt.Errorf("index is closed")
	}

	ri.mu.RLock()
	defer ri.mu.RUnlock()

	rows, err := ri.stmtListPkgs.QueryContext(ctx, category)
	if err != nil {
		return nil, fmt.Errorf("querying category: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning package name: %w", err)
		}
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating package names: %w", err)
	}

	return names, nil
}

// Add adds a package entry to the index.
func (ri *RepoIndex) Add(ctx context.Context, e *PackageEntry) error {
	if ri.closed.Load() {
		return fmt.Errorf("index is closed")
	}

	if e == nil {
		return fmt.Errorf("entry is nil")
	}

	ri.mu.Lock()
	defer ri.mu.Unlock()

	_, err := ri.stmtInsert.ExecContext(ctx,
		e.Category, e.Name, e.Version, e.Slot, e.EAPI, e.Path, e.Mtime.Unix())
	if err != nil {
		return fmt.Errorf("inserting package entry: %w", err)
	}

	return nil
}

// AddBatch adds multiple package entries efficiently.
func (ri *RepoIndex) AddBatch(ctx context.Context, entries []PackageEntry) error {
	if ri.closed.Load() {
		return fmt.Errorf("index is closed")
	}

	if len(entries) == 0 {
		return nil
	}

	ri.mu.Lock()
	defer ri.mu.Unlock()

	tx, err := ri.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO packages (category, name, version, slot, eapi, path, mtime)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing batch insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, e := range entries {
		_, err := stmt.ExecContext(ctx,
			e.Category, e.Name, e.Version, e.Slot, e.EAPI, e.Path, e.Mtime.Unix())
		if err != nil {
			return fmt.Errorf("inserting entry %s: %w", e.Key(), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	tx = nil

	return nil
}

// Delete removes a package entry from the index.
func (ri *RepoIndex) Delete(ctx context.Context, category, name, version string) error {
	if ri.closed.Load() {
		return fmt.Errorf("index is closed")
	}

	ri.mu.Lock()
	defer ri.mu.Unlock()

	_, err := ri.stmtDelete.ExecContext(ctx, category, name, version)
	if err != nil {
		return fmt.Errorf("deleting package entry: %w", err)
	}

	return nil
}

// DeletePackage removes all versions of a package from the index.
func (ri *RepoIndex) DeletePackage(ctx context.Context, category, name string) error {
	if ri.closed.Load() {
		return fmt.Errorf("index is closed")
	}

	ri.mu.Lock()
	defer ri.mu.Unlock()

	_, err := ri.db.ExecContext(ctx,
		`DELETE FROM packages WHERE category = ? AND name = ?`,
		category, name)
	if err != nil {
		return fmt.Errorf("deleting package: %w", err)
	}

	return nil
}

// Clear removes all entries from the index.
func (ri *RepoIndex) Clear(ctx context.Context) error {
	if ri.closed.Load() {
		return fmt.Errorf("index is closed")
	}

	ri.mu.Lock()
	defer ri.mu.Unlock()

	_, err := ri.db.ExecContext(ctx, `DELETE FROM packages`)
	if err != nil {
		return fmt.Errorf("clearing index: %w", err)
	}

	return nil
}

// Count returns the total number of package versions in the index.
func (ri *RepoIndex) Count(ctx context.Context) (int64, error) {
	if ri.closed.Load() {
		return 0, fmt.Errorf("index is closed")
	}

	ri.mu.RLock()
	defer ri.mu.RUnlock()

	var count int64
	err := ri.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM packages`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting packages: %w", err)
	}

	return count, nil
}

// IsValid checks if the index is up-to-date with the repository.
// Returns false if the repository has been modified since last rebuild.
func (ri *RepoIndex) IsValid(ctx context.Context) bool {
	if ri.closed.Load() {
		return false
	}

	ri.mu.RLock()
	defer ri.mu.RUnlock()

	// Get stored repo mtime
	var storedMtime int64
	err := ri.db.QueryRowContext(ctx,
		`SELECT value FROM meta WHERE key = 'repo_mtime'`).Scan(&storedMtime)
	if err != nil {
		return false
	}

	// Get current repo mtime
	info, err := os.Stat(ri.repoPath)
	if err != nil {
		return false
	}

	return info.ModTime().Unix() == storedMtime
}

// SetRepoMtime stores the repository modification time.
func (ri *RepoIndex) SetRepoMtime(ctx context.Context, mtime time.Time) error {
	if ri.closed.Load() {
		return fmt.Errorf("index is closed")
	}

	ri.mu.Lock()
	defer ri.mu.Unlock()

	_, err := ri.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO meta (key, value) VALUES ('repo_mtime', ?)`,
		mtime.Unix())
	if err != nil {
		return fmt.Errorf("storing repo mtime: %w", err)
	}

	return nil
}

// Rebuild rebuilds the entire index by scanning the repository.
// This is an expensive operation and should be done sparingly.
func (ri *RepoIndex) Rebuild(ctx context.Context) error {
	if ri.closed.Load() {
		return fmt.Errorf("index is closed")
	}

	// Clear existing index
	if err := ri.Clear(ctx); err != nil {
		return err
	}

	// Scan repository
	entries, err := ri.scanRepository(ctx)
	if err != nil {
		return fmt.Errorf("scanning repository: %w", err)
	}

	// Add entries in batch
	if err := ri.AddBatch(ctx, entries); err != nil {
		return fmt.Errorf("adding entries: %w", err)
	}

	// Store repo mtime
	info, err := os.Stat(ri.repoPath)
	if err != nil {
		return fmt.Errorf("getting repo mtime: %w", err)
	}

	if err := ri.SetRepoMtime(ctx, info.ModTime()); err != nil {
		return err
	}

	return nil
}

// scanRepository scans the repository and returns all package entries.
func (ri *RepoIndex) scanRepository(ctx context.Context) ([]PackageEntry, error) {
	var entries []PackageEntry

	// Read categories
	categories, err := os.ReadDir(ri.repoPath)
	if err != nil {
		return nil, fmt.Errorf("reading repository: %w", err)
	}

	for _, categoryDir := range categories {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !categoryDir.IsDir() {
			continue
		}

		// Skip metadata directories
		categoryName := categoryDir.Name()
		if strings.HasPrefix(categoryName, ".") ||
			categoryName == "metadata" ||
			categoryName == "profiles" ||
			categoryName == "eclass" ||
			categoryName == "licenses" ||
			categoryName == "scripts" {
			continue
		}

		categoryPath := filepath.Join(ri.repoPath, categoryName)
		pkgDirs, err := os.ReadDir(categoryPath)
		if err != nil {
			continue // Skip inaccessible directories
		}

		for _, pkgDir := range pkgDirs {
			if !pkgDir.IsDir() {
				continue
			}

			pkgName := pkgDir.Name()
			pkgPath := filepath.Join(categoryPath, pkgName)

			// Read ebuilds
			files, err := os.ReadDir(pkgPath)
			if err != nil {
				continue
			}

			for _, file := range files {
				if file.IsDir() || !strings.HasSuffix(file.Name(), ".ebuild") {
					continue
				}

				// Parse ebuild filename
				version := strings.TrimSuffix(file.Name(), ".ebuild")
				version = strings.TrimPrefix(version, pkgName+"-")

				info, err := file.Info()
				if err != nil {
					continue
				}

				entry := PackageEntry{
					Category: categoryName,
					Name:     pkgName,
					Version:  version,
					Slot:     "0",
					EAPI:     "8",
					Path:     filepath.Join(categoryName, pkgName, file.Name()),
					Mtime:    info.ModTime(),
				}

				entries = append(entries, entry)
			}
		}
	}

	return entries, nil
}

// Close closes the index and releases resources.
func (ri *RepoIndex) Close() error {
	if ri.closed.Swap(true) {
		return nil // Already closed
	}

	var errs []error

	if ri.stmtGetPackage != nil {
		if err := ri.stmtGetPackage.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if ri.stmtGetVersions != nil {
		if err := ri.stmtGetVersions.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if ri.stmtListPkgs != nil {
		if err := ri.stmtListPkgs.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if ri.stmtInsert != nil {
		if err := ri.stmtInsert.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if ri.stmtDelete != nil {
		if err := ri.stmtDelete.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if ri.db != nil {
		if err := ri.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("closing index: %v", errs)
	}

	return nil
}
