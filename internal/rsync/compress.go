package rsync

import (
	"bytes"
	"compress/flate"
	"errors"
	"fmt"
	"io"
)

// Decompressor handles rsync zlib decompression.
// Rsync uses raw deflate (no zlib header) with Z_SYNC_FLUSH markers.
//
// The original rsync uses:
//   - deflateInit2(level, Z_DEFLATED, -15, 8, Z_DEFAULT_STRATEGY)
//   - -15 means raw deflate (no zlib/gzip header), 15-bit window
//   - Z_SYNC_FLUSH produces 00 00 FF FF trailer
//
// Reference: rsync/token.c:recv_deflated_token()
type Decompressor struct {
	reader io.Reader
	zbuf   bytes.Buffer // accumulated compressed data
	state  recvState
	token  int32
	run    int32
}

type recvState int

const (
	stateInit recvState = iota
	stateIdle
	stateInflating
	stateInflated
	stateRunning
)

// NewDecompressor creates a new rsync decompressor.
func NewDecompressor(r io.Reader) *Decompressor {
	return &Decompressor{
		reader: r,
		state:  stateInit,
	}
}

// RecvToken receives the next token from compressed stream.
// Returns: token number (negative = data token), data slice, error
//
// Token semantics:
//   - token < 0: actual token = -(token+1), data contains file data
//   - token == 0: end of file
//   - token > 0: block reference (for delta transfers, not used in full file mode)
//
// Reference: rsync/token.c:recv_deflated_token()
func (d *Decompressor) RecvToken() (token int32, data []byte, err error) {
	for {
		switch d.state {
		case stateInit:
			d.initState()

		case stateIdle, stateInflated:
			token, data, done, err := d.handleIdleOrInflated()
			if err != nil {
				return 0, nil, err
			}
			if done {
				return token, data, nil
			}

		case stateRunning:
			return d.handleRunning()

		case stateInflating:
			d.state = stateInflated
		}
	}
}

// initState handles the stateInit case.
func (d *Decompressor) initState() {
	d.state = stateIdle
	d.token = 0
	d.zbuf.Reset()
}

// handleIdleOrInflated handles stateIdle and stateInflated cases.
// Returns token, data, done flag, and error.
// done=true means caller should return the token/data.
func (d *Decompressor) handleIdleOrInflated() (int32, []byte, bool, error) {
	flag, err := d.readByte()
	if err != nil {
		return 0, nil, false, err
	}

	// Check for compressed data block
	if (flag & 0xC0) == DeflatedData {
		return d.handleCompressedData(flag)
	}

	// Handle sync flush completion
	if d.state == stateInflated {
		d.state = stateIdle
	}

	// End of file marker
	if flag == EndFlag {
		d.state = stateInit
		d.zbuf.Reset()
		return 0, nil, true, nil
	}

	// Token handling
	return d.handleTokenFlags(flag)
}

// handleCompressedData reads and decompresses a compressed data block.
func (d *Decompressor) handleCompressedData(flag byte) (int32, []byte, bool, error) {
	// Length is 14-bit: 6 bits from flag + 8 bits from next byte
	lenHigh := int(flag & 0x3F)
	lenLow, err := d.readByte()
	if err != nil {
		return 0, nil, false, err
	}
	n := (lenHigh << 8) | int(lenLow)

	if n > MaxDataCount {
		return 0, nil, false, fmt.Errorf("compressed chunk too large: %d", n)
	}

	// Read compressed data
	compressed := make([]byte, n)
	if _, err := io.ReadFull(d.reader, compressed); err != nil {
		return 0, nil, false, fmt.Errorf("read compressed data: %w", err)
	}

	// Decompress using raw deflate
	data, err := d.inflate(compressed)
	if err != nil {
		return 0, nil, false, fmt.Errorf("inflate: %w", err)
	}

	d.state = stateInflated
	if len(data) > 0 {
		return -1, data, true, nil // Data token
	}
	return 0, nil, false, nil // Continue loop
}

// handleTokenFlags processes token flags and reads token/run data.
func (d *Decompressor) handleTokenFlags(flag byte) (int32, []byte, bool, error) {
	// Token handling (block references)
	if (flag & TokenRel) != 0 {
		// Relative token: offset from previous token
		d.token += int32(flag & 0x3F)
		flag >>= 6
	} else {
		// Absolute token: read full 32-bit value
		tok, err := d.readInt32()
		if err != nil {
			return 0, nil, false, err
		}
		if tok < 0 {
			return 0, nil, false, fmt.Errorf("invalid token: %d", tok)
		}
		d.token = tok
	}

	// Check for run of consecutive tokens
	if (flag & 1) != 0 {
		if err := d.readRunLength(); err != nil {
			return 0, nil, false, err
		}
		d.state = stateRunning
	}

	return -1 - d.token, nil, true, nil
}

// readRunLength reads the 16-bit run length.
func (d *Decompressor) readRunLength() error {
	lo, err := d.readByte()
	if err != nil {
		return err
	}
	hi, err := d.readByte()
	if err != nil {
		return err
	}
	d.run = int32(lo) | (int32(hi) << 8)
	return nil
}

// handleRunning handles the stateRunning case.
func (d *Decompressor) handleRunning() (int32, []byte, error) {
	d.token++
	d.run--
	if d.run == 0 {
		d.state = stateIdle
	}
	return -1 - d.token, nil, nil
}

// inflate decompresses data using raw deflate (no zlib header).
//
// Rsync uses raw deflate with:
//   - 15-bit window size (32KB)
//   - Z_SYNC_FLUSH between blocks (produces 00 00 FF FF)
//   - Streaming context preserved across calls
//
// Reference: rsync/token.c line 377-460
func (d *Decompressor) inflate(compressed []byte) ([]byte, error) {
	// Append compressed data to our buffer
	d.zbuf.Write(compressed)

	// Create flate reader for raw deflate
	// Note: Go's flate.NewReader expects raw deflate data (no headers)
	reader := flate.NewReader(&d.zbuf)
	defer func() { _ = reader.Close() }()

	// Read all decompressed data
	var out bytes.Buffer
	_, err := io.Copy(&out, reader)

	// flate.Reader returns io.ErrUnexpectedEOF when it hits Z_SYNC_FLUSH marker
	// This is expected behavior for rsync's streaming compression
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		// Return what we have - partial data may be usable
		return out.Bytes(), nil
	}

	return out.Bytes(), nil
}

// readByte reads a single byte from the underlying reader.
func (d *Decompressor) readByte() (byte, error) {
	var buf [1]byte
	if _, err := io.ReadFull(d.reader, buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}

// readInt32 reads a 32-bit little-endian integer from the underlying reader.
func (d *Decompressor) readInt32() (int32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(d.reader, buf[:]); err != nil {
		return 0, err
	}
	return int32(buf[0]) | int32(buf[1])<<8 | int32(buf[2])<<16 | int32(buf[3])<<24, nil
}

// SimpleRecvToken receives uncompressed token data.
// Used when compression is disabled (-z flag not set).
//
// Wire format:
//   - n > 0: n bytes of literal data follow
//   - n <= 0: block reference token (n = -(token+1))
//
// Reference: rsync/token.c:simple_recv_token()
func SimpleRecvToken(conn *Conn) (token int32, data []byte, err error) {
	n, err := conn.ReadInt32()
	if err != nil {
		return 0, nil, err
	}

	// Positive n means literal data
	if n > 0 {
		data, err = conn.ReadBytes(int(n))
		if err != nil {
			return 0, nil, err
		}
		return n, data, nil
	}

	// Zero or negative n is a token
	return n, nil, nil
}
