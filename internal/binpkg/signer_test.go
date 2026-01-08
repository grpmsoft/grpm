package binpkg

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

// TestNewGPGSigner tests creating a GPG signer.
func TestNewGPGSigner(t *testing.T) {
	signer := NewGPGSigner("AABBCCDD")

	if signer == nil {
		t.Fatal("NewGPGSigner() returned nil")
	}

	if signer.KeyID != "AABBCCDD" {
		t.Errorf("KeyID = %s, want AABBCCDD", signer.KeyID)
	}

	if signer.GPGPath != "gpg" {
		t.Errorf("GPGPath = %s, want gpg", signer.GPGPath)
	}

	if !signer.Armor {
		t.Error("Armor should be true by default")
	}
}

// TestGPGSigner_Sign_FileNotFound tests signing non-existent file.
func TestGPGSigner_Sign_FileNotFound(t *testing.T) {
	signer := NewGPGSigner("AABBCCDD")

	_, err := signer.Sign("/nonexistent/package.gpkg.tar")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

// TestGPGSigner_Verify_InvalidSignatureType tests verification with wrong signature type.
func TestGPGSigner_Verify_InvalidSignatureType(t *testing.T) {
	signer := NewGPGSigner("AABBCCDD")
	tmpDir := t.TempDir()

	pkgPath := filepath.Join(tmpDir, "test.gpkg.tar")
	if err := os.WriteFile(pkgPath, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create signature with wrong type
	sig := &Signature{
		Type:  SignatureRSA, // Wrong type
		Data:  []byte("invalid"),
		KeyID: "test",
	}

	err := signer.Verify(pkgPath, sig)
	if err == nil {
		t.Error("Expected error for invalid signature type")
	}
}

// TestNewSSHSigner tests creating an SSH signer.
func TestNewSSHSigner(t *testing.T) {
	signer := NewSSHSigner("/path/to/key")

	if signer == nil {
		t.Fatal("NewSSHSigner() returned nil")
	}

	if signer.KeyPath != "/path/to/key" {
		t.Errorf("KeyPath = %s, want /path/to/key", signer.KeyPath)
	}

	if signer.SSHKeygenPath != "ssh-keygen" {
		t.Errorf("SSHKeygenPath = %s, want ssh-keygen", signer.SSHKeygenPath)
	}
}

// TestSSHSigner_Sign_FileNotFound tests signing non-existent package.
func TestSSHSigner_Sign_FileNotFound(t *testing.T) {
	signer := NewSSHSigner("/path/to/key")

	_, err := signer.Sign("/nonexistent/package.gpkg.tar")
	if err == nil {
		t.Error("Expected error for non-existent package file")
	}
}

// TestSSHSigner_Sign_KeyNotFound tests signing with non-existent key.
func TestSSHSigner_Sign_KeyNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a package file
	pkgPath := filepath.Join(tmpDir, "test.gpkg.tar")
	if err := os.WriteFile(pkgPath, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	signer := NewSSHSigner("/nonexistent/key")

	_, err := signer.Sign(pkgPath)
	if err == nil {
		t.Error("Expected error for non-existent key")
	}
}

// TestSSHSigner_Verify_InvalidSignatureType tests verification with wrong type.
func TestSSHSigner_Verify_InvalidSignatureType(t *testing.T) {
	signer := NewSSHSigner("/path/to/key")
	tmpDir := t.TempDir()

	pkgPath := filepath.Join(tmpDir, "test.gpkg.tar")
	if err := os.WriteFile(pkgPath, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	sig := &Signature{
		Type:  SignatureGPG, // Wrong type
		Data:  []byte("invalid"),
		KeyID: "test",
	}

	err := signer.Verify(pkgPath, sig)
	if err == nil {
		t.Error("Expected error for invalid signature type")
	}
}

// TestNewRSASigner_InvalidKeyPath tests creating signer with invalid key path.
func TestNewRSASigner_InvalidKeyPath(t *testing.T) {
	_, err := NewRSASigner("/nonexistent/key.pem")
	if err == nil {
		t.Error("Expected error for non-existent key file")
	}
}

// TestNewRSASigner_InvalidPEM tests creating signer with invalid PEM.
func TestNewRSASigner_InvalidPEM(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "invalid.pem")

	// Write invalid PEM data
	if err := os.WriteFile(keyPath, []byte("not valid pem"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := NewRSASigner(keyPath)
	if err == nil {
		t.Error("Expected error for invalid PEM")
	}
}

// TestNewRSASigner_InvalidPrivateKey tests creating signer with invalid key.
func TestNewRSASigner_InvalidPrivateKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "invalid.pem")

	// Write PEM with invalid key data
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: []byte("not a valid key"),
	}
	pemData := pem.EncodeToMemory(block)
	if err := os.WriteFile(keyPath, pemData, 0600); err != nil {
		t.Fatal(err)
	}

	_, err := NewRSASigner(keyPath)
	if err == nil {
		t.Error("Expected error for invalid private key")
	}
}

// TestRSASigner_Sign_FileNotFound tests signing non-existent file.
func TestRSASigner_Sign_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := createTestRSAKey(t, tmpDir)

	signer, err := NewRSASigner(keyPath)
	if err != nil {
		t.Fatalf("NewRSASigner() error = %v", err)
	}

	_, err = signer.Sign("/nonexistent/package.gpkg.tar")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

// TestRSASigner_SignAndVerify tests RSA sign and verify workflow.
func TestRSASigner_SignAndVerify(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := createTestRSAKey(t, tmpDir)

	signer, err := NewRSASigner(keyPath)
	if err != nil {
		t.Fatalf("NewRSASigner() error = %v", err)
	}

	// Create a test package file
	pkgPath := filepath.Join(tmpDir, "test.gpkg.tar")
	pkgContent := []byte("This is a test binary package content")
	if err := os.WriteFile(pkgPath, pkgContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Sign the package
	sig, err := signer.Sign(pkgPath)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	if sig == nil {
		t.Fatal("Sign() returned nil signature")
	}

	if sig.Type != SignatureRSA {
		t.Errorf("Signature type = %v, want %v", sig.Type, SignatureRSA)
	}

	if len(sig.Data) == 0 {
		t.Error("Signature data is empty")
	}

	if sig.KeyID == "" {
		t.Error("Signature KeyID is empty")
	}

	if sig.Created.IsZero() {
		t.Error("Signature Created time is zero")
	}

	// Verify the signature
	err = signer.Verify(pkgPath, sig)
	if err != nil {
		t.Errorf("Verify() error = %v", err)
	}
}

// TestRSASigner_Verify_InvalidSignatureType tests verification with wrong type.
func TestRSASigner_Verify_InvalidSignatureType(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := createTestRSAKey(t, tmpDir)

	signer, err := NewRSASigner(keyPath)
	if err != nil {
		t.Fatalf("NewRSASigner() error = %v", err)
	}

	pkgPath := filepath.Join(tmpDir, "test.gpkg.tar")
	if err := os.WriteFile(pkgPath, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	sig := &Signature{
		Type:  SignatureGPG, // Wrong type
		Data:  []byte("invalid"),
		KeyID: "test",
	}

	err = signer.Verify(pkgPath, sig)
	if err == nil {
		t.Error("Expected error for invalid signature type")
	}
}

// TestRSASigner_Verify_InvalidSignature tests verification with tampered data.
func TestRSASigner_Verify_InvalidSignature(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := createTestRSAKey(t, tmpDir)

	signer, err := NewRSASigner(keyPath)
	if err != nil {
		t.Fatalf("NewRSASigner() error = %v", err)
	}

	pkgPath := filepath.Join(tmpDir, "test.gpkg.tar")
	if err := os.WriteFile(pkgPath, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Sign the package
	sig, err := signer.Sign(pkgPath)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	// Tamper with the signature data
	sig.Data[0] ^= 0xFF

	// Verification should fail
	err = signer.Verify(pkgPath, sig)
	if err == nil {
		t.Error("Expected error for tampered signature")
	}
}

// TestRSASigner_Verify_TamperedPackage tests verification with tampered package.
func TestRSASigner_Verify_TamperedPackage(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := createTestRSAKey(t, tmpDir)

	signer, err := NewRSASigner(keyPath)
	if err != nil {
		t.Fatalf("NewRSASigner() error = %v", err)
	}

	pkgPath := filepath.Join(tmpDir, "test.gpkg.tar")
	if err := os.WriteFile(pkgPath, []byte("original content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Sign the original package
	sig, err := signer.Sign(pkgPath)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	// Tamper with the package content
	if err := os.WriteFile(pkgPath, []byte("tampered content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Verification should fail
	err = signer.Verify(pkgPath, sig)
	if err == nil {
		t.Error("Expected error for tampered package")
	}
}

// TestRSASigner_Verify_FileNotFound tests verification with non-existent file.
func TestRSASigner_Verify_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := createTestRSAKey(t, tmpDir)

	signer, err := NewRSASigner(keyPath)
	if err != nil {
		t.Fatalf("NewRSASigner() error = %v", err)
	}

	sig := &Signature{
		Type:  SignatureRSA,
		Data:  []byte("signature data"),
		KeyID: "test",
	}

	err = signer.Verify("/nonexistent/package.gpkg.tar", sig)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

// TestPackageSignerInterface tests that all signers implement the interface.
func TestPackageSignerInterface(t *testing.T) {
	var _ PackageSigner = &GPGSigner{}
	var _ PackageSigner = &SSHSigner{}

	// RSASigner also implements PackageSigner
	tmpDir := t.TempDir()
	keyPath := createTestRSAKey(t, tmpDir)
	rsaSigner, err := NewRSASigner(keyPath)
	if err != nil {
		t.Fatalf("NewRSASigner() error = %v", err)
	}
	var _ PackageSigner = rsaSigner
}

// createTestRSAKey creates a test RSA private key for testing.
func createTestRSAKey(t *testing.T, dir string) string {
	t.Helper()

	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Encode private key to PEM
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}

	keyPath := filepath.Join(dir, "test_key.pem")
	pemData := pem.EncodeToMemory(block)
	if err := os.WriteFile(keyPath, pemData, 0600); err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	return keyPath
}

// TestGPGSigner_CustomGPGPath tests setting custom GPG path.
func TestGPGSigner_CustomGPGPath(t *testing.T) {
	signer := NewGPGSigner("AABBCCDD")
	signer.GPGPath = "/usr/bin/gpg2"

	if signer.GPGPath != "/usr/bin/gpg2" {
		t.Errorf("GPGPath = %s, want /usr/bin/gpg2", signer.GPGPath)
	}
}

// TestGPGSigner_DisableArmor tests disabling armor output.
func TestGPGSigner_DisableArmor(t *testing.T) {
	signer := NewGPGSigner("AABBCCDD")
	signer.Armor = false

	if signer.Armor {
		t.Error("Armor should be false after setting")
	}
}

// TestSSHSigner_CustomPath tests setting custom ssh-keygen path.
func TestSSHSigner_CustomPath(t *testing.T) {
	signer := NewSSHSigner("/path/to/key")
	signer.SSHKeygenPath = "/usr/bin/ssh-keygen"

	if signer.SSHKeygenPath != "/usr/bin/ssh-keygen" {
		t.Errorf("SSHKeygenPath = %s, want /usr/bin/ssh-keygen", signer.SSHKeygenPath)
	}
}

// TestSSHSigner_Passphrase tests setting passphrase.
func TestSSHSigner_Passphrase(t *testing.T) {
	signer := NewSSHSigner("/path/to/key")
	signer.Passphrase = "secret"

	if signer.Passphrase != "secret" {
		t.Errorf("Passphrase = %s, want secret", signer.Passphrase)
	}
}

// BenchmarkRSASigner_Sign benchmarks RSA signing.
func BenchmarkRSASigner_Sign(b *testing.B) {
	tmpDir := b.TempDir()

	// Generate RSA key
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}
	keyPath := filepath.Join(tmpDir, "test_key.pem")
	_ = os.WriteFile(keyPath, pem.EncodeToMemory(block), 0600)

	signer, _ := NewRSASigner(keyPath)

	// Create test package
	pkgPath := filepath.Join(tmpDir, "test.gpkg.tar")
	_ = os.WriteFile(pkgPath, make([]byte, 1024), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = signer.Sign(pkgPath)
	}
}

// BenchmarkRSASigner_Verify benchmarks RSA verification.
func BenchmarkRSASigner_Verify(b *testing.B) {
	tmpDir := b.TempDir()

	// Generate RSA key
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}
	keyPath := filepath.Join(tmpDir, "test_key.pem")
	_ = os.WriteFile(keyPath, pem.EncodeToMemory(block), 0600)

	signer, _ := NewRSASigner(keyPath)

	// Create test package and sign it
	pkgPath := filepath.Join(tmpDir, "test.gpkg.tar")
	_ = os.WriteFile(pkgPath, make([]byte, 1024), 0644)
	sig, _ := signer.Sign(pkgPath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = signer.Verify(pkgPath, sig)
	}
}
