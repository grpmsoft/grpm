package rsync

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Protocol constants
const (
	ProtocolVersion = 27
	ChunkSize       = 32 * 1024
	MaxDataCount    = 16383 // 14-bit max for compressed chunks
)

// Multiplexing tags (rsync protocol)
const (
	MsgData    uint8 = 0
	MsgError   uint8 = 1
	MsgInfo    uint8 = 2
	MsgDeleted uint8 = 3
	MsgSuccess uint8 = 4
	MsgNoSend  uint8 = 5
)

const mplexBase = 7

// Compression flag bytes
const (
	EndFlag      = 0x00 // End of file
	TokenLong    = 0x20 // 32-bit token number follows
	TokenRunLong = 0x21 // 32-bit token + 16-bit run count
	DeflatedData = 0x40 // Compressed data (+ 6-bit high len)
	TokenRel     = 0x80 // Relative token (+ 6-bit offset)
	TokenRunRel  = 0xC0 // Relative token + run count
)

// Conn wraps an io.ReadWriter with rsync wire protocol methods.
type Conn struct {
	Reader io.Reader
	Writer io.Writer
}

// NewConn creates a new protocol connection.
func NewConn(r io.Reader, w io.Writer) *Conn {
	return &Conn{Reader: r, Writer: w}
}

// WriteByte writes a single byte.
func (c *Conn) WriteByte(b byte) error {
	_, err := c.Writer.Write([]byte{b})
	return err
}

// WriteInt32 writes a 32-bit little-endian integer.
func (c *Conn) WriteInt32(v int32) error {
	return binary.Write(c.Writer, binary.LittleEndian, v)
}

// WriteInt64 writes a 64-bit integer (using rsync's variable encoding).
func (c *Conn) WriteInt64(v int64) error {
	if v >= 0 && v <= 0x7FFFFFFF {
		return c.WriteInt32(int32(v))
	}
	if err := c.WriteInt32(-1); err != nil {
		return err
	}
	return binary.Write(c.Writer, binary.LittleEndian, v)
}

// WriteString writes a string without null terminator.
func (c *Conn) WriteString(s string) error {
	_, err := io.WriteString(c.Writer, s)
	return err
}

// WriteLine writes a string with newline.
func (c *Conn) WriteLine(s string) error {
	if err := c.WriteString(s); err != nil {
		return err
	}
	return c.WriteByte('\n')
}

// ReadByte reads a single byte.
func (c *Conn) ReadByte() (byte, error) {
	var buf [1]byte
	if _, err := io.ReadFull(c.Reader, buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}

// ReadInt32 reads a 32-bit little-endian integer.
func (c *Conn) ReadInt32() (int32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(c.Reader, buf[:]); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(buf[:])), nil
}

// ReadInt64 reads a 64-bit integer (rsync variable encoding).
func (c *Conn) ReadInt64() (int64, error) {
	v, err := c.ReadInt32()
	if err != nil {
		return 0, err
	}
	if v != -1 {
		return int64(v), nil
	}
	var v64 int64
	if err := binary.Read(c.Reader, binary.LittleEndian, &v64); err != nil {
		return 0, err
	}
	return v64, nil
}

// ReadBytes reads exactly n bytes.
func (c *Conn) ReadBytes(n int) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(c.Reader, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// ReadLine reads a newline-terminated string.
func (c *Conn) ReadLine() (string, error) {
	var buf []byte
	for {
		b, err := c.ReadByte()
		if err != nil {
			return "", err
		}
		if b == '\n' {
			return string(buf), nil
		}
		if b == '\r' {
			continue // Skip CR
		}
		buf = append(buf, b)
		if len(buf) > 4096 {
			return "", fmt.Errorf("line too long")
		}
	}
}

// MultiplexReader reads multiplexed rsync messages.
type MultiplexReader struct {
	conn    *Conn
	pending []byte
	debug   Logger
}

// NewMultiplexReader creates a multiplexed reader.
func NewMultiplexReader(c *Conn) *MultiplexReader {
	return &MultiplexReader{conn: c}
}

// SetDebug enables debug logging.
func (m *MultiplexReader) SetDebug(l Logger) {
	m.debug = l
}

// ReadMsg reads the next multiplexed message.
func (m *MultiplexReader) ReadMsg() (tag uint8, data []byte, err error) {
	// Read raw 4 bytes for debugging
	var headerBuf [4]byte
	if _, err := io.ReadFull(m.conn.Reader, headerBuf[:]); err != nil {
		return 0, nil, fmt.Errorf("read header: %w", err)
	}

	header := int32(headerBuf[0]) | int32(headerBuf[1])<<8 | int32(headerBuf[2])<<16 | int32(headerBuf[3])<<24

	rawTag := uint8(uint32(header) >> 24)
	length := uint32(header) & 0x00FFFFFF

	// Debug: print raw header bytes
	if m.debug != nil {
		m.debug.Printf("mplex header: %02x %02x %02x %02x (tag=%d, len=%d)",
			headerBuf[0], headerBuf[1], headerBuf[2], headerBuf[3], rawTag, length)
	}

	// Validate tag
	if rawTag < mplexBase {
		return 0, nil, fmt.Errorf("invalid mplex tag: %d (expected >=%d), raw bytes: %02x %02x %02x %02x",
			rawTag, mplexBase, headerBuf[0], headerBuf[1], headerBuf[2], headerBuf[3])
	}

	tag = rawTag - mplexBase

	if length > 256*1024 {
		return 0, nil, fmt.Errorf("message too large: %d bytes (tag=%d)", length, rawTag)
	}

	data, err = m.conn.ReadBytes(int(length))
	if err != nil {
		return 0, nil, err
	}

	return tag, data, nil
}

// Read implements io.Reader, extracting data from multiplexed stream.
func (m *MultiplexReader) Read(p []byte) (n int, err error) {
	if len(m.pending) > 0 {
		n = copy(p, m.pending)
		m.pending = m.pending[n:]
		return n, nil
	}

	tag, data, err := m.ReadMsg()
	if err != nil {
		return 0, err
	}

	switch tag {
	case MsgData:
		n = copy(p, data)
		if n < len(data) {
			m.pending = data[n:]
		}
		return n, nil
	case MsgError:
		// Server-side errors are logged as warnings, not fatal
		// These can be I/O errors on the server side, permission issues, etc.
		if m.debug != nil {
			m.debug.Printf("server error (non-fatal): %s", string(data))
		}
		// Continue reading - don't abort the sync for server-side errors
		return m.Read(p)
	case MsgInfo:
		// Log info and continue reading
		if m.debug != nil {
			m.debug.Printf("server info: %s", string(data))
		}
		return m.Read(p)
	default:
		return 0, fmt.Errorf("unexpected message tag: %d", tag)
	}
}
