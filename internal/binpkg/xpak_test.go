package binpkg

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// createTestXPAK creates a valid XPAK structure for testing.
func createTestXPAK(entries map[string][]byte) []byte {
	var indexBuf bytes.Buffer
	var dataBuf bytes.Buffer

	// Build index and data areas
	for key, value := range entries {
		// Write index entry
		keyBytes := []byte(key)

		// key_len (4 bytes)
		_ = binary.Write(&indexBuf, binary.BigEndian, uint32(len(keyBytes)))

		// key
		indexBuf.Write(keyBytes)

		// data_len (4 bytes)
		_ = binary.Write(&indexBuf, binary.BigEndian, uint32(len(value)))

		// data_offset (4 bytes)
		_ = binary.Write(&indexBuf, binary.BigEndian, uint32(dataBuf.Len()))

		// Write data
		dataBuf.Write(value)
	}

	indexData := indexBuf.Bytes()
	dataData := dataBuf.Bytes()

	// Build XPAK structure
	// Format: XPAKPACK + index + data + (index_len + data_len + xpak_len + XPAKSTOP)
	var xpakBuf bytes.Buffer

	// XPAKPACK magic (part of xpak_len)
	xpakBuf.WriteString(xpakMagic)

	// Index area
	xpakBuf.Write(indexData)

	// Data area
	xpakBuf.Write(dataData)

	// Calculate xpak_len = len(XPAKPACK) + len(index) + len(data)
	xpakLen := uint32(8 + len(indexData) + len(dataData))

	// Footer: index_len (4) + data_len (4) + xpak_len (4) + XPAKSTOP (8)
	var footerBuf bytes.Buffer
	_ = binary.Write(&footerBuf, binary.BigEndian, uint32(len(indexData)))
	_ = binary.Write(&footerBuf, binary.BigEndian, uint32(len(dataData)))
	_ = binary.Write(&footerBuf, binary.BigEndian, xpakLen)
	footerBuf.WriteString(xpakStop)

	xpakBuf.Write(footerBuf.Bytes())

	return xpakBuf.Bytes()
}

func TestParseXPAK_Valid(t *testing.T) {
	entries := map[string][]byte{
		"EAPI":       []byte("8"),
		"USE":        []byte("ssl python"),
		"BUILD_TIME": []byte("1234567890"),
		"CFLAGS":     []byte("-O2 -pipe"),
	}

	xpakData := createTestXPAK(entries)
	reader := bytes.NewReader(xpakData)

	xpak, err := ParseXPAK(reader)
	if err != nil {
		t.Fatalf("ParseXPAK() failed: %v", err)
	}

	// Verify all entries
	for key, expectedValue := range entries {
		actualValue, exists := xpak.Get(key)
		if !exists {
			t.Errorf("Key %s not found in XPAK", key)
			continue
		}

		if !bytes.Equal(actualValue, expectedValue) {
			t.Errorf("Value for key %s = %s, expected %s", key, actualValue, expectedValue)
		}
	}

	// Verify number of entries
	if len(xpak.Entries) != len(entries) {
		t.Errorf("Number of entries = %d, expected %d", len(xpak.Entries), len(entries))
	}
}

func TestParseXPAK_GetString(t *testing.T) {
	entries := map[string][]byte{
		"EAPI": []byte("8"),
		"USE":  []byte("ssl python"),
	}

	xpakData := createTestXPAK(entries)
	reader := bytes.NewReader(xpakData)

	xpak, err := ParseXPAK(reader)
	if err != nil {
		t.Fatalf("ParseXPAK() failed: %v", err)
	}

	// Test GetString
	eapi, exists := xpak.GetString("EAPI")
	if !exists {
		t.Error("EAPI key not found")
	}
	if eapi != "8" {
		t.Errorf("EAPI = %s, expected 8", eapi)
	}

	// Test non-existent key
	_, exists = xpak.GetString("NONEXISTENT")
	if exists {
		t.Error("GetString() should return false for non-existent key")
	}
}

func TestParseXPAK_Keys(t *testing.T) {
	entries := map[string][]byte{
		"EAPI":  []byte("8"),
		"USE":   []byte("ssl"),
		"CHOST": []byte("x86_64-pc-linux-gnu"),
	}

	xpakData := createTestXPAK(entries)
	reader := bytes.NewReader(xpakData)

	xpak, err := ParseXPAK(reader)
	if err != nil {
		t.Fatalf("ParseXPAK() failed: %v", err)
	}

	keys := xpak.Keys()
	if len(keys) != len(entries) {
		t.Errorf("Keys() returned %d keys, expected %d", len(keys), len(entries))
	}

	// Verify all keys present
	keyMap := make(map[string]bool)
	for _, key := range keys {
		keyMap[key] = true
	}

	for expectedKey := range entries {
		if !keyMap[expectedKey] {
			t.Errorf("Key %s not found in Keys() result", expectedKey)
		}
	}
}

