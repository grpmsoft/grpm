// Package binpkg implements remote binhost uploading.
package binpkg

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// BinhostUploader uploads binary packages to remote binhost.
//
// Supported protocols:
//   - HTTP/HTTPS (via PUT or POST multipart)
//   - SSH/SFTP (via scp or sftp command)
//
// Example HTTP:
//
//	uploader := binpkg.NewHTTPUploader("https://binhost.example.com")
//	uploader.SetAuth("username", "password")
//	err := uploader.Upload(pkg)
//
// Example SSH:
//
//	uploader := binpkg.NewSSHUploader("ssh://user@host:/var/binhost")
//	uploader.SetKeyPath("/home/user/.ssh/id_rsa")
//	err := uploader.Upload(pkg)
type BinhostUploader interface {
	// Upload uploads a binary package
	Upload(pkg *BinaryPackage) error

	// UploadFile uploads a single file
	UploadFile(localPath, remotePath string) error

	// UploadDirectory uploads an entire directory
	UploadDirectory(localDir, remoteDir string) error
}

// UploadOptions configures the upload process.
type UploadOptions struct {
	// ResumeSupport enables resume for interrupted uploads
	ResumeSupport bool

	// ProgressCallback is called with upload progress (0-100)
	ProgressCallback func(percent int)

	// Timeout is the upload timeout
	Timeout time.Duration

	// RetryCount is number of retries on failure
	RetryCount int

	// VerifyChecksum verifies file checksum after upload
	VerifyChecksum bool
}

// HTTPUploader uploads via HTTP/HTTPS.
type HTTPUploader struct {
	// BaseURL is the binhost base URL (e.g., "https://binhost.example.com")
	BaseURL string

	// Auth is HTTP authentication
	Username string
	Password string

	// HTTPClient is the underlying HTTP client
	HTTPClient *http.Client

	// Method is HTTP method (PUT or POST)
	Method string

	// UseMultipart enables multipart/form-data for POST
	UseMultipart bool

	// Verbose enables detailed logging
	Verbose bool
}

// NewHTTPUploader creates a new HTTP uploader.
func NewHTTPUploader(baseURL string) *HTTPUploader {
	return &HTTPUploader{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Minute,
		},
		Method:       "PUT",
		UseMultipart: false,
		Verbose:      false,
	}
}

// SetAuth sets HTTP authentication.
func (u *HTTPUploader) SetAuth(username, password string) {
	u.Username = username
	u.Password = password
}

// SetTimeout sets HTTP timeout.
func (u *HTTPUploader) SetTimeout(timeout time.Duration) {
	u.HTTPClient.Timeout = timeout
}

// Upload uploads a binary package via HTTP.
func (u *HTTPUploader) Upload(pkg *BinaryPackage) error {
	if pkg == nil {
		return fmt.Errorf("package cannot be nil")
	}

	if u.Verbose {
		fmt.Printf("Uploading package: %s-%s\n", pkg.Package.Name, pkg.Package.Version)
	}

	// Determine remote path
	// Format: category/package-version.gpkg.tar
	parts := strings.SplitN(pkg.Package.Name, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid package name format: %s", pkg.Package.Name)
	}
	category := parts[0]
	packageName := parts[1]

	remotePath := fmt.Sprintf("%s/%s-%s%s", category, packageName, pkg.Package.Version, pkg.Format.Extension())

	// Upload package file
	if err := u.UploadFile(pkg.Path, remotePath); err != nil {
		return fmt.Errorf("failed to upload package: %w", err)
	}

	// Upload signature if present
	if pkg.Signature != nil {
		sigPath := pkg.Path + ".sig"
		if _, err := os.Stat(sigPath); err == nil {
			if err := u.UploadFile(sigPath, remotePath+".sig"); err != nil {
				// Non-critical error
				if u.Verbose {
					fmt.Printf("Warning: failed to upload signature: %v\n", err)
				}
			}
		}
	}

	if u.Verbose {
		fmt.Printf("Package uploaded successfully\n")
	}

	return nil
}

// UploadFile uploads a single file via HTTP.
func (u *HTTPUploader) UploadFile(localPath, remotePath string) error {
	// Open local file
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Build URL
	uploadURL, err := url.JoinPath(u.BaseURL, remotePath)
	if err != nil {
		return fmt.Errorf("failed to build URL: %w", err)
	}

	if u.Method == "PUT" {
		return u.uploadFilePUT(uploadURL, file, stat.Size())
	} else if u.Method == "POST" && u.UseMultipart {
		return u.uploadFilePOSTMultipart(uploadURL, localPath, file)
	} else {
		return fmt.Errorf("unsupported upload method: %s", u.Method)
	}
}

