package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// SQLiteCache implements Cache using SQLite for persistent storage.
// Uses modernc.org/sqlite which is pure Go (no CGO required).
type SQLiteCache struct {
	db     *sql.DB
	path   string
	closed atomic.Bool

	// Prepared statements for common operations
	stmtGet    *sql.Stmt
	stmtPut    *sql.Stmt
	stmtDelete *sql.Stmt

	// Statistics
	mu     sync.RWMutex
	hits   int64
	misses int64
}

// NewSQLiteCache creates a new SQLite-backed cache.
// Creates the database file and schema if they don't exist.
func NewSQLiteCache(path string) (*SQLiteCache, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}

	// Open database with WAL mode for concurrent reads
	dsn := fmt.Sprintf("file:%s?_journal=WAL&_sync=NORMAL&_busy_timeout=5000", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite is single-writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // Keep connection alive

	// Verify connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	c := &SQLiteCache{
		db:   db,
		path: path,
	}

	// Initialize schema
	if err := c.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	// Prepare statements
	if err := c.prepareStatements(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("preparing statements: %w", err)
	}

	return c, nil
}

// initSchema creates the database schema if it doesn't exist.
func (c *SQLiteCache) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS metadata (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category TEXT NOT NULL,
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			eapi TEXT,
			slot TEXT,
			subslot TEXT,
			keywords TEXT,
			iuse TEXT,
			use_flags TEXT,
			license TEXT,
			description TEXT,
			homepage TEXT,
			depend TEXT,
			rdepend TEXT,
			bdepend TEXT,
			pdepend TEXT,
			src_uri TEXT,
			ebuild_mtime INTEGER,
			cached_at INTEGER,
			UNIQUE(category, name, version)
		);

		CREATE INDEX IF NOT EXISTS idx_category_name ON metadata(category, name);
		CREATE INDEX IF NOT EXISTS idx_cached_at ON metadata(cached_at);
	`

	_, err := c.db.Exec(schema)
	return err
}

// prepareStatements creates prepared statements for common operations.
func (c *SQLiteCache) prepareStatements() error {
	var err error

	c.stmtGet, err = c.db.Prepare(`
		SELECT category, name, version, eapi, slot, subslot,
			   keywords, iuse, use_flags, license, description, homepage,
			   depend, rdepend, bdepend, pdepend, src_uri,
			   ebuild_mtime, cached_at
		FROM metadata
		WHERE category = ? AND name = ? AND version = ?
	`)
	if err != nil {
		return fmt.Errorf("preparing get statement: %w", err)
	}

	c.stmtPut, err = c.db.Prepare(`
		INSERT OR REPLACE INTO metadata (
			category, name, version, eapi, slot, subslot,
			keywords, iuse, use_flags, license, description, homepage,
			depend, rdepend, bdepend, pdepend, src_uri,
			ebuild_mtime, cached_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing put statement: %w", err)
	}

	c.stmtDelete, err = c.db.Prepare(`
		DELETE FROM metadata
		WHERE category = ? AND name = ? AND version = ?
	`)
	if err != nil {
		return fmt.Errorf("preparing delete statement: %w", err)
	}

	return nil
}

// Get retrieves cached metadata for a package.
func (c *SQLiteCache) Get(ctx context.Context, category, name, version string) (*Entry, error) {
	if c.closed.Load() {
		return nil, ErrCacheClosed
	}

	entry := &Entry{}
	var keywordsJSON, iuseJSON, useJSON, srcURIJSON sql.NullString
	var ebuildMtime, cachedAt int64

	err := c.stmtGet.QueryRowContext(ctx, category, name, version).Scan(
		&entry.Category, &entry.Name, &entry.Version,
		&entry.EAPI, &entry.Slot, &entry.SubSlot,
		&keywordsJSON, &iuseJSON, &useJSON,
		&entry.License, &entry.Description, &entry.Homepage,
		&entry.Depend, &entry.RDepend, &entry.BDepend, &entry.PDepend,
		&srcURIJSON, &ebuildMtime, &cachedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying cache: %w", err)
	}

	// Parse JSON arrays
	if keywordsJSON.Valid {
		if err := json.Unmarshal([]byte(keywordsJSON.String), &entry.Keywords); err != nil {
			entry.Keywords = nil
		}
	}
	if iuseJSON.Valid {
		if err := json.Unmarshal([]byte(iuseJSON.String), &entry.IUSE); err != nil {
			entry.IUSE = nil
		}
	}
	if useJSON.Valid {
		if err := json.Unmarshal([]byte(useJSON.String), &entry.Use); err != nil {
			entry.Use = nil
		}
	}
	if srcURIJSON.Valid {
		if err := json.Unmarshal([]byte(srcURIJSON.String), &entry.SrcURI); err != nil {
			entry.SrcURI = nil
		}
	}

	// Convert timestamps
	entry.EbuildMtime = time.Unix(ebuildMtime, 0)
	entry.CachedAt = time.Unix(cachedAt, 0)

	c.mu.Lock()
	c.hits++
	c.mu.Unlock()

	return entry, nil
}

