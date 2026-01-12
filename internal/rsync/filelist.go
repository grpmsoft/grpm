package rsync

import (
	"fmt"
	"io"
	"os"
	"time"
)

// FileList represents a collection of files from rsync server.
type FileList struct {
	Files []FileEntry
}

// FileEntry represents a file in the rsync file list.
type FileEntry struct {
	Path    string      // Full path relative to module root
	Mode    os.FileMode // Unix file mode
	Size    int64       // File size in bytes
	ModTime time.Time   // Modification time
	IsDir   bool        // True if directory
	Link    string      // Symlink target (if symlink)
}

// FileListReader reads file lists from rsync protocol stream.
// Protocol reference: rsync/flist.c:recv_file_entry()
type FileListReader struct {
	reader io.Reader
	// State preserved between file entries
	lastName string
	lastMode os.FileMode
	lastTime time.Time
	// Protocol version
	version int
}

// NewFileListReader creates a new file list reader.
func NewFileListReader(r io.Reader, version int) *FileListReader {
	return &FileListReader{
		reader:  r,
		version: version,
	}
}

// ReadFileList reads all file entries until end marker.
// Returns the complete file list or error.
//
// Wire format (protocol 27):
//   - flags byte (0 = end of list)
//   - file name (compressed against previous)
//   - file size (varlong)
//   - mod time (if not XMIT_SAME_TIME)
//   - mode (if not XMIT_SAME_MODE)
//   - uid/gid (if preserving)
//   - symlink target (if symlink)
//
// Reference: rsync/flist.c:recv_file_list()
func (r *FileListReader) ReadFileList() (*FileList, error) {
	var files []FileEntry

	for {
		// Read flags byte
		flagsByte, err := r.readByte()
		if err != nil {
			return nil, fmt.Errorf("read flags: %w", err)
		}

		// End of list marker (flags == 0)
		if flagsByte == 0 {
			break
		}

		// Convert to uint16 for extended flags support
		flags := uint16(flagsByte)

		// For protocol >= 28, check for extended flags
		if r.version >= 28 && (flags&XmitExtendedFlags) != 0 {
			// Read extended flags byte
			extFlags, err := r.readByte()
			if err != nil {
				return nil, fmt.Errorf("read ext flags: %w", err)
			}
			flags = flags | (uint16(extFlags) << 8)
		}

		entry, err := r.readEntry(flags)
		if err != nil {
			return nil, fmt.Errorf("read entry: %w", err)
		}

		files = append(files, entry)
	}

	return &FileList{Files: files}, nil
}

// readEntry reads a single file entry from the wire.
// Reference: rsync/flist.c:recv_file_entry() lines 700-1100
func (r *FileListReader) readEntry(flags uint16) (FileEntry, error) {
	var entry FileEntry

	// Read file name
	name, err := r.readFileName(flags)
	if err != nil {
		return entry, fmt.Errorf("read name: %w", err)
	}
	entry.Path = name
	r.lastName = name

	// Read file size (protocol 27 uses 32-bit, protocol 30+ uses varlong)
	if r.version >= 30 {
		entry.Size, err = r.readVarlong30(3)
	} else {
		size, err := r.readInt32()
		if err != nil {
			return entry, fmt.Errorf("read size: %w", err)
		}
		entry.Size = int64(size)
	}
	if err != nil {
		return entry, fmt.Errorf("read size: %w", err)
	}

	// Read mod time (if not same as previous)
	if (flags & XmitSameTime) == 0 {
		mtime, err := r.readInt32()
		if err != nil {
			return entry, fmt.Errorf("read mtime: %w", err)
		}
		entry.ModTime = time.Unix(int64(mtime), 0)
		r.lastTime = entry.ModTime
	} else {
		entry.ModTime = r.lastTime
	}

	// Read file mode (if not same as previous)
	if (flags & XmitSameMode) == 0 {
		mode, err := r.readInt32()
		if err != nil {
			return entry, fmt.Errorf("read mode: %w", err)
		}
		entry.Mode = fromWireMode(uint32(mode))
		r.lastMode = entry.Mode
	} else {
		entry.Mode = r.lastMode
	}

	entry.IsDir = entry.Mode.IsDir()

	// Handle symlinks
	if entry.Mode&os.ModeSymlink != 0 {
		linkLen, err := r.readInt32()
		if err != nil {
			return entry, fmt.Errorf("read link length: %w", err)
		}
		if linkLen > 0 && linkLen < 65536 {
			linkData := make([]byte, linkLen)
			if _, err := io.ReadFull(r.reader, linkData); err != nil {
				return entry, fmt.Errorf("read link target: %w", err)
			}
			entry.Link = string(linkData)
		}
	}

	return entry, nil
}

