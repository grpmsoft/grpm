package rsync

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Client is the rsync client for Gentoo repository synchronization.
type Client struct {
	// Options
	Compress bool // Enable zlib compression (-z)
	Delete   bool // Delete extraneous files (--delete)
	Timeout  time.Duration
	Logger   Logger

	// Protocol state
	remoteVersion int
	conn          net.Conn
	wire          *Conn
	mplex         *MultiplexReader
	// NOTE: No mplexW - client writes are always RAW (not multiplexed)
}

// Logger interface for rsync client logging.
type Logger interface {
	Printf(format string, v ...interface{})
}

// defaultLogger is a no-op logger.
type defaultLogger struct{}

func (defaultLogger) Printf(string, ...interface{}) {}

// NewClient creates a new rsync client.
func NewClient() *Client {
	return &Client{
		Compress: true,
		Delete:   true,
		Timeout:  30 * time.Second,
		Logger:   defaultLogger{},
	}
}

// Sync synchronizes a remote rsync URL to a local directory.
// URL format: rsync://host[:port]/module[/path]
func (c *Client) Sync(ctx context.Context, rsyncURL, destDir string) error {
	// Parse URL
	u, err := url.Parse(rsyncURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	if u.Scheme != "rsync" {
		return fmt.Errorf("unsupported scheme: %s (expected rsync)", u.Scheme)
	}

	// Extract host and port
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "873" // Default rsync port
	}

	// Extract module and path
	path := strings.TrimPrefix(u.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return fmt.Errorf("module name required in URL")
	}
	module := parts[0]
	remotePath := ""
	if len(parts) > 1 {
		remotePath = parts[1]
	}

	// Connect
	addr := net.JoinHostPort(host, port)
	c.Logger.Printf("connecting to %s", addr)

	dialer := net.Dialer{Timeout: c.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", addr, err)
	}
	defer func() { _ = conn.Close() }()

	c.conn = conn
	c.wire = NewConn(conn, conn)

	// NOTE: We do NOT set connection deadline from context.
	// For large repository syncs (150k+ files), the operation takes minutes.
	// Context cancellation is handled by closing the connection in the select below.

	// Run protocol with context cancellation
	done := make(chan error, 1)
	go func() {
		done <- c.runProtocol(ctx, module, remotePath, destDir)
	}()

	select {
	case <-ctx.Done():
		// Context canceled/timeout - close connection to unblock goroutine
		_ = conn.Close()
		<-done // Wait for goroutine to finish
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// runProtocol executes the rsync protocol exchange.
func (c *Client) runProtocol(ctx context.Context, module, remotePath, destDir string) error {
	// 1. Exchange greeting
	if err := c.exchangeGreeting(); err != nil {
		return fmt.Errorf("greeting: %w", err)
	}

	// 2. Select module
	if err := c.selectModule(module); err != nil {
		return fmt.Errorf("select module: %w", err)
	}

	// 3. Send arguments
	if err := c.sendArgs(remotePath); err != nil {
		return fmt.Errorf("send args: %w", err)
	}

	// 4. Read checksum seed (raw int32, not multiplexed)
	// See rsync's setup_protocol() - seed is sent BEFORE multiplex is enabled
	seed, err := c.wire.ReadInt32()
	if err != nil {
		return fmt.Errorf("read checksum seed: %w", err)
	}
	c.Logger.Printf("checksum seed: 0x%08x", uint32(seed))

	// 5. Enable multiplexing for protocol >= 23 (daemon mode)
	// After setup_protocol(), server calls io_start_multiplex_in/out for proto >= 23
	// This means:
	// - Server reads from us via demultiplexer (we must send multiplexed data)
	// - Server writes to us via multiplexer (we already have MultiplexReader)
	negotiatedVersion := c.remoteVersion
	if negotiatedVersion > ProtocolVersion {
		negotiatedVersion = ProtocolVersion
	}
	c.Logger.Printf("enabling multiplexing: server=%d, client=%d, negotiated=%d",
		c.remoteVersion, ProtocolVersion, negotiatedVersion)

	// Wrap reader in 256KB bufio for better TCP flow control (critical for large syncs)
	// This matches gokrazy/rsync implementation which uses bufio.NewReaderSize(mrd, 256*1024)
	bufferedReader := bufio.NewReaderSize(c.wire.Reader, 256*1024)
	bufferedConn := NewConn(bufferedReader, c.wire.Writer)
	c.mplex = NewMultiplexReader(bufferedConn)
	c.mplex.SetDebug(c.Logger)

	// 6. Send exclusion list (empty - no filters)
	// Filter list is sent RAW (before server enables multiplex input)
	c.Logger.Printf("sending empty filter list (raw)")
	if err := c.wire.WriteInt32(0); err != nil {
		return fmt.Errorf("write filter terminator: %w", err)
	}

	// NOTE: Client writes remain RAW throughout the protocol!
	// Only server writes are multiplexed (we read via MultiplexReader).
	// This is the key insight from gokrazy/rsync implementation.

	// 7. Receive file list
	files, err := c.receiveFileList()
	if err != nil {
		return fmt.Errorf("receive file list: %w", err)
	}
	c.Logger.Printf("received %d files", len(files))

	// Read io_error after file list
	// This is sent by server after file list is complete
	ioerr, err := c.readMultiplexedInt32()
	if err != nil {
		c.Logger.Printf("warning: read ioerr: %v (continuing anyway)", err)
	} else {
		c.Logger.Printf("server io_error: %d", ioerr)
	}
	c.Logger.Printf("file list complete, starting file transfer")

	// 8. Request and receive files
	if err := c.receiveFiles(ctx, files, destDir); err != nil {
		return fmt.Errorf("receive files: %w", err)
	}

	// 9. Read statistics from server (3 x int64)
	// See gokrazy/rsync report() function
	readBytes, err := c.readMultiplexedInt64()
	if err != nil {
		c.Logger.Printf("warning: read stats (read): %v", err)
	} else {
		c.Logger.Printf("server stats: read=%d", readBytes)
	}
	writtenBytes, err := c.readMultiplexedInt64()
	if err != nil {
		c.Logger.Printf("warning: read stats (written): %v", err)
	} else {
		c.Logger.Printf("server stats: written=%d", writtenBytes)
	}
	totalSize, err := c.readMultiplexedInt64()
	if err != nil {
		c.Logger.Printf("warning: read stats (size): %v", err)
	} else {
		c.Logger.Printf("server stats: size=%d", totalSize)
	}

	// 10. Send final goodbye message (raw -1)
	c.Logger.Printf("sending final goodbye")
	if err := c.wire.WriteInt32(-1); err != nil {
		c.Logger.Printf("warning: write goodbye: %v", err)
	}

	// 11. Handle deletion if enabled
	if c.Delete {
		if err := c.deleteExtraneous(destDir, files); err != nil {
			return fmt.Errorf("delete extraneous: %w", err)
		}
	}

	return nil
}

// exchangeGreeting handles the initial protocol greeting.
// rsync daemon protocol: server sends greeting first, then client responds with module name.
// Format: @RSYNCD: <version>
func (c *Client) exchangeGreeting() error {
	// Read server greeting FIRST (server initiates)
	c.Logger.Printf("waiting for server greeting...")
	line, err := c.wire.ReadLine()
	if err != nil {
		return fmt.Errorf("read greeting: %w", err)
	}
	c.Logger.Printf("received: %s", line)

	// Parse version from "@RSYNCD: <version>"
	if !strings.HasPrefix(line, "@RSYNCD: ") {
		return fmt.Errorf("invalid greeting: %s", line)
	}

	versionStr := strings.TrimPrefix(line, "@RSYNCD: ")
	// Handle subversion format "27.0"
	if idx := strings.Index(versionStr, "."); idx != -1 {
		versionStr = versionStr[:idx]
	}
	// Handle auth options after version
	if idx := strings.Index(versionStr, " "); idx != -1 {
		versionStr = versionStr[:idx]
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return fmt.Errorf("parse version from %q: %w", line, err)
	}

	c.remoteVersion = version
	c.Logger.Printf("server version: %d", c.remoteVersion)

	// Send OUR version back to server
	ourGreeting := fmt.Sprintf("@RSYNCD: %d", ProtocolVersion)
	c.Logger.Printf("sending our version: %s", ourGreeting)
	if err := c.wire.WriteLine(ourGreeting); err != nil {
		return fmt.Errorf("write our version: %w", err)
	}

	return nil
}

// selectModule selects the rsync module.
func (c *Client) selectModule(module string) error {
	// Send module name
	c.Logger.Printf("selecting module: %s", module)
	if err := c.wire.WriteLine(module); err != nil {
		return fmt.Errorf("write module: %w", err)
	}

	// Read response lines until we get @RSYNCD: OK or an error
	for {
		line, err := c.wire.ReadLine()
		if err != nil {
			return fmt.Errorf("read module response: %w", err)
		}

		if line == "@RSYNCD: OK" {
			return nil
		}

		if strings.HasPrefix(line, "@ERROR") {
			return fmt.Errorf("server error: %s", line)
		}

		// MOTD or other informational lines - log them
		if line != "" {
			c.Logger.Printf("server: %s", line)
		}
	}
}

// sendArgs sends the rsync command-line arguments.
// Note: --delete is NOT sent to server - it's handled locally by the client
// after receiving the file list.
func (c *Client) sendArgs(remotePath string) error {
	args := []string{
		"--server",
		"--sender",
	}

	// Note: --delete is a CLIENT-SIDE operation, NOT sent to server
	// Public rsync mirrors (like rsync.gentoo.org) reject --delete

	// Recursive transfer
	args = append(args, "-r")

	// Preserve times
	args = append(args, "-t")

	// Preserve permissions
	args = append(args, "-p")

	// Preserve links
	args = append(args, "-l")

	// Whole file mode - skip block checksums, send entire files
	// This is simpler and faster for initial sync
	args = append(args, "-W")

	// Note: Some servers disable compression (-z), so we don't use it by default
	// The German mirror (halifax) explicitly disables it

	// Add path
	args = append(args, ".")
	if remotePath != "" {
		args = append(args, remotePath)
	}

	// Send each argument as a line
	for _, arg := range args {
		if err := c.wire.WriteLine(arg); err != nil {
			return fmt.Errorf("write arg %q: %w", arg, err)
		}
	}

	// Empty line to end args
	if err := c.wire.WriteLine(""); err != nil {
		return fmt.Errorf("write args terminator: %w", err)
	}

	c.Logger.Printf("sent %d args", len(args))
	return nil
}

// FileEntry is defined in filelist.go

// receiveFileList receives the file list from the server.
func (c *Client) receiveFileList() ([]FileEntry, error) {
	var files []FileEntry
	var lastEntry FileEntry // Track previous entry for SAME_* flags

	for {
		// Read flags byte
		flags, err := c.readMultiplexedByte()
		if err != nil {
			return nil, fmt.Errorf("read flags: %w", err)
		}

		// End of list marker
		if flags == 0 {
			break
		}

		entry, err := c.readFileEntry(flags, lastEntry)
		if err != nil {
			return nil, fmt.Errorf("read file entry: %w", err)
		}

		lastEntry = entry
		files = append(files, entry)

		if len(files)%1000 == 0 {
			c.Logger.Printf("received %d files...", len(files))
		}
	}

	return files, nil
}

// XMIT flags for file list entries (from rsync.h).
const (
	XmitTopDir        = 1 << 0
	XmitSameMode      = 1 << 1
	XmitExtendedFlags = 1 << 2 // Protocol 28+, but we use for proto 27 too
	XmitSameUID       = 1 << 3
	XmitSameGID       = 1 << 4
	XmitSameName      = 1 << 5
	XmitLongName      = 1 << 6
	XmitSameTime      = 1 << 7
)

// readFileEntry reads a single file entry from the wire.
func (c *Client) readFileEntry(flags byte, lastEntry FileEntry) (FileEntry, error) {
	var entry FileEntry

	// Read name
	name, err := c.readFileName(flags, lastEntry.Path)
	if err != nil {
		return entry, fmt.Errorf("read name: %w", err)
	}
	entry.Path = name

	// Read size (always present)
	size, err := c.readMultiplexedInt64()
	if err != nil {
		return entry, fmt.Errorf("read size: %w", err)
	}
	entry.Size = size

	// Read mod time (if not same as previous, inherit from lastEntry otherwise)
	if (flags & XmitSameTime) == 0 {
		mtime, err := c.readMultiplexedInt32()
		if err != nil {
			return entry, fmt.Errorf("read mtime: %w", err)
		}
		entry.ModTime = time.Unix(int64(mtime), 0)
	} else {
		entry.ModTime = lastEntry.ModTime
	}

	// Read mode (if not same as previous, inherit from lastEntry otherwise)
	if (flags & XmitSameMode) == 0 {
		mode, err := c.readMultiplexedInt32()
		if err != nil {
			return entry, fmt.Errorf("read mode: %w", err)
		}
		entry.Mode = os.FileMode(mode)
	} else {
		entry.Mode = lastEntry.Mode
	}

	// Set IsDir based on:
	// 1. XmitTopDir flag (0x01) - rsync sets this for directories
	// 2. Unix mode S_IFDIR (0040000)
	// 3. os.ModeDir bit
	entry.IsDir = (flags&XmitTopDir != 0) ||
		(entry.Mode&os.ModeDir != 0) ||
		((uint32(entry.Mode) & 0o170000) == 0o040000) // S_IFMT mask & S_IFDIR

	// Special case: "." and ".." are always directories regardless of mode
	if name == "." || name == ".." {
		entry.IsDir = true
	}

	// Handle symlinks - check for symlink bit (0120000 in Unix)
	// Use S_IFMT mask (0170000) to extract file type
	isSymlink := entry.Mode&os.ModeSymlink != 0 ||
		((uint32(entry.Mode) & 0o170000) == 0o120000)
	if isSymlink {
		entry.Mode |= os.ModeSymlink
		linkLen, err := c.readMultiplexedInt32()
		if err != nil {
			return entry, fmt.Errorf("read link length: %w", err)
		}
		if linkLen > 0 {
			linkData, err := c.readMultiplexedBytes(int(linkLen))
			if err != nil {
				return entry, fmt.Errorf("read link target: %w", err)
			}
			entry.Link = string(linkData)
		}
	}

	return entry, nil
}

// readFileName reads a file name from the wire.
func (c *Client) readFileName(flags byte, lastName string) (string, error) {
	var name string

	// Determine how much of the name is shared with the previous name
	sameLen := 0
	if (flags & XmitSameName) != 0 {
		l, err := c.readMultiplexedByte()
		if err != nil {
			return "", err
		}
		sameLen = int(l)
	}

	// Read the different part length
	var diffLen int
	if (flags & XmitLongName) != 0 {
		l, err := c.readMultiplexedInt32()
		if err != nil {
			return "", err
		}
		diffLen = int(l)
	} else {
		l, err := c.readMultiplexedByte()
		if err != nil {
			return "", err
		}
		diffLen = int(l)
	}

	// Read the different part
	diffPart, err := c.readMultiplexedBytes(diffLen)
	if err != nil {
		return "", err
	}

	// Combine with shared prefix
	if sameLen > 0 && sameLen <= len(lastName) {
		name = lastName[:sameLen] + string(diffPart)
	} else {
		name = string(diffPart)
	}

	return name, nil
}

// receiveFiles implements rsync generator/receiver protocol.
// Based on gokrazy/rsync and C rsync reference implementation:
// - Generator: sends file indices + sum_head for files needing update
// - Receiver: reads file indices + data from sender
// Both run CONCURRENTLY to prevent buffer overflow (broken pipe).
//
//nolint:gocyclo // Complex by nature - handles rsync wire protocol state machine
func (c *Client) receiveFiles(ctx context.Context, files []FileEntry, destDir string) error {
	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	// Create all directories first (generator handles this locally)
	for _, entry := range files {
		if entry.IsDir {
			destPath := filepath.Join(destDir, entry.Path)
			if err := os.MkdirAll(destPath, entry.Mode|0700); err != nil {
				return fmt.Errorf("create directory %s: %w", destPath, err)
			}
		}
	}

	// Create symlinks (generator handles this locally)
	for _, entry := range files {
		if entry.Mode&os.ModeSymlink != 0 {
			destPath := filepath.Join(destDir, entry.Path)
			_ = os.Remove(destPath)
			if err := os.Symlink(entry.Link, destPath); err != nil {
				c.Logger.Printf("warning: create symlink %s: %v", destPath, err)
			}
		}
	}

	// Collect indices of regular files to request
	var regularFiles []int
	for i, entry := range files {
		if entry.IsDir || entry.Mode&os.ModeSymlink != 0 {
			continue
		}
		if entry.Path == "." || entry.Path == ".." {
			continue
		}
		if len(entry.Path) > 0 && entry.Path[len(entry.Path)-1] == '/' {
			continue
		}
		regularFiles = append(regularFiles, i)
	}
	c.Logger.Printf("need to request %d regular files", len(regularFiles))

	// Run generator and receiver CONCURRENTLY with flow control
	// Use a buffered channel as semaphore to limit outstanding requests.
	// This prevents server buffer overflow that causes connection reset.
	const maxOutstanding = 100 // Max requests in flight
	flowControl := make(chan struct{}, maxOutstanding)
	errChan := make(chan error, 2)

	// Generator goroutine: sends file requests
	go func() {
		errChan <- c.runGenerator(ctx, regularFiles, flowControl)
	}()

	// Receiver goroutine: reads file data
	go func() {
		errChan <- c.runReceiver(ctx, files, destDir, flowControl)
	}()

	// Wait for both to complete
	var firstErr error
	for i := 0; i < 2; i++ {
		if err := <-errChan; err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// runGenerator sends file requests to the server with flow control.
// Uses flowControl channel as semaphore to limit outstanding requests.
// This prevents server buffer overflow that causes connection reset.
func (c *Client) runGenerator(ctx context.Context, regularFiles []int, flowControl chan struct{}) error {
	c.Logger.Printf("=== GENERATOR: sending %d file requests (raw) ===", len(regularFiles))

	// Buffer for request: file_index (4 bytes) + sum_head (16 bytes) = 20 bytes
	// sum_head: count=0, blength=0, s2length=0, remainder=0 (all zeros for full file)
	requestBuf := make([]byte, 20)

	for i, idx := range regularFiles {
		// Flow control: block if too many outstanding requests
		select {
		case <-ctx.Done():
			return ctx.Err()
		case flowControl <- struct{}{}: // Acquire slot
		}

		// Encode file index as little-endian int32
		requestBuf[0] = byte(idx)
		requestBuf[1] = byte(idx >> 8)
		requestBuf[2] = byte(idx >> 16)
		requestBuf[3] = byte(idx >> 24)
		// Bytes 4-19 stay zero (sum_head)

		// Write request as single atomic write
		if _, err := c.wire.Writer.Write(requestBuf); err != nil {
			return fmt.Errorf("write request for %d: %w", idx, err)
		}

		// Log progress
		if (i+1)%10000 == 0 {
			c.Logger.Printf("generator: sent %d/%d requests", i+1, len(regularFiles))
		}
	}

	// Send phase 1 done (-1)
	c.Logger.Printf("generator: sending NDX_DONE (phase 1)")
	if err := c.wire.WriteInt32(-1); err != nil {
		return fmt.Errorf("write NDX_DONE phase 1: %w", err)
	}

	// Send phase 2 done (-1) - for redo phase
	c.Logger.Printf("generator: sending NDX_DONE (phase 2)")
	if err := c.wire.WriteInt32(-1); err != nil {
		return fmt.Errorf("write NDX_DONE phase 2: %w", err)
	}

	c.Logger.Printf("generator: complete")
	return nil
}

// runReceiver reads file data from the server with flow control.
// Releases slots in flowControl channel to allow generator to send more requests.
func (c *Client) runReceiver(ctx context.Context, files []FileEntry, destDir string, flowControl chan struct{}) error {
	c.Logger.Printf("=== RECEIVER: waiting for files ===")
	filesReceived := 0
	phase := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read file index from sender (through multiplexer)
		recvIdx, err := c.readMultiplexedInt32()
		if err != nil {
			return fmt.Errorf("read file index: %w", err)
		}

		if recvIdx == -1 {
			// NDX_DONE - end of phase
			phase++
			c.Logger.Printf("receiver: got NDX_DONE, phase=%d", phase)
			if phase >= 2 {
				break
			}
			continue
		}

		if recvIdx < 0 || int(recvIdx) >= len(files) {
			return fmt.Errorf("invalid file index: %d (max %d)", recvIdx, len(files))
		}

		entry := files[recvIdx]
		destPath := filepath.Join(destDir, entry.Path)

		// Receive file data
		if err := c.receiveFile(entry, destPath); err != nil {
			return fmt.Errorf("receive file %s: %w", entry.Path, err)
		}

		// Release flow control slot to allow generator to send more
		select {
		case <-flowControl: // Release slot
		default:
			// Channel empty - shouldn't happen but don't block
		}

		filesReceived++
		if filesReceived%1000 == 0 {
			c.Logger.Printf("received %d files...", filesReceived)
		}
	}

	c.Logger.Printf("receiver: complete, received %d files", filesReceived)
	return nil
}

// receiveFile receives a single file's data.
// Protocol flow for protocol 27:
//  1. We send: file_index + sum_head (count=0, blength=0, s2length=0, remainder=0)
//  2. Sender responds: file_index + sum_head + tokens
//
// See sender.c:send_files() - write_ndx_and_attrs() then write_sum_head()
func (c *Client) receiveFile(entry FileEntry, destPath string) error {
	// Read sum_head echoed back by sender (4 int32s for protocol >= 27)
	// See io.c:write_sum_head() - count, blength, s2length, remainder
	sumCount, err := c.readMultiplexedInt32()
	if err != nil {
		return fmt.Errorf("read sum_head count: %w", err)
	}
	sumBlength, err := c.readMultiplexedInt32()
	if err != nil {
		return fmt.Errorf("read sum_head blength: %w", err)
	}
	sumS2length, err := c.readMultiplexedInt32()
	if err != nil {
		return fmt.Errorf("read sum_head s2length: %w", err)
	}
	sumRemainder, err := c.readMultiplexedInt32()
	if err != nil {
		return fmt.Errorf("read sum_head remainder: %w", err)
	}
	c.Logger.Printf("sum_head: count=%d, blength=%d, s2length=%d, remainder=%d",
		sumCount, sumBlength, sumS2length, sumRemainder)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// Create temporary file
	tmpPath := destPath + ".rsync-tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(tmpPath)
	}()

	// Receive file data
	if c.Compress {
		err = c.receiveCompressedData(f)
	} else {
		err = c.receiveUncompressedData(f)
	}

	if err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Set file mode and times
	if err := os.Chmod(tmpPath, entry.Mode); err != nil {
		c.Logger.Printf("warning: chmod %s: %v", destPath, err)
	}
	if err := os.Chtimes(tmpPath, entry.ModTime, entry.ModTime); err != nil {
		c.Logger.Printf("warning: chtimes %s: %v", destPath, err)
	}

	// Rename to final destination
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("rename to destination: %w", err)
	}

	return nil
}

// receiveUncompressedData receives uncompressed file data using simple token protocol.
// rsync token format for whole-file mode:
// 1. Match token (-1 for whole file = no matching block)
// 2. Data length (positive int32)
// 3. Data bytes
// 4. End marker (0)
// 5. Checksum (16 bytes for MD4/MD5)
func (c *Client) receiveUncompressedData(w io.Writer) error {
	totalBytes := 0

	for {
		// Read token
		token, err := c.readMultiplexedInt32()
		if err != nil {
			return fmt.Errorf("read token: %w", err)
		}

		c.Logger.Printf("token: %d (0x%x)", token, uint32(token))

		// Token == 0 means end of file
		if token == 0 {
			c.Logger.Printf("end of file marker, total bytes: %d", totalBytes)
			break
		}

		// Token < 0 means literal data follows
		// For whole-file mode: token = -(len + 1), so len = -(token + 1)
		// But for simple protocol: positive token = data length
		var dataLen int
		if token < 0 {
			// Negative token: this is a match token or literal data indicator
			// In whole-file mode with -W, we might get -1 followed by data length
			dataLen = -int(token + 1)
			if dataLen <= 0 {
				// -1 means "no match, whole file follows" - read actual length next
				dataLen32, err := c.readMultiplexedInt32()
				if err != nil {
					return fmt.Errorf("read data length after -1: %w", err)
				}
				dataLen = int(dataLen32)
			}
		} else {
			// Positive token = data length
			dataLen = int(token)
		}

		c.Logger.Printf("reading %d bytes of data", dataLen)

		if dataLen > 0 && dataLen <= 10*1024*1024 {
			data, err := c.readMultiplexedBytes(dataLen)
			if err != nil {
				return fmt.Errorf("read data (%d bytes): %w", dataLen, err)
			}

			if _, err := w.Write(data); err != nil {
				return err
			}
			totalBytes += dataLen
		}
	}

	// Read file checksum (16 bytes)
	_, checksumErr := c.readMultiplexedBytes(16)
	if checksumErr != nil {
		c.Logger.Printf("warning: failed to read checksum: %v", checksumErr)
	}

	return nil
}

// receiveCompressedData receives compressed file data using zlib.
func (c *Client) receiveCompressedData(w io.Writer) error {
	decomp := NewDecompressor(c.mplex)

	for {
		token, data, err := decomp.RecvToken()
		if err != nil {
			return fmt.Errorf("recv token: %w", err)
		}

		// Token 0 means end of file
		if token == 0 {
			break
		}

		// Negative token means data
		if token < 0 && data != nil {
			if _, err := w.Write(data); err != nil {
				return err
			}
		}
	}

	// Read file checksum
	_, _ = c.readMultiplexedBytes(16)

	return nil
}

// deleteExtraneous removes files that exist locally but not in the remote file list.
func (c *Client) deleteExtraneous(destDir string, files []FileEntry) error {
	// Build set of expected paths
	expected := make(map[string]bool)
	for _, f := range files {
		expected[f.Path] = true
	}

	// Walk local directory
	return filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Ignore errors
		}

		rel, err := filepath.Rel(destDir, path)
		if err != nil || rel == "." {
			return nil
		}

		// Convert to forward slashes for comparison
		rel = filepath.ToSlash(rel)

		if !expected[rel] {
			c.Logger.Printf("deleting extraneous: %s", rel)
			if info.IsDir() {
				return os.RemoveAll(path)
			}
			return os.Remove(path)
		}

		return nil
	})
}