// Put stores metadata for a package.
func (c *SQLiteCache) Put(ctx context.Context, entry *Entry) error {
	if c.closed.Load() {
		return ErrCacheClosed
	}

	if entry == nil {
		return ErrInvalidEntry
	}

	// Set cached_at if not set
	if entry.CachedAt.IsZero() {
		entry.CachedAt = time.Now()
	}

	// Serialize arrays to JSON
	keywordsJSON, _ := json.Marshal(entry.Keywords)
	iuseJSON, _ := json.Marshal(entry.IUSE)
	useJSON, _ := json.Marshal(entry.Use)
	srcURIJSON, _ := json.Marshal(entry.SrcURI)

	_, err := c.stmtPut.ExecContext(ctx,
		entry.Category, entry.Name, entry.Version,
		entry.EAPI, entry.Slot, entry.SubSlot,
		string(keywordsJSON), string(iuseJSON), string(useJSON),
		entry.License, entry.Description, entry.Homepage,
		entry.Depend, entry.RDepend, entry.BDepend, entry.PDepend,
		string(srcURIJSON),
		entry.EbuildMtime.Unix(), entry.CachedAt.Unix(),
	)

	if err != nil {
		return fmt.Errorf("storing cache entry: %w", err)
	}

	return nil
}

// PutBatch stores multiple entries efficiently using a transaction.
func (c *SQLiteCache) PutBatch(ctx context.Context, entries []*Entry) error {
	if c.closed.Load() {
		return ErrCacheClosed
	}

	if len(entries) == 0 {
		return nil
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// Prepare statement within transaction
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO metadata (
			category, name, version, eapi, slot, subslot,
			keywords, iuse, use_flags, license, description, homepage,
			depend, rdepend, bdepend, pdepend, src_uri,
			ebuild_mtime, cached_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing batch statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	now := time.Now()

	for _, entry := range entries {
		if entry == nil {
			continue
		}

		if entry.CachedAt.IsZero() {
			entry.CachedAt = now
		}

		keywordsJSON, _ := json.Marshal(entry.Keywords)
		iuseJSON, _ := json.Marshal(entry.IUSE)
		useJSON, _ := json.Marshal(entry.Use)
		srcURIJSON, _ := json.Marshal(entry.SrcURI)

		_, err := stmt.ExecContext(ctx,
			entry.Category, entry.Name, entry.Version,
			entry.EAPI, entry.Slot, entry.SubSlot,
			string(keywordsJSON), string(iuseJSON), string(useJSON),
			entry.License, entry.Description, entry.Homepage,
			entry.Depend, entry.RDepend, entry.BDepend, entry.PDepend,
			string(srcURIJSON),
			entry.EbuildMtime.Unix(), entry.CachedAt.Unix(),
		)
		if err != nil {
			return fmt.Errorf("inserting entry %s: %w", entry.Key(), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	tx = nil // Prevent rollback in defer

	return nil
}

// Delete removes cached metadata for a package.
func (c *SQLiteCache) Delete(ctx context.Context, category, name, version string) error {
	if c.closed.Load() {
		return ErrCacheClosed
	}

	_, err := c.stmtDelete.ExecContext(ctx, category, name, version)
	if err != nil {
		return fmt.Errorf("deleting cache entry: %w", err)
	}

	return nil
}

// Invalidate removes all entries older than given mtime for a package.
func (c *SQLiteCache) Invalidate(ctx context.Context, category, name string, mtime time.Time) error {
	if c.closed.Load() {
		return ErrCacheClosed
	}

	_, err := c.db.ExecContext(ctx,
		`DELETE FROM metadata WHERE category = ? AND name = ? AND ebuild_mtime < ?`,
		category, name, mtime.Unix(),
	)
	if err != nil {
		return fmt.Errorf("invalidating cache entries: %w", err)
	}

	return nil
}

// InvalidateAll removes all entries from the cache.
func (c *SQLiteCache) InvalidateAll(ctx context.Context) error {
	if c.closed.Load() {
		return ErrCacheClosed
	}

	_, err := c.db.ExecContext(ctx, `DELETE FROM metadata`)
	if err != nil {
		return fmt.Errorf("clearing cache: %w", err)
	}

	// Reset statistics
	c.mu.Lock()
	c.hits = 0
	c.misses = 0
	c.mu.Unlock()

	return nil
}

// Stats returns cache statistics.
func (c *SQLiteCache) Stats() Stats {
	c.mu.RLock()
	hits := c.hits
	misses := c.misses
	c.mu.RUnlock()

	var count int64
	var size int64

	if !c.closed.Load() {
		// Get entry count
		row := c.db.QueryRow(`SELECT COUNT(*) FROM metadata`)
		_ = row.Scan(&count)

		// Get approximate size from SQLite page info
		row = c.db.QueryRow(`SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()`)
		_ = row.Scan(&size)
	}

	return Stats{
		Hits:       hits,
		Misses:     misses,
		Entries:    count,
		Size:       size,
		LastUpdate: time.Now(),
	}
}

// Close closes the cache and releases resources.
func (c *SQLiteCache) Close() error {
	if c.closed.Swap(true) {
		return nil // Already closed
	}

	var errs []error

	if c.stmtGet != nil {
		if err := c.stmtGet.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.stmtPut != nil {
		if err := c.stmtPut.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.stmtDelete != nil {
		if err := c.stmtDelete.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.db != nil {
		if err := c.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("closing cache: %v", errs)
	}

	return nil
}
