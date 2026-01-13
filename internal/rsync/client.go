package rsync

import (
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

	// Set read/write deadline based on context
	// This ensures operations don't hang indefinitely
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			c.Logger.Printf("warning: failed to set connection deadline: %v", err)
		}
	}

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

	// 4. Send exclusion list (empty - no filters)
	// Server calls recv_filter_list() and waits for 0 to indicate no filters
	c.Logger.Printf("sending empty filter list")
	if err := c.wire.WriteInt32(0); err != nil {
		return fmt.Errorf("write filter terminator: %w", err)
	}

	// 5. Read checksum seed (raw int32, not multiplexed for protocol < 30)
	// See rsync's setup_protocol() - seed is sent before multiplex is enabled
	seed, err := c.wire.ReadInt32()
	if err != nil {
		return fmt.Errorf("read checksum seed: %w", err)
	}
	c.Logger.Printf("checksum seed: 0x%08x", uint32(seed))

	// 6. Enable multiplexing
	c.Logger.Printf("enabling multiplexing, protocol version: %d", c.remoteVersion)
	c.mplex = NewMultiplexReader(c.wire)
	c.mplex.SetDebug(c.Logger)

	// 6. Receive file list
	files, err := c.receiveFileList()
	if err != nil {
		return fmt.Errorf("receive file list: %w", err)
	}
	c.Logger.Printf("received %d files", len(files))

	// 6.5. Read I/O error count (sent after file list)
	// For protocol < 30: 1 byte, for >= 30: int32
	// HOWEVER: Some fields might be sent even for proto 27 if server is >= 30
	// Let's read as int32 to be safe since we saw 4 extra bytes
	ioErrCount, err := c.readMultiplexedInt32()
	if err != nil {
		return fmt.Errorf("read io error count: %w", err)
	}
	c.Logger.Printf("I/O error count: %d", ioErrCount)

	// 7. Request and receive files
	if err := c.receiveFiles(ctx, files, destDir); err != nil {
		return fmt.Errorf("receive files: %w", err)
	}

	// 8. Handle deletion if enabled
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

	// Set IsDir based on mode - check for directory bit in Unix mode
	// rsync uses Unix permission bits where 0040000 (S_IFDIR) indicates directory
	entry.IsDir = entry.Mode&os.ModeDir != 0 || (uint32(entry.Mode)&0o40000) != 0

	// Handle symlinks - check for symlink bit (0120000 in Unix)
	isSymlink := entry.Mode&os.ModeSymlink != 0 || (uint32(entry.Mode)&0o120000) == 0o120000
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