// readMultiplexedByte reads a byte through the multiplexer.
func (c *Client) readMultiplexedByte() (byte, error) {
	var buf [1]byte
	if _, err := io.ReadFull(c.mplex, buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}

// readMultiplexedInt32 reads a 32-bit integer through the multiplexer.
func (c *Client) readMultiplexedInt32() (int32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(c.mplex, buf[:]); err != nil {
		return 0, err
	}
	return int32(buf[0]) | int32(buf[1])<<8 | int32(buf[2])<<16 | int32(buf[3])<<24, nil
}

// readMultiplexedInt64 reads a 64-bit integer through the multiplexer.
// Uses rsync's variable encoding: int32, unless -1 then read full int64.
func (c *Client) readMultiplexedInt64() (int64, error) {
	v, err := c.readMultiplexedInt32()
	if err != nil {
		return 0, err
	}
	if v != -1 {
		return int64(v), nil
	}
	var buf [8]byte
	if _, err := io.ReadFull(c.mplex, buf[:]); err != nil {
		return 0, err
	}
	return int64(buf[0]) | int64(buf[1])<<8 | int64(buf[2])<<16 | int64(buf[3])<<24 |
		int64(buf[4])<<32 | int64(buf[5])<<40 | int64(buf[6])<<48 | int64(buf[7])<<56, nil
}

// readMultiplexedBytes reads exactly n bytes through the multiplexer.
func (c *Client) readMultiplexedBytes(n int) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(c.mplex, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
