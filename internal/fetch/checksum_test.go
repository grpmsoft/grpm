package fetch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Known test data - "hello world\n" (12 bytes)
const testContent = "hello world\n"

// Pre-computed hashes for "hello world\n"
// Generated with: echo -n "hello world\n" | sha256sum / sha512sum / b2sum
const (
	// SHA256 of "hello world\n"
	testSHA256 = "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447"
	// SHA512 of "hello world\n"
	testSHA512 = "db3974a97f2407b7cae1ae637c0030687a11913274d578492558e39c16c017de84eacdc8c62fe34ee4e12b4b1428817f09b6a2760c3f8a664ceae94d2434a593"
	// BLAKE2B-512 of "hello world\n"
	testBLAKE2B = "fec91c70284c72d0d4e3684788a90de9338a5b2f47f01fedbe203cafd68708718ae5672d10eca804a8121904047d40d1d6cf11e7a76419357a9469af41f22d01"
)

// createTestFile creates a temporary file with test content
func createTestFile(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "testfile")

	if err := os.WriteFile(path, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	return path
}

// TestComputeHash tests hash computation for all algorithms
func TestComputeHash(t *testing.T) {
	path := createTestFile(t)

	tests := []struct {
		algorithm Algorithm
		expected  string
	}{
		{AlgorithmSHA256, testSHA256},
		{AlgorithmSHA512, testSHA512},
		{AlgorithmBLAKE2B, testBLAKE2B},
	}

	for _, tt := range tests {
		t.Run(string(tt.algorithm), func(t *testing.T) {
			hash, err := ComputeHash(path, tt.algorithm)
			if err != nil {
				t.Fatalf("ComputeHash failed: %v", err)
			}

			if hash != tt.expected {
				t.Errorf("hash mismatch:\ngot:  %s\nwant: %s", hash, tt.expected)
			}
		})
	}
}

// TestComputeHashFromReader tests hash computation from io.Reader
func TestComputeHashFromReader(t *testing.T) {
	tests := []struct {
		algorithm Algorithm
		expected  string
	}{
		{AlgorithmSHA256, testSHA256},
		{AlgorithmSHA512, testSHA512},
		{AlgorithmBLAKE2B, testBLAKE2B},
	}

	for _, tt := range tests {
		t.Run(string(tt.algorithm), func(t *testing.T) {
			reader := strings.NewReader(testContent)
			hash, err := ComputeHashFromReader(reader, tt.algorithm)
			if err != nil {
				t.Fatalf("ComputeHashFromReader failed: %v", err)
			}

			if hash != tt.expected {
				t.Errorf("hash mismatch:\ngot:  %s\nwant: %s", hash, tt.expected)
			}
		})
	}
}

// TestComputeHashUnsupportedAlgorithm tests error for unsupported algorithm
func TestComputeHashUnsupportedAlgorithm(t *testing.T) {
	path := createTestFile(t)

	_, err := ComputeHash(path, Algorithm("MD5"))
	if err == nil {
		t.Error("expected error for unsupported algorithm")
	}

	if !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Errorf("expected ErrUnsupportedAlgorithm, got: %v", err)
	}
}