// uploadFilePUT uploads via HTTP PUT.
func (u *HTTPUploader) uploadFilePUT(url string, file io.Reader, size int64) error {
	req, err := http.NewRequest("PUT", url, file)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.ContentLength = size

	// Add authentication
	if u.Username != "" {
		req.SetBasicAuth(u.Username, u.Password)
	}

	// Send request
	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// uploadFilePOSTMultipart uploads via HTTP POST multipart/form-data.
func (u *HTTPUploader) uploadFilePOSTMultipart(url, filename string, file io.Reader) error {
	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file field
	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Add authentication
	if u.Username != "" {
		req.SetBasicAuth(u.Username, u.Password)
	}

	// Send request
	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// UploadDirectory uploads an entire directory via HTTP.
func (u *HTTPUploader) UploadDirectory(localDir, remoteDir string) error {
	return filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}

		// Build remote path
		remotePath := filepath.Join(remoteDir, relPath)

		// Upload file
		return u.UploadFile(path, remotePath)
	})
}

// SSHUploader uploads via SSH/SCP.
type SSHUploader struct {
	// Host is the SSH host (e.g., "user@example.com")
	Host string

	// RemoteDir is the remote binhost directory
	RemoteDir string

	// KeyPath is path to SSH private key
	KeyPath string

	// Port is SSH port (default: 22)
	Port int

	// SCPPath is path to scp binary (default: "scp")
	SCPPath string

	// Verbose enables detailed logging
	Verbose bool
}

// NewSSHUploader creates a new SSH uploader.
//
// URL format: ssh://user@host[:port]/path
func NewSSHUploader(sshURL string) (*SSHUploader, error) {
	u, err := url.Parse(sshURL)
	if err != nil {
		return nil, fmt.Errorf("invalid SSH URL: %w", err)
	}

	if u.Scheme != "ssh" {
		return nil, fmt.Errorf("invalid scheme: %s (expected ssh)", u.Scheme)
	}

	port := 22
	if u.Port() != "" {
		_, _ = fmt.Sscanf(u.Port(), "%d", &port)
	}

	return &SSHUploader{
		Host:      u.User.String() + "@" + u.Hostname(),
		RemoteDir: u.Path,
		Port:      port,
		SCPPath:   "scp",
		Verbose:   false,
	}, nil
}

// SetKeyPath sets SSH private key path.
func (u *SSHUploader) SetKeyPath(keyPath string) {
	u.KeyPath = keyPath
}

// Upload uploads a binary package via SSH/SCP.
func (u *SSHUploader) Upload(pkg *BinaryPackage) error {
	if pkg == nil {
		return fmt.Errorf("package cannot be nil")
	}

	if u.Verbose {
		fmt.Printf("Uploading package via SSH: %s-%s\n", pkg.Package.Name, pkg.Package.Version)
	}

	// Determine remote path
	parts := strings.SplitN(pkg.Package.Name, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid package name format: %s", pkg.Package.Name)
	}
	category := parts[0]
	packageName := parts[1]

	remotePath := filepath.Join(u.RemoteDir, category, fmt.Sprintf("%s-%s%s", packageName, pkg.Package.Version, pkg.Format.Extension()))

	// Upload package file
	if err := u.UploadFile(pkg.Path, remotePath); err != nil {
		return fmt.Errorf("failed to upload package: %w", err)
	}

	// Upload signature if present
	if pkg.Signature != nil {
		sigPath := pkg.Path + ".sig"
		if _, err := os.Stat(sigPath); err == nil {
			if err := u.UploadFile(sigPath, remotePath+".sig"); err != nil {
				// Non-critical error
				if u.Verbose {
					fmt.Printf("Warning: failed to upload signature: %v\n", err)
				}
			}
		}
	}

	if u.Verbose {
		fmt.Printf("Package uploaded successfully\n")
	}

	return nil
}

// UploadFile uploads a single file via SCP.
func (u *SSHUploader) UploadFile(localPath, remotePath string) error {
	// Build scp command
	args := []string{}

	// Add SSH key if specified
	if u.KeyPath != "" {
		args = append(args, "-i", u.KeyPath)
	}

	// Add port if not default
	if u.Port != 22 {
		args = append(args, "-P", fmt.Sprintf("%d", u.Port))
	}

	// Add paths
	args = append(args, localPath, fmt.Sprintf("%s:%s", u.Host, remotePath))

	// Execute scp
	cmd := exec.Command(u.SCPPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp failed: %w\nOutput: %s", err, output)
	}

	return nil
}

// UploadDirectory uploads an entire directory via SCP.
func (u *SSHUploader) UploadDirectory(localDir, remoteDir string) error {
	// Build scp command with recursive option
	args := []string{"-r"}

	// Add SSH key if specified
	if u.KeyPath != "" {
		args = append(args, "-i", u.KeyPath)
	}

	// Add port if not default
	if u.Port != 22 {
		args = append(args, "-P", fmt.Sprintf("%d", u.Port))
	}

	// Add paths
	args = append(args, localDir, fmt.Sprintf("%s:%s", u.Host, remoteDir))

	// Execute scp
	cmd := exec.Command(u.SCPPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp failed: %w\nOutput: %s", err, output)
	}

	return nil
}
