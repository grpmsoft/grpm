// Package binpkg implements binary package signing.
package binpkg

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// PackageSigner signs binary packages.
type PackageSigner interface {
	// Sign signs a package file and returns signature
	Sign(packagePath string) (*Signature, error)

	// Verify verifies a package signature
	Verify(packagePath string, signature *Signature) error
}

// GPGSigner signs packages using GPG.
type GPGSigner struct {
	// KeyID is the GPG key ID to use for signing
	KeyID string

	// Passphrase for unlocking private key (optional)
	Passphrase string

	// GPGPath is path to gpg binary (default: "gpg")
	GPGPath string

	// Armor enables ASCII armored output
	Armor bool
}

// NewGPGSigner creates a new GPG signer.
func NewGPGSigner(keyID string) *GPGSigner {
	return &GPGSigner{
		KeyID:   keyID,
		GPGPath: "gpg",
		Armor:   true,
	}
}

// Sign signs a package using GPG.
func (g *GPGSigner) Sign(packagePath string) (*Signature, error) {
	// Check if package exists
	if _, err := os.Stat(packagePath); err != nil {
		return nil, fmt.Errorf("package file not found: %w", err)
	}

	// Build gpg command
	args := []string{
		"--detach-sign",
		"--local-user", g.KeyID,
	}

	if g.Armor {
		args = append(args, "--armor")
	}

	if g.Passphrase != "" {
		args = append(args, "--passphrase", g.Passphrase, "--batch")
	}

	args = append(args, packagePath)

	// Execute gpg
	cmd := exec.Command(g.GPGPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gpg signing failed: %w\nOutput: %s", err, output)
	}

	// Read signature file
	sigPath := packagePath + ".sig"
	if g.Armor {
		sigPath = packagePath + ".asc"
	}

	sigData, err := os.ReadFile(sigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read signature file: %w", err)
	}

	return &Signature{
		Type:    SignatureGPG,
		Data:    sigData,
		KeyID:   g.KeyID,
		Created: time.Now(),
	}, nil
}

// Verify verifies a GPG signature.
func (g *GPGSigner) Verify(packagePath string, signature *Signature) error {
	if signature.Type != SignatureGPG {
		return fmt.Errorf("invalid signature type: expected GPG, got %v", signature.Type)
	}

	// Write signature to temporary file
	sigPath := packagePath + ".sig.tmp"
	if err := os.WriteFile(sigPath, signature.Data, 0644); err != nil {
		return fmt.Errorf("failed to write signature file: %w", err)
	}
	defer func() { _ = os.Remove(sigPath) }()

	// Verify signature
	cmd := exec.Command(g.GPGPath, "--verify", sigPath, packagePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gpg verification failed: %w\nOutput: %s", err, output)
	}

	return nil
}

// SSHSigner signs packages using SSH keys (ed25519).
type SSHSigner struct {
	// KeyPath is path to SSH private key
	KeyPath string

	// Passphrase for encrypted keys (optional)
	Passphrase string

	// SSHKeygenPath is path to ssh-keygen binary (default: "ssh-keygen")
	SSHKeygenPath string
}

// NewSSHSigner creates a new SSH signer.
func NewSSHSigner(keyPath string) *SSHSigner {
	return &SSHSigner{
		KeyPath:       keyPath,
		SSHKeygenPath: "ssh-keygen",
	}
}

// Sign signs a package using SSH key.
func (s *SSHSigner) Sign(packagePath string) (*Signature, error) {
	// Check if package exists
	if _, err := os.Stat(packagePath); err != nil {
		return nil, fmt.Errorf("package file not found: %w", err)
	}

	// Check if key exists
	if _, err := os.Stat(s.KeyPath); err != nil {
		return nil, fmt.Errorf("SSH key not found: %w", err)
	}

	// Build ssh-keygen command
	args := []string{
		"-Y", "sign",
		"-f", s.KeyPath,
		"-n", "file",
		packagePath,
	}

	// Execute ssh-keygen
	cmd := exec.Command(s.SSHKeygenPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("SSH signing failed: %w\nOutput: %s", err, output)
	}

	// Read signature file
	sigPath := packagePath + ".sig"
	sigData, err := os.ReadFile(sigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read signature file: %w", err)
	}

	// Extract key ID from public key
	pubKeyData, err := os.ReadFile(s.KeyPath + ".pub")
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}
	keyID := strings.TrimSpace(string(pubKeyData))

	return &Signature{
		Type:    SignatureSSH,
		Data:    sigData,
		KeyID:   keyID,
		Created: time.Now(),
	}, nil
}