// readFileName reads a file name, possibly compressed against previous name.
//
// Wire format:
//   - If XMIT_SAME_NAME: read l1 (byte) = shared prefix length
//   - If XMIT_LONG_NAME: read l2 (int32) = unique suffix length
//   - Else: read l2 (byte) = unique suffix length
//   - Read l2 bytes of unique suffix
//   - Result = lastName[:l1] + suffix
//
// Reference: rsync/flist.c lines 716-733
func (r *FileListReader) readFileName(flags uint16) (string, error) {
	var l1 int // Shared prefix length
	var l2 int // Unique suffix length

	// Read shared prefix length
	if (flags & XmitSameName) != 0 {
		b, err := r.readByte()
		if err != nil {
			return "", err
		}
		l1 = int(b)
	}

	// Read unique suffix length
	if (flags & XmitLongName) != 0 {
		// For protocol < 30, long name uses 32-bit length
		if r.version >= 30 {
			v, err := r.readVarint30()
			if err != nil {
				return "", err
			}
			l2 = int(v)
		} else {
			v, err := r.readInt32()
			if err != nil {
				return "", err
			}
			l2 = int(v)
		}
	} else {
		b, err := r.readByte()
		if err != nil {
			return "", err
		}
		l2 = int(b)
	}

	// Validate lengths
	if l1 > len(r.lastName) {
		l1 = len(r.lastName)
	}
	if l2 > 65536 {
		return "", fmt.Errorf("file name too long: %d", l2)
	}

	// Read unique suffix
	suffix := make([]byte, l2)
	if _, err := io.ReadFull(r.reader, suffix); err != nil {
		return "", err
	}

	// Combine prefix and suffix
	var name string
	if l1 > 0 {
		name = r.lastName[:l1] + string(suffix)
	} else {
		name = string(suffix)
	}

	return name, nil
}

// fromWireMode converts rsync wire mode to os.FileMode.
// Reference: rsync/lib/sysxattrs.c:from_wire_mode()
func fromWireMode(mode uint32) os.FileMode {
	// Unix permission bits
	perm := os.FileMode(mode & 0777)

	// File type (stored in high bits)
	switch mode & 0xF000 {
	case 0x8000: // S_IFREG - regular file
		return perm
	case 0x4000: // S_IFDIR - directory
		return perm | os.ModeDir
	case 0xA000: // S_IFLNK - symbolic link
		return perm | os.ModeSymlink
	case 0x6000: // S_IFBLK - block device
		return perm | os.ModeDevice
	case 0x2000: // S_IFCHR - character device
		return perm | os.ModeDevice | os.ModeCharDevice
	case 0xC000: // S_IFSOCK - socket
		return perm | os.ModeSocket
	case 0x1000: // S_IFIFO - named pipe
		return perm | os.ModeNamedPipe
	default:
		return perm
	}
}

// readByte reads a single byte.
func (r *FileListReader) readByte() (byte, error) {
	var buf [1]byte
	if _, err := io.ReadFull(r.reader, buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}

// readInt32 reads a 32-bit little-endian integer.
func (r *FileListReader) readInt32() (int32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r.reader, buf[:]); err != nil {
		return 0, err
	}
	return int32(buf[0]) | int32(buf[1])<<8 | int32(buf[2])<<16 | int32(buf[3])<<24, nil
}

// readVarint30 reads a variable-length integer (protocol 30+).
// Format: 1-5 bytes depending on value magnitude.
func (r *FileListReader) readVarint30() (int32, error) {
	b, err := r.readByte()
	if err != nil {
		return 0, err
	}

	// Single byte: 0-127
	if b < 0x80 {
		return int32(b), nil
	}

	// Multi-byte encoding
	var result int32
	cnt := int(b & 0x7F)
	if cnt > 4 {
		return 0, fmt.Errorf("invalid varint: %d bytes", cnt)
	}

	for i := 0; i < cnt; i++ {
		b, err := r.readByte()
		if err != nil {
			return 0, err
		}
		result |= int32(b) << uint(i*8)
	}

	return result, nil
}

// readVarlong30 reads a variable-length 64-bit integer (protocol 30+).
// Reference: rsync/io.c:read_varlong30()
func (r *FileListReader) readVarlong30(minBytes int) (int64, error) {
	b, err := r.readByte()
	if err != nil {
		return 0, err
	}

	// Determine number of extra bytes from first byte
	extra := 0
	switch {
	case b == 0xFF:
		extra = minBytes + 8 - 1
	case (b & 0x80) == 0:
		extra = minBytes - 1
	case (b & 0xC0) == 0x80:
		extra = minBytes
	case (b & 0xE0) == 0xC0:
		extra = minBytes + 1
	case (b & 0xF0) == 0xE0:
		extra = minBytes + 2
	case (b & 0xF8) == 0xF0:
		extra = minBytes + 3
	case (b & 0xFC) == 0xF8:
		extra = minBytes + 4
	case (b & 0xFE) == 0xFC:
		extra = minBytes + 5
	case b == 0xFE:
		extra = minBytes + 6
	}

	if extra > 8 {
		extra = 8
	}

	var result int64
	if b != 0xFF {
		// Mask off the encoding bits
		mask := byte(0xFF >> (8 - extra + minBytes))
		result = int64(b & mask)
	}

	// Read extra bytes
	for i := 0; i < extra; i++ {
		b, err := r.readByte()
		if err != nil {
			return 0, err
		}
		result |= int64(b) << uint((i+1)*8)
	}

	return result, nil
}