func TestParseXPAK_InvalidMagic(t *testing.T) {
	// Create XPAK with invalid magic in header
	var buf bytes.Buffer
	buf.WriteString("INVALID!")
	buf.Write(make([]byte, 100)) // Padding

	// Footer (valid structure but data has invalid magic)
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))   // index_len
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))   // data_len
	_ = binary.Write(&buf, binary.BigEndian, uint32(108)) // xpak_len (8 + 100)
	buf.WriteString(xpakStop)

	reader := bytes.NewReader(buf.Bytes())

	_, err := ParseXPAK(reader)
	if err == nil {
		t.Error("ParseXPAK() should fail with invalid magic")
	}
}

func TestParseXPAK_EmptyEntries(t *testing.T) {
	entries := map[string][]byte{}

	xpakData := createTestXPAK(entries)
	reader := bytes.NewReader(xpakData)

	xpak, err := ParseXPAK(reader)
	if err != nil {
		t.Fatalf("ParseXPAK() failed: %v", err)
	}

	if len(xpak.Entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(xpak.Entries))
	}
}

func TestParseXPAK_LargeValues(t *testing.T) {
	largeData := make([]byte, 10000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	entries := map[string][]byte{
		"LARGE": largeData,
		"SMALL": []byte("test"),
	}

	xpakData := createTestXPAK(entries)
	reader := bytes.NewReader(xpakData)

	xpak, err := ParseXPAK(reader)
	if err != nil {
		t.Fatalf("ParseXPAK() failed: %v", err)
	}

	largeValue, exists := xpak.Get("LARGE")
	if !exists {
		t.Fatal("LARGE key not found")
	}

	if !bytes.Equal(largeValue, largeData) {
		t.Error("Large value corrupted")
	}
}

func TestParseXPAK_TooShort(t *testing.T) {
	// Create a buffer that's too short to be valid XPAK
	buf := bytes.NewReader([]byte("short"))

	_, err := ParseXPAK(buf)
	if err == nil {
		t.Error("ParseXPAK() should fail with too-short data")
	}
}

func BenchmarkParseXPAK(b *testing.B) {
	entries := map[string][]byte{
		"EAPI":       []byte("8"),
		"USE":        []byte("ssl python xml unicode"),
		"BUILD_TIME": []byte("1234567890"),
		"CFLAGS":     []byte("-O2 -pipe -march=native"),
		"CXXFLAGS":   []byte("-O2 -pipe -march=native"),
		"LDFLAGS":    []byte("-Wl,-O1 -Wl,--as-needed"),
	}

	xpakData := createTestXPAK(entries)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(xpakData)
		_, _ = ParseXPAK(reader)
	}
}

func BenchmarkXPAK_GetString(b *testing.B) {
	entries := map[string][]byte{
		"EAPI": []byte("8"),
		"USE":  []byte("ssl python"),
	}

	xpakData := createTestXPAK(entries)
	reader := bytes.NewReader(xpakData)
	xpak, _ := ParseXPAK(reader)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = xpak.GetString("EAPI")
	}
}

// TestXPAKRoundTrip tests that we can create and parse XPAK correctly.
func TestXPAKRoundTrip(t *testing.T) {
	original := map[string][]byte{
		"KEY1": []byte("value1"),
		"KEY2": []byte("value with spaces"),
		"KEY3": []byte(""),
		"KEY4": []byte("multiline\nvalue\ntest"),
	}

	// Create XPAK
	xpakData := createTestXPAK(original)

	// Parse it back
	reader := bytes.NewReader(xpakData)
	xpak, err := ParseXPAK(reader)
	if err != nil {
		t.Fatalf("ParseXPAK() failed: %v", err)
	}

	// Verify round-trip
	for key, expectedValue := range original {
		actualValue, exists := xpak.Get(key)
		if !exists {
			t.Errorf("Key %s not found after round-trip", key)
			continue
		}

		if !bytes.Equal(actualValue, expectedValue) {
			t.Errorf("Value mismatch for key %s after round-trip", key)
		}
	}
}

// TestXPAKSeeker tests that ParseXPAK works with io.ReadSeeker.
func TestXPAKSeeker(t *testing.T) {
	entries := map[string][]byte{
		"TEST": []byte("value"),
	}

	xpakData := createTestXPAK(entries)

	// Add some data before XPAK (simulating tar.bz2 data)
	prefix := []byte("some tar data here...")
	combined := append(prefix, xpakData...)

	reader := bytes.NewReader(combined)

	// Parse should work even with data before XPAK
	xpak, err := ParseXPAK(reader)
	if err != nil {
		t.Fatalf("ParseXPAK() failed: %v", err)
	}

	value, exists := xpak.GetString("TEST")
	if !exists || value != "value" {
		t.Error("Failed to parse XPAK with prefix data")
	}
}