// Verify verifies an SSH signature.
func (s *SSHSigner) Verify(packagePath string, signature *Signature) error {
	if signature.Type != SignatureSSH {
		return fmt.Errorf("invalid signature type: expected SSH, got %v", signature.Type)
	}

	// Write signature to temporary file
	sigPath := packagePath + ".sig.tmp"
	if err := os.WriteFile(sigPath, signature.Data, 0644); err != nil {
		return fmt.Errorf("failed to write signature file: %w", err)
	}
	defer func() { _ = os.Remove(sigPath) }()

	// Create allowed_signers file
	allowedSignersPath := packagePath + ".allowed_signers.tmp"
	allowedSigners := fmt.Sprintf("* %s\n", signature.KeyID)
	if err := os.WriteFile(allowedSignersPath, []byte(allowedSigners), 0644); err != nil {
		return fmt.Errorf("failed to write allowed_signers: %w", err)
	}
	defer func() { _ = os.Remove(allowedSignersPath) }()

	// Verify signature
	cmd := exec.Command(s.SSHKeygenPath,
		"-Y", "verify",
		"-f", allowedSignersPath,
		"-I", "*",
		"-n", "file",
		"-s", sigPath,
		"<", packagePath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("SSH verification failed: %w\nOutput: %s", err, output)
	}

	return nil
}

// RSASigner signs packages using RSA keys.
type RSASigner struct {
	// PrivateKey is the RSA private key
	PrivateKey *rsa.PrivateKey

	// KeyPath is path to RSA private key file (PEM format)
	KeyPath string
}

// NewRSASigner creates a new RSA signer.
func NewRSASigner(keyPath string) (*RSASigner, error) {
	signer := &RSASigner{
		KeyPath: keyPath,
	}

	// Load private key
	if err := signer.loadPrivateKey(); err != nil {
		return nil, err
	}

	return signer, nil
}

// loadPrivateKey loads RSA private key from PEM file.
func (r *RSASigner) loadPrivateKey() error {
	keyData, err := os.ReadFile(r.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	r.PrivateKey = privateKey
	return nil
}

// Sign signs a package using RSA key.
func (r *RSASigner) Sign(packagePath string) (*Signature, error) {
	// Check if package exists
	if _, err := os.Stat(packagePath); err != nil {
		return nil, fmt.Errorf("package file not found: %w", err)
	}

	// Read package file
	packageData, err := os.ReadFile(packagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read package: %w", err)
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256(packageData)

	// Sign hash
	signature, err := rsa.SignPKCS1v15(rand.Reader, r.PrivateKey, crypto.SHA256, hash[:])
	if err != nil {
		return nil, fmt.Errorf("RSA signing failed: %w", err)
	}

	return &Signature{
		Type:    SignatureRSA,
		Data:    signature,
		KeyID:   fmt.Sprintf("rsa:%d", r.PrivateKey.N.BitLen()),
		Created: time.Now(),
	}, nil
}

// Verify verifies an RSA signature.
func (r *RSASigner) Verify(packagePath string, signature *Signature) error {
	if signature.Type != SignatureRSA {
		return fmt.Errorf("invalid signature type: expected RSA, got %v", signature.Type)
	}

	// Read package file
	packageData, err := os.ReadFile(packagePath)
	if err != nil {
		return fmt.Errorf("failed to read package: %w", err)
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256(packageData)

	// Verify signature
	err = rsa.VerifyPKCS1v15(&r.PrivateKey.PublicKey, crypto.SHA256, hash[:], signature.Data)
	if err != nil {
		return fmt.Errorf("RSA verification failed: %w", err)
	}

	return nil
}