// TestComputeHashNonexistentFile tests error for nonexistent file
func TestComputeHashNonexistentFile(t *testing.T) {
	_, err := ComputeHash("/nonexistent/file", AlgorithmSHA256)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// TestVerify tests checksum verification
func TestVerify(t *testing.T) {
	path := createTestFile(t)

	t.Run("verify success with BLAKE2B", func(t *testing.T) {
		checksums := NewChecksums("", "", testBLAKE2B)
		if err := Verify(path, checksums); err != nil {
			t.Errorf("Verify failed: %v", err)
		}
	})

	t.Run("verify success with SHA512", func(t *testing.T) {
		checksums := NewChecksums("", testSHA512, "")
		if err := Verify(path, checksums); err != nil {
			t.Errorf("Verify failed: %v", err)
		}
	})

	t.Run("verify success with SHA256", func(t *testing.T) {
		checksums := NewChecksums(testSHA256, "", "")
		if err := Verify(path, checksums); err != nil {
			t.Errorf("Verify failed: %v", err)
		}
	})

	t.Run("verify failure - wrong hash", func(t *testing.T) {
		checksums := NewChecksums("wronghash", "", "")
		err := Verify(path, checksums)
		if err == nil {
			t.Error("expected error for wrong hash")
		}

		if !errors.Is(err, ErrChecksumMismatch) {
			t.Errorf("expected ErrChecksumMismatch, got: %v", err)
		}
	})

	t.Run("verify failure - no checksums", func(t *testing.T) {
		checksums := Checksums{}
		err := Verify(path, checksums)
		if err == nil {
			t.Error("expected error for no checksums")
		}

		if !errors.Is(err, ErrNoChecksum) {
			t.Errorf("expected ErrNoChecksum, got: %v", err)
		}
	})
}

// TestVerifyWithResult tests detailed verification results
func TestVerifyWithResult(t *testing.T) {
	path := createTestFile(t)

	t.Run("successful verification", func(t *testing.T) {
		checksums := NewChecksums(testSHA256, testSHA512, testBLAKE2B)
		result, err := VerifyWithResult(path, checksums)
		if err != nil {
			t.Fatalf("VerifyWithResult failed: %v", err)
		}

		// Should use BLAKE2B (preferred)
		if result.Algorithm != AlgorithmBLAKE2B {
			t.Errorf("expected algorithm BLAKE2B, got %s", result.Algorithm)
		}

		if result.Expected != testBLAKE2B {
			t.Errorf("expected hash %s, got %s", testBLAKE2B, result.Expected)
		}

		if !result.Match {
			t.Error("expected Match to be true")
		}

		if result.Actual != testBLAKE2B {
			t.Errorf("actual hash mismatch: got %s, want %s", result.Actual, testBLAKE2B)
		}
	})

	t.Run("failed verification", func(t *testing.T) {
		checksums := NewChecksums("wronghash", "", "")
		result, err := VerifyWithResult(path, checksums)
		if err != nil {
			t.Fatalf("VerifyWithResult should not error: %v", err)
		}

		if result.Match {
			t.Error("expected Match to be false for wrong hash")
		}

		if result.Expected != "wronghash" {
			t.Errorf("expected hash 'wronghash', got %s", result.Expected)
		}
	})
}

// TestVerifyAll tests comprehensive verification
func TestVerifyAll(t *testing.T) {
	path := createTestFile(t)

	t.Run("all checksums match", func(t *testing.T) {
		checksums := NewChecksums(testSHA256, testSHA512, testBLAKE2B)
		results, err := VerifyAll(path, checksums)
		if err != nil {
			t.Fatalf("VerifyAll failed: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("expected 3 results, got %d", len(results))
		}

		for _, r := range results {
			if !r.Match {
				t.Errorf("expected all checksums to match, %s did not", r.Algorithm)
			}
		}
	})

	t.Run("partial checksums", func(t *testing.T) {
		checksums := NewChecksums(testSHA256, "", "")
		results, err := VerifyAll(path, checksums)
		if err != nil {
			t.Fatalf("VerifyAll failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}

		if results[0].Algorithm != AlgorithmSHA256 {
			t.Errorf("expected SHA256, got %s", results[0].Algorithm)
		}
	})

	t.Run("first mismatch fails", func(t *testing.T) {
		checksums := NewChecksums("", "", "wrongblake2b")
		_, err := VerifyAll(path, checksums)
		if err == nil {
			t.Error("expected error for wrong hash")
		}

		if !errors.Is(err, ErrChecksumMismatch) {
			t.Errorf("expected ErrChecksumMismatch, got: %v", err)
		}
	})

	t.Run("no checksums", func(t *testing.T) {
		checksums := Checksums{}
		_, err := VerifyAll(path, checksums)
		if err == nil {
			t.Error("expected error for no checksums")
		}

		if !errors.Is(err, ErrNoChecksum) {
			t.Errorf("expected ErrNoChecksum, got: %v", err)
		}
	})
}

// TestComputeChecksums tests computing all checksums
func TestComputeChecksums(t *testing.T) {
	path := createTestFile(t)

	checksums, err := ComputeChecksums(path)
	if err != nil {
		t.Fatalf("ComputeChecksums failed: %v", err)
	}

	if checksums.SHA256 != testSHA256 {
		t.Errorf("SHA256 mismatch:\ngot:  %s\nwant: %s", checksums.SHA256, testSHA256)
	}

	if checksums.SHA512 != testSHA512 {
		t.Errorf("SHA512 mismatch:\ngot:  %s\nwant: %s", checksums.SHA512, testSHA512)
	}

	if checksums.BLAKE2B != testBLAKE2B {
		t.Errorf("BLAKE2B mismatch:\ngot:  %s\nwant: %s", checksums.BLAKE2B, testBLAKE2B)
	}
}

// TestComputeChecksumsNonexistentFile tests error handling
func TestComputeChecksumsNonexistentFile(t *testing.T) {
	_, err := ComputeChecksums("/nonexistent/file")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// TestNewHasher tests hasher creation
func TestNewHasher(t *testing.T) {
	tests := []struct {
		algorithm Algorithm
		wantError bool
	}{
		{AlgorithmSHA256, false},
		{AlgorithmSHA512, false},
		{AlgorithmBLAKE2B, false},
		{Algorithm("INVALID"), true},
	}

	for _, tt := range tests {
		t.Run(string(tt.algorithm), func(t *testing.T) {
			_, err := newHasher(tt.algorithm)
			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
