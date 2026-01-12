package rsync

import (
	"bytes"
	"testing"
)

func TestConnWriteReadInt32(t *testing.T) {
	var buf bytes.Buffer
	conn := NewConn(&buf, &buf)

	// Test positive value
	if err := conn.WriteInt32(12345); err != nil {
		t.Fatalf("WriteInt32: %v", err)
	}

	// Test negative value
	if err := conn.WriteInt32(-98765); err != nil {
		t.Fatalf("WriteInt32: %v", err)
	}

	// Read back
	v1, err := conn.ReadInt32()
	if err != nil {
		t.Fatalf("ReadInt32: %v", err)
	}
	if v1 != 12345 {
		t.Errorf("expected 12345, got %d", v1)
	}

	v2, err := conn.ReadInt32()
	if err != nil {
		t.Fatalf("ReadInt32: %v", err)
	}
	if v2 != -98765 {
		t.Errorf("expected -98765, got %d", v2)
	}
}

func TestConnWriteReadInt64(t *testing.T) {
	var buf bytes.Buffer
	conn := NewConn(&buf, &buf)

	testCases := []int64{
		0,
		1,
		0x7FFFFFFF,         // Max 32-bit positive
		0x80000000,         // Needs 64-bit encoding
		0x123456789ABCDEF0, // Large 64-bit value
		-1,                 // Triggers 64-bit encoding
	}

	for _, tc := range testCases {
		buf.Reset()
		if err := conn.WriteInt64(tc); err != nil {
			t.Fatalf("WriteInt64(%d): %v", tc, err)
		}

		v, err := conn.ReadInt64()
		if err != nil {
			t.Fatalf("ReadInt64: %v", err)
		}
		if v != tc {
			t.Errorf("expected %d, got %d", tc, v)
		}
	}
}

func TestConnWriteReadLine(t *testing.T) {
	var buf bytes.Buffer
	conn := NewConn(&buf, &buf)

	testLines := []string{
		"@RSYNCD: 27",
		"hello world",
		"",
		"line with spaces and special chars!@#$%",
	}

	for _, line := range testLines {
		if err := conn.WriteLine(line); err != nil {
			t.Fatalf("WriteLine(%q): %v", line, err)
		}
	}

	for _, expected := range testLines {
		got, err := conn.ReadLine()
		if err != nil {
			t.Fatalf("ReadLine: %v", err)
		}
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	}
}

func TestConnReadLineSkipsCR(t *testing.T) {
	// Test that \r is stripped (Windows line endings)
	buf := bytes.NewBufferString("hello\r\nworld\r\n")
	conn := NewConn(buf, nil)

	line1, err := conn.ReadLine()
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if line1 != "hello" {
		t.Errorf("expected 'hello', got %q", line1)
	}

	line2, err := conn.ReadLine()
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if line2 != "world" {
		t.Errorf("expected 'world', got %q", line2)
	}
}

func TestMultiplexReaderDataMessage(t *testing.T) {
	var buf bytes.Buffer
	conn := NewConn(&buf, &buf)

	// Create a multiplexed data message
	// Format: [tag+length:4 bytes][data]
	// Tag = MsgData (0) + mplexBase (7) = 7, in high byte
	// Length = data length in lower 3 bytes
	data := []byte("test data message")
	header := int32((7 << 24) | len(data))

	if err := conn.WriteInt32(header); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := buf.Write(data); err != nil {
		t.Fatalf("write data: %v", err)
	}

	// Read through multiplexer
	mplex := NewMultiplexReader(NewConn(&buf, nil))
	readBuf := make([]byte, 100)
	n, err := mplex.Read(readBuf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if string(readBuf[:n]) != string(data) {
		t.Errorf("expected %q, got %q", data, readBuf[:n])
	}
}

func TestProtocolConstants(t *testing.T) {
	// Verify protocol constants match rsync implementation
	if ProtocolVersion != 27 {
		t.Errorf("expected ProtocolVersion 27, got %d", ProtocolVersion)
	}
	if ChunkSize != 32*1024 {
		t.Errorf("expected ChunkSize 32KB, got %d", ChunkSize)
	}
	if MaxDataCount != 16383 {
		t.Errorf("expected MaxDataCount 16383, got %d", MaxDataCount)
	}

	// Flag constants
	if EndFlag != 0x00 {
		t.Errorf("expected EndFlag 0x00, got %#x", EndFlag)
	}
	if DeflatedData != 0x40 {
		t.Errorf("expected DeflatedData 0x40, got %#x", DeflatedData)
	}
	if TokenRel != 0x80 {
		t.Errorf("expected TokenRel 0x80, got %#x", TokenRel)
	}
}
