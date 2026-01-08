package fetch

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"

	"golang.org/x/crypto/blake2b"
)

// ErrNoChecksum indicates no checksum was provided for verification.
var ErrNoChecksum = errors.New("no checksum provided")

// ErrUnsupportedAlgorithm indicates an unsupported hash algorithm was requested.
var ErrUnsupportedAlgorithm = errors.New("unsupported hash algorithm")

// Algorithm represents a cryptographic hash algorithm.
type Algorithm string

const (
	// AlgorithmSHA256 is the SHA-256 algorithm (256-bit, 64 hex chars)
	AlgorithmSHA256 Algorithm = "SHA256"

	// AlgorithmSHA512 is the SHA-512 algorithm (512-bit, 128 hex chars)
	AlgorithmSHA512 Algorithm = "SHA512"

	// AlgorithmBLAKE2B is the BLAKE2b algorithm (512-bit, 128 hex chars)
	// This is the preferred algorithm in modern Portage
	AlgorithmBLAKE2B Algorithm = "BLAKE2B"
)

// VerifyResult contains the result of a checksum verification.
type VerifyResult struct {
	// Algorithm is the algorithm used for verification
	Algorithm Algorithm

	// Expected is the expected hash value
	Expected string

	// Actual is the computed hash value
	Actual string

	// Match indicates if the hashes match
	Match bool
}

// Verify verifies the checksum of a file against expected values.
//
// The function tries algorithms in order of preference: BLAKE2B > SHA512 > SHA256.
// It uses the first algorithm for which a checksum is provided.
//
// Returns nil if verification succeeds, ErrChecksumMismatch if the computed
// hash does not match, or ErrNoChecksum if no checksums were provided.
func Verify(path string, expected Checksums) error {
	result, err := VerifyWithResult(path, expected)
	if err != nil {
		return err
	}

	if !result.Match {
		return fmt.Errorf("%w: %s expected %s, got %s",
			ErrChecksumMismatch, result.Algorithm, result.Expected, result.Actual)
	}

	return nil
}

// VerifyWithResult verifies the checksum and returns detailed results.
//
// This function allows callers to inspect which algorithm was used
// and what the actual hash value is, useful for debugging.
func VerifyWithResult(path string, expected Checksums) (*VerifyResult, error) {
	algorithm, expectedHash := expected.Preferred()
	if algorithm == "" {
		return nil, ErrNoChecksum
	}

	actualHash, err := ComputeHash(path, Algorithm(algorithm))
	if err != nil {
		return nil, fmt.Errorf("computing %s hash: %w", algorithm, err)
	}

	return &VerifyResult{
		Algorithm: Algorithm(algorithm),
		Expected:  expectedHash,
		Actual:    actualHash,
		Match:     actualHash == expectedHash,
	}, nil
}

// VerifyAll verifies all provided checksums and returns results for each.
//
// This is useful for comprehensive verification when multiple algorithms
// are available. Returns an error if any checksum fails.
func VerifyAll(path string, expected Checksums) ([]VerifyResult, error) {
	var results []VerifyResult

	if expected.BLAKE2B != "" {
		hash, err := ComputeHash(path, AlgorithmBLAKE2B)
		if err != nil {
			return nil, fmt.Errorf("computing BLAKE2B: %w", err)
		}
		result := VerifyResult{
			Algorithm: AlgorithmBLAKE2B,
			Expected:  expected.BLAKE2B,
			Actual:    hash,
			Match:     hash == expected.BLAKE2B,
		}
		results = append(results, result)
		if !result.Match {
			return results, fmt.Errorf("%w: BLAKE2B expected %s, got %s",
				ErrChecksumMismatch, expected.BLAKE2B, hash)
		}
	}

	if expected.SHA512 != "" {
		hash, err := ComputeHash(path, AlgorithmSHA512)
		if err != nil {
			return nil, fmt.Errorf("computing SHA512: %w", err)
		}
		result := VerifyResult{
			Algorithm: AlgorithmSHA512,
			Expected:  expected.SHA512,
			Actual:    hash,
			Match:     hash == expected.SHA512,
		}
		results = append(results, result)
		if !result.Match {
			return results, fmt.Errorf("%w: SHA512 expected %s, got %s",
				ErrChecksumMismatch, expected.SHA512, hash)
		}
	}

	if expected.SHA256 != "" {
		hash, err := ComputeHash(path, AlgorithmSHA256)
		if err != nil {
			return nil, fmt.Errorf("computing SHA256: %w", err)
		}
		result := VerifyResult{
			Algorithm: AlgorithmSHA256,
			Expected:  expected.SHA256,
			Actual:    hash,
			Match:     hash == expected.SHA256,
		}
		results = append(results, result)
		if !result.Match {
			return results, fmt.Errorf("%w: SHA256 expected %s, got %s",
				ErrChecksumMismatch, expected.SHA256, hash)
		}
	}

	if len(results) == 0 {
		return nil, ErrNoChecksum
	}

	return results, nil
}

// ComputeHash computes the hash of a file using the specified algorithm.
//
// Returns the hexadecimal-encoded hash string.
func ComputeHash(path string, algorithm Algorithm) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening file: %w", err)
	}
	defer func() { _ = file.Close() }()

	return ComputeHashFromReader(file, algorithm)
}

// ComputeHashFromReader computes the hash from an io.Reader.
//
// This is useful for computing hashes during download without
// writing to disk first.
func ComputeHashFromReader(r io.Reader, algorithm Algorithm) (string, error) {
	hasher, err := newHasher(algorithm)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(hasher, r); err != nil {
		return "", fmt.Errorf("reading data: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// newHasher creates a new hash.Hash for the specified algorithm.
func newHasher(algorithm Algorithm) (hash.Hash, error) {
	switch algorithm {
	case AlgorithmSHA256:
		return sha256.New(), nil
	case AlgorithmSHA512:
		return sha512.New(), nil
	case AlgorithmBLAKE2B:
		h, err := blake2b.New512(nil)
		if err != nil {
			return nil, fmt.Errorf("creating BLAKE2B hasher: %w", err)
		}
		return h, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, algorithm)
	}
}

// ComputeChecksums computes all three hash algorithms for a file.
//
// This is useful when creating or verifying Manifest entries.
func ComputeChecksums(path string) (Checksums, error) {
	sha256Hash, err := ComputeHash(path, AlgorithmSHA256)
	if err != nil {
		return Checksums{}, fmt.Errorf("computing SHA256: %w", err)
	}

	sha512Hash, err := ComputeHash(path, AlgorithmSHA512)
	if err != nil {
		return Checksums{}, fmt.Errorf("computing SHA512: %w", err)
	}

	blake2bHash, err := ComputeHash(path, AlgorithmBLAKE2B)
	if err != nil {
		return Checksums{}, fmt.Errorf("computing BLAKE2B: %w", err)
	}

	return NewChecksums(sha256Hash, sha512Hash, blake2bHash), nil
}