// receiveFiles requests and receives file data from the server.
// rsync protocol: client (generator) sends file indices + empty checksum header,
// then server (sender) sends file data for each requested file.
// See sender.c:send_files() - sender WAITS for read_ndx() from client.
//
//nolint:gocyclo // Complex by nature - handles rsync wire protocol state machine
func (c *Client) receiveFiles(ctx context.Context, files []FileEntry, destDir string) error {
	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	// Create all directories first
	for _, entry := range files {
		if entry.IsDir {
			destPath := filepath.Join(destDir, entry.Path)
			if err := os.MkdirAll(destPath, entry.Mode|0700); err != nil {
				return fmt.Errorf("create directory %s: %w", destPath, err)
			}
		}
	}

	// Create symlinks
	for _, entry := range files {
		if entry.Mode&os.ModeSymlink != 0 {
			destPath := filepath.Join(destDir, entry.Path)
			_ = os.Remove(destPath)
			if err := os.Symlink(entry.Link, destPath); err != nil {
				c.Logger.Printf("warning: create symlink %s: %v", destPath, err)
			}
		}
	}

	// rsync generator/sender protocol:
	// Generator sends file index + sum_head for files needing update
	// Sender receives request, sends file data, then echoes file index
	// Flow is pipelined - we send requests and receive responses concurrently
	//
	// For simplicity, we'll batch requests in smaller groups to avoid
	// overwhelming the server's request queue

	// Collect indices of regular files to request
	var regularFiles []int
	for i, entry := range files {
		if !entry.IsDir && entry.Mode&os.ModeSymlink == 0 {
			regularFiles = append(regularFiles, i)
		}
	}
	c.Logger.Printf("need to request %d regular files (dirs/symlinks: %d)", len(regularFiles), len(files)-len(regularFiles))

	// Debug: print first 5 regular files and first 5 non-regular files
	for i := 0; i < 5 && i < len(regularFiles); i++ {
		f := files[regularFiles[i]]
		c.Logger.Printf("regular[%d] idx=%d: %s (mode=0x%x, isDir=%v)", i, regularFiles[i], f.Path, uint32(f.Mode), f.IsDir)
	}
	for i, f := range files[:min(10, len(files))] {
		c.Logger.Printf("file[%d]: %s (mode=0x%x, isDir=%v, isSymlink=%v)", i, f.Path, uint32(f.Mode), f.IsDir, f.Mode&os.ModeSymlink != 0)
	}

	// Request and receive files one at a time
	// rsync protocol uses pipelining, but for simplicity we'll do sequential
	filesReceived := 0

	for _, idx := range regularFiles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Send file request - write 20 bytes total:
		// 4 bytes: file index
		// 16 bytes: sum_head (count=0, blength=0, s2length=0, remainder=0)
		c.Logger.Printf("requesting file %d: %s", idx, files[idx].Path)

		// Build the request bytes to log them
		var reqBuf [20]byte
		reqBuf[0] = byte(idx)
		reqBuf[1] = byte(idx >> 8)
		reqBuf[2] = byte(idx >> 16)
		reqBuf[3] = byte(idx >> 24)
		// Bytes 4-19 are zeros (sum_head with all zeros)
		c.Logger.Printf("sending request bytes: %x", reqBuf)

		// Write file index
		if err := c.wire.WriteInt32(int32(idx)); err != nil {
			return fmt.Errorf("write file index %d: %w", idx, err)
		}

		// Send empty sum header for fresh sync
		for i := 0; i < 4; i++ {
			if err := c.wire.WriteInt32(0); err != nil {
				return fmt.Errorf("write sum header[%d]: %w", i, err)
			}
		}
		c.Logger.Printf("request sent, waiting for response...")

		// Read file index through multiplexer (it handles the message framing)
		recvIdx, err := c.readMultiplexedInt32()
		if err != nil {
			return fmt.Errorf("read file index: %w", err)
		}
		c.Logger.Printf("received file index: %d", recvIdx)

		if recvIdx != int32(idx) {
			c.Logger.Printf("warning: requested idx=%d but received idx=%d", idx, recvIdx)
		}

		if recvIdx < 0 || int(recvIdx) >= len(files) {
			if recvIdx == -1 {
				c.Logger.Printf("received NDX_DONE, stopping")
				break
			}
			return fmt.Errorf("invalid file index: %d (max %d)", recvIdx, len(files))
		}

		entry := files[recvIdx]
		destPath := filepath.Join(destDir, entry.Path)

		// Receive file data
		if err := c.receiveFile(entry, destPath); err != nil {
			return fmt.Errorf("receive file %s: %w", entry.Path, err)
		}

		// Send acknowledgment - receiver sends file index back after successful receipt
		// See receiver.c:recv_files() - write_ndx(f_out, ndx)
		if err := c.wire.WriteInt32(recvIdx); err != nil {
			return fmt.Errorf("write ack for file %d: %w", recvIdx, err)
		}

		filesReceived++
		if filesReceived%1000 == 0 {
			c.Logger.Printf("received %d files...", filesReceived)
		}
	}

	// Send NDX_DONE (-1) to indicate end of file requests
	c.Logger.Printf("sent all file requests (%d files), sending NDX_DONE", filesReceived)
	if err := c.wire.WriteInt32(-1); err != nil {
		return fmt.Errorf("write NDX_DONE: %w", err)
	}

	// Wait for final NDX_DONE from sender
	finalIdx, err := c.readMultiplexedInt32()
	if err != nil {
		return fmt.Errorf("read final NDX_DONE: %w", err)
	}
	if finalIdx != -1 {
		c.Logger.Printf("warning: expected final NDX_DONE, got %d", finalIdx)
	}

	c.Logger.Printf("sync complete: received %d files", filesReceived)

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
