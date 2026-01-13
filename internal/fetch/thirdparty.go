package fetch

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ThirdPartyMirrors maps mirror names to their URLs.
//
// Example entries:
//   - "gnu" -> ["https://ftp.gnu.org/gnu/", "https://mirrors.kernel.org/gnu/"]
//   - "sourceforge" -> ["https://downloads.sourceforge.net/", ...]
//
// This allows expansion of mirror:// URLs to real HTTP URLs.
type ThirdPartyMirrors map[string][]string

// ParseThirdPartyMirrors parses the thirdpartymirrors file from a Portage repository.
//
// The file is located at: <repoPath>/profiles/thirdpartymirrors
//
// File format (one entry per line):
//
//	<mirror_name> <url1> [url2] [url3] ...
//
// Example:
//
//	gnu https://ftp.gnu.org/gnu/ https://mirrors.kernel.org/gnu/
//	sourceforge https://downloads.sourceforge.net/
//
// Returns empty map if file doesn't exist or is unreadable.
func ParseThirdPartyMirrors(repoPath string) ThirdPartyMirrors {
	path := filepath.Join(repoPath, "profiles", "thirdpartymirrors")

	file, err := os.Open(path)
	if err != nil {
		return make(ThirdPartyMirrors)
	}
	defer func() { _ = file.Close() }()

	mirrors := make(ThirdPartyMirrors)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		name := fields[0]
		urls := fields[1:]

		// Normalize URLs (ensure trailing slash)
		for i, url := range urls {
			if !strings.HasSuffix(url, "/") {
				urls[i] = url + "/"
			}
		}

		mirrors[name] = urls
	}

	return mirrors
}

// ExpandMirrorURL expands a mirror:// URL to a list of real URLs.
//
// If the URL is not a mirror:// URL, it is returned as-is.
// If the mirror name is unknown, the original URL is returned.
//
// Example:
//
//	mirrors.ExpandMirrorURL("mirror://gnu/hello/hello-2.12.tar.gz")
//	// Returns: [
//	//   "https://ftp.gnu.org/gnu/hello/hello-2.12.tar.gz",
//	//   "https://mirrors.kernel.org/gnu/hello/hello-2.12.tar.gz",
//	// ]
//
//	mirrors.ExpandMirrorURL("https://example.com/file.tar.gz")
//	// Returns: ["https://example.com/file.tar.gz"]
func (m ThirdPartyMirrors) ExpandMirrorURL(url string) []string {
	// Not a mirror:// URL
	if !strings.HasPrefix(url, "mirror://") {
		return []string{url}
	}

	// Parse: mirror://gnu/hello/file.tar.gz -> gnu, hello/file.tar.gz
	rest := strings.TrimPrefix(url, "mirror://")
	parts := strings.SplitN(rest, "/", 2)

	if len(parts) < 2 {
		// Malformed mirror URL
		return []string{url}
	}

	mirrorName := parts[0]
	filePath := parts[1]

	baseURLs, ok := m[mirrorName]
	if !ok || len(baseURLs) == 0 {
		// Unknown mirror, return original URL
		return []string{url}
	}

	// Build expanded URLs
	expanded := make([]string, 0, len(baseURLs))
	for _, baseURL := range baseURLs {
		expanded = append(expanded, baseURL+filePath)
	}

	return expanded
}

// ExpandURIs expands all mirror:// URLs in a list.
//
// Non-mirror URLs are passed through unchanged.
// Mirror URLs are expanded to all available mirror alternatives.
func (m ThirdPartyMirrors) ExpandURIs(uris []string) []string {
	var expanded []string

	for _, uri := range uris {
		expanded = append(expanded, m.ExpandMirrorURL(uri)...)
	}

	return expanded
}

// KnownMirrors returns a list of known mirror names.
func (m ThirdPartyMirrors) KnownMirrors() []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	return names
}
