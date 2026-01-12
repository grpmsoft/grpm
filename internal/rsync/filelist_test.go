package rsync

import (
	"bytes"
	"os"
	"testing"
)

func TestFromWireMode(t *testing.T) {
	testCases := []struct {
		wire     uint32
		expected os.FileMode
		desc     string
	}{
		{0x81A4, 0644, "regular file 644"},
		{0x81FF, 0777, "regular file 777"},
		{0x41ED, os.ModeDir | 0755, "directory 755"},
		{0xA1FF, os.ModeSymlink | 0777, "symlink"},
		{0x61B0, os.ModeDevice | 0660, "block device"},
		{0x21B6, os.ModeDevice | os.ModeCharDevice | 0666, "char device"},
		{0xC1FF, os.ModeSocket | 0777, "socket"},
		{0x11B6, os.ModeNamedPipe | 0666, "named pipe"},
	}

	for _, tc := range testCases {
		got := fromWireMode(tc.wire)
		if got != tc.expected {
			t.Errorf("%s: fromWireMode(%#x) = %v, expected %v", tc.desc, tc.wire, got, tc.expected)
		}
	}
}

func TestFileListReaderReadFileName(t *testing.T) {
	// Test file name compression reading
	testCases := []struct {
		name     string
		flags    uint16
		lastName string
		input    []byte
		expected string
	}{
		{
			name:     "simple name",
			flags:    0,
			lastName: "",
			input:    []byte{4, 't', 'e', 's', 't'},
			expected: "test",
		},
		{
			name:     "same prefix",
			flags:    XmitSameName,
			lastName: "dir/file1.txt",
			input:    []byte{4, 9, 'f', 'i', 'l', 'e', '2', '.', 't', 'x', 't'},
			expected: "dir/file2.txt",
		},
		{
			name:     "long name",
			flags:    XmitLongName,
			lastName: "",
			// Little-endian int32: 10 = 0x0A, 0x00, 0x00, 0x00
			input:    append([]byte{10, 0, 0, 0}, []byte("longername")...),
			expected: "longername",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := NewFileListReader(bytes.NewReader(tc.input), 27)
			reader.lastName = tc.lastName

			got, err := reader.readFileName(tc.flags)
			if err != nil {
				t.Fatalf("readFileName: %v", err)
			}
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestFileListReaderReadVarint30(t *testing.T) {
	testCases := []struct {
		input    []byte
		expected int32
	}{
		{[]byte{0}, 0},
		{[]byte{1}, 1},
		{[]byte{127}, 127},
		// Multi-byte values (0x80 + count, then bytes)
		{[]byte{0x81, 0x80}, 128},
		{[]byte{0x82, 0x00, 0x01}, 256},
	}

	for _, tc := range testCases {
		reader := NewFileListReader(bytes.NewReader(tc.input), 30)
		got, err := reader.readVarint30()
		if err != nil {
			t.Fatalf("readVarint30(%v): %v", tc.input, err)
		}
		if got != tc.expected {
			t.Errorf("readVarint30(%v) = %d, expected %d", tc.input, got, tc.expected)
		}
	}
}

func TestFileListXmitFlags(t *testing.T) {
	// Verify XMIT flags match rsync implementation
	if XmitTopDir != 1<<0 {
		t.Error("XmitTopDir mismatch")
	}
	if XmitSameMode != 1<<1 {
		t.Error("XmitSameMode mismatch")
	}
	if XmitExtendedFlags != 1<<2 {
		t.Error("XmitExtendedFlags mismatch")
	}
	if XmitSameUID != 1<<3 {
		t.Error("XmitSameUID mismatch")
	}
	if XmitSameGID != 1<<4 {
		t.Error("XmitSameGID mismatch")
	}
	if XmitSameName != 1<<5 {
		t.Error("XmitSameName mismatch")
	}
	if XmitLongName != 1<<6 {
		t.Error("XmitLongName mismatch")
	}
	if XmitSameTime != 1<<7 {
		t.Error("XmitSameTime mismatch")
	}
}

func TestFileEntryFields(t *testing.T) {
	// Test FileEntry struct fields
	entry := FileEntry{
		Path:  "/test/path",
		Mode:  os.ModeDir | 0755,
		Size:  12345,
		IsDir: true,
		Link:  "",
	}

	if !entry.IsDir {
		t.Error("expected IsDir to be true")
	}
	if entry.Mode&os.ModeDir == 0 {
		t.Error("expected ModeDir to be set")
	}
	if entry.Size != 12345 {
		t.Errorf("expected Size 12345, got %d", entry.Size)
	}
}
