package binpkg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// XPAK is the metadata format used in TBZ2 packages.
//
// Format (little-endian):
//   - Index area: key_len (4) + key + data_len (4) + data_offset (4)
//   - Data area: concatenated data values
//   - Footer: "XPAKPACK" + index_len (4) + data_len (4) + xpak_len (4) + "XPAKSTOP"
//
// See: https://wiki.gentoo.org/wiki/XPAK
type XPAK struct {
	// Entries maps metadata keys to values
	Entries map[string][]byte
}

// Magic numbers for XPAK format
const (
	xpakMagic = "XPAKPACK"
	xpakStop  = "XPAKSTOP"
)

// ParseXPAK parses XPAK metadata from a reader.
//
// The reader should be positioned at the start of XPAK data.
func ParseXPAK(r io.ReadSeeker) (*XPAK, error) {
	// Find XPAK footer by seeking from end
	// Footer format: index_len (4) + data_len (4) + xpak_len (4) + XPAKSTOP (8)
	const footerSize = 4 + 4 + 4 + 8 // 20 bytes

	// Seek to footer
	if _, err := r.Seek(-footerSize, io.SeekEnd); err != nil {
		return nil, fmt.Errorf("failed to seek to XPAK footer: %w", err)
	}

	// Read footer
	footer := make([]byte, footerSize)
	if _, err := io.ReadFull(r, footer); err != nil {
		return nil, fmt.Errorf("failed to read XPAK footer: %w", err)
	}

	// Verify XPAKSTOP magic
	if string(footer[12:20]) != xpakStop {
		return nil, fmt.Errorf("invalid XPAK stop: expected %s, got %s", xpakStop, string(footer[12:20]))
	}

	// Parse lengths
	indexLen := binary.BigEndian.Uint32(footer[0:4])
	dataLen := binary.BigEndian.Uint32(footer[4:8])
	xpakLen := binary.BigEndian.Uint32(footer[8:12])

	// Verify xpakLen = indexLen + dataLen + 8 (for "XPAKPACK")
	expectedLen := indexLen + dataLen + 8
	if xpakLen != expectedLen {
		return nil, fmt.Errorf("XPAK length mismatch: expected %d, got %d", expectedLen, xpakLen)
	}

	// Seek to start of XPAK data
	// xpakStart = fileSize - xpakLen - footerSize
	xpakStart := -(int64(xpakLen) + footerSize)
	if _, err := r.Seek(xpakStart, io.SeekEnd); err != nil {
		return nil, fmt.Errorf("failed to seek to XPAK start: %w", err)
	}

	// Read XPAK data
	xpakData := make([]byte, xpakLen)
	if _, err := io.ReadFull(r, xpakData); err != nil {
		return nil, fmt.Errorf("failed to read XPAK data: %w", err)
	}

	// Verify magic again
	if string(xpakData[0:8]) != xpakMagic {
		return nil, fmt.Errorf("invalid XPAK header magic")
	}

	// Parse index and data
	indexData := xpakData[8 : 8+indexLen]
	dataArea := xpakData[8+indexLen : 8+indexLen+dataLen]

	return parseXPAKEntries(indexData, dataArea)
}

// parseXPAKEntries parses XPAK index and data areas.
func parseXPAKEntries(indexData, dataArea []byte) (*XPAK, error) {
	entries := make(map[string][]byte)

	indexReader := bytes.NewReader(indexData)

	for indexReader.Len() > 0 {
		// Read key length (4 bytes)
		var keyLen uint32
		if err := binary.Read(indexReader, binary.BigEndian, &keyLen); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to read key length: %w", err)
		}

		// Read key
		key := make([]byte, keyLen)
		if _, err := io.ReadFull(indexReader, key); err != nil {
			return nil, fmt.Errorf("failed to read key: %w", err)
		}

		// Read data length (4 bytes)
		var dataLen uint32
		if err := binary.Read(indexReader, binary.BigEndian, &dataLen); err != nil {
			return nil, fmt.Errorf("failed to read data length: %w", err)
		}

		// Read data offset (4 bytes)
		var dataOffset uint32
		if err := binary.Read(indexReader, binary.BigEndian, &dataOffset); err != nil {
			return nil, fmt.Errorf("failed to read data offset: %w", err)
		}

		// Validate offset and length
		if dataOffset+dataLen > uint32(len(dataArea)) {
			return nil, fmt.Errorf("data offset %d + length %d exceeds data area size %d",
				dataOffset, dataLen, len(dataArea))
		}

		// Extract data
		data := dataArea[dataOffset : dataOffset+dataLen]

		// Store entry
		entries[string(key)] = data
	}

	return &XPAK{
		Entries: entries,
	}, nil
}

// Get returns the value for a metadata key.
func (x *XPAK) Get(key string) ([]byte, bool) {
	value, exists := x.Entries[key]
	return value, exists
}

// GetString returns the value as a string.
func (x *XPAK) GetString(key string) (string, bool) {
	value, exists := x.Entries[key]
	if !exists {
		return "", false
	}
	return string(value), true
}

// Keys returns all metadata keys.
func (x *XPAK) Keys() []string {
	keys := make([]string, 0, len(x.Entries))
	for key := range x.Entries {
		keys = append(keys, key)
	}
	return keys
}
