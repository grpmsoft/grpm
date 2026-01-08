package state

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// VarDBLoader loads packages from Gentoo VarDB format.
type VarDBLoader struct {
	root string
}

// NewVarDBLoader creates a new VarDB loader.
//
// The root should be /var/db/pkg directory.
func NewVarDBLoader(root string) *VarDBLoader {
	return &VarDBLoader{
		root: root,
	}
}

// parsePkgNameVersion parses package name and version from directory name.
// Example: "hello-2.10" -> ("hello", "2.10")
//
//	"zlib-1.2.13" -> ("zlib", "1.2.13")
func parsePkgNameVersion(pkgDirName string) (name, version string) {
	// Find last hyphen that separates name from version
	// Assumption: version starts with a digit
	lastHyphen := -1
	for i := len(pkgDirName) - 1; i >= 0; i-- {
		if pkgDirName[i] == '-' {
			// Check if next char is a digit
			if i+1 < len(pkgDirName) && pkgDirName[i+1] >= '0' && pkgDirName[i+1] <= '9' {
				lastHyphen = i
				break
			}
		}
	}

	if lastHyphen == -1 {
		// No version found, return whole string as name
		return pkgDirName, ""
	}

	return pkgDirName[:lastHyphen], pkgDirName[lastHyphen+1:]
}

// Load loads all installed packages from VarDB.
//
// This function scans the VarDB directory structure and loads all
// package metadata files.
//
// Example:
//
//	loader := NewVarDBLoader("/var/db/pkg")
//	db := NewPackageDatabase("/var/db/pkg")
//	if err := loader.LoadInto(db); err != nil {
//	    return err
//	}
func (vl *VarDBLoader) LoadInto(db *PackageDatabase) error {
	// Scan /var/db/pkg for category directories
	categories, err := os.ReadDir(vl.root)
	if err != nil {
		return fmt.Errorf("failed to read VarDB root: %w", err)
	}

	for _, category := range categories {
		if !category.IsDir() {
			continue
		}

		categoryPath := filepath.Join(vl.root, category.Name())
		packages, err := os.ReadDir(categoryPath)
		if err != nil {
			continue // Skip invalid categories
		}

		for _, pkgDir := range packages {
			if !pkgDir.IsDir() {
				continue
			}

			pkgPath := filepath.Join(categoryPath, pkgDir.Name())
			installedPkg, err := vl.loadPackage(pkgPath, category.Name(), pkgDir.Name())
			if err != nil {
				// Log error but continue with other packages
				continue
			}

			if err := db.Add(installedPkg); err != nil {
				continue
			}
		}
	}

	return nil
}

// loadPackage loads a single package from VarDB.
func (vl *VarDBLoader) loadPackage(path, category, pkgDirName string) (*InstalledPackage, error) {
	// Parse package name and version from directory name
	// Format: packagename-version (e.g., "hello-2.10", "zlib-1.2.13")
	name, version := parsePkgNameVersion(pkgDirName)

	installedPkg := &InstalledPackage{
		Package: &pkg.Package{
			Name:    fmt.Sprintf("%s/%s", category, name),
			Version: version,
			Slot:    pkg.Slot{Name: "0"}, // Default slot
		},
		Files: make([]InstalledFile, 0),
		USE:   make([]string, 0),
	}

	// Load CONTENTS (file list)
	if err := vl.loadContents(path, installedPkg); err != nil {
		// CONTENTS is required
		return nil, err
	}

	// Load USE flags
	_ = vl.loadUSE(path, installedPkg)

	// Load CFLAGS
	_ = vl.loadCFLAGS(path, installedPkg)

	// Load BUILD_TIME
	_ = vl.loadBuildTime(path, installedPkg)

	// Load SIZE
	_ = vl.loadSize(path, installedPkg)

	// Load EAPI
	_ = vl.loadEAPI(path, installedPkg)

	return installedPkg, nil
}

// loadContents loads the CONTENTS file.
//
// CONTENTS format:
//
//	obj /path/to/file hash mode mtime
//	dir /path/to/dir
//	sym /path/to/link -> target mtime
func (vl *VarDBLoader) loadContents(pkgPath string, installedPkg *InstalledPackage) error {
	contentsPath := filepath.Join(pkgPath, "CONTENTS")
	file, err := os.Open(contentsPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		installedFile, err := parseContentsLine(line)
		if err != nil {
			continue // Skip invalid lines
		}

		installedPkg.Files = append(installedPkg.Files, installedFile)
	}

	return scanner.Err()
}

// parseContentsLine parses a single line from CONTENTS file.
func parseContentsLine(line string) (InstalledFile, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return InstalledFile{}, fmt.Errorf("invalid CONTENTS line")
	}

	fileType := fields[0]
	path := fields[1]

	switch fileType {
	case "obj":
		// obj /path/to/file hash mode mtime
		if len(fields) < 5 {
			return InstalledFile{}, fmt.Errorf("invalid obj line")
		}

		hash := fields[2]
		mode, _ := strconv.ParseUint(fields[3], 8, 32)
		mtime, _ := strconv.ParseInt(fields[4], 10, 64)

		return InstalledFile{
			Path:  path,
			Type:  FileTypeRegular,
			Mode:  uint32(mode),
			Hash:  hash,
			MTime: mtime,
		}, nil

	case "dir":
		// dir /path/to/dir
		return InstalledFile{
			Path: path,
			Type: FileTypeDirectory,
		}, nil

	case "sym":
		// sym /path/to/link -> target mtime
		if len(fields) < 4 {
			return InstalledFile{}, fmt.Errorf("invalid sym line")
		}

		// Find -> separator
		arrowIdx := -1
		for i, f := range fields {
			if f == "->" {
				arrowIdx = i
				break
			}
		}

		if arrowIdx == -1 || arrowIdx+2 > len(fields) {
			return InstalledFile{}, fmt.Errorf("invalid sym line")
		}

		target := fields[arrowIdx+1]
		mtime := int64(0)
		if arrowIdx+2 < len(fields) {
			mtime, _ = strconv.ParseInt(fields[arrowIdx+2], 10, 64)
		}

		return InstalledFile{
			Path:   path,
			Type:   FileTypeSymlink,
			Target: target,
			MTime:  mtime,
		}, nil

	default:
		return InstalledFile{}, fmt.Errorf("unknown file type: %s", fileType)
	}
}

// loadUSE loads the USE file.
func (vl *VarDBLoader) loadUSE(pkgPath string, installedPkg *InstalledPackage) error {
	usePath := filepath.Join(pkgPath, "USE")
	content, err := os.ReadFile(usePath)
	if err != nil {
		return err
	}

	useFlags := strings.Fields(string(content))
	installedPkg.USE = useFlags

	return nil
}

// loadCFLAGS loads the CFLAGS file.
func (vl *VarDBLoader) loadCFLAGS(pkgPath string, installedPkg *InstalledPackage) error {
	cflagsPath := filepath.Join(pkgPath, "CFLAGS")
	content, err := os.ReadFile(cflagsPath)
	if err != nil {
		return err
	}

	installedPkg.CFLAGS = strings.TrimSpace(string(content))
	installedPkg.BuildInfo.CFLAGS = installedPkg.CFLAGS

	// Also load CXXFLAGS
	cxxflagsPath := filepath.Join(pkgPath, "CXXFLAGS")
	if cxxContent, err := os.ReadFile(cxxflagsPath); err == nil {
		installedPkg.CXXFLAGS = strings.TrimSpace(string(cxxContent))
		installedPkg.BuildInfo.CXXFLAGS = installedPkg.CXXFLAGS
	}

	// Also load LDFLAGS
	ldflagsPath := filepath.Join(pkgPath, "LDFLAGS")
	if ldContent, err := os.ReadFile(ldflagsPath); err == nil {
		installedPkg.LDFLAGS = strings.TrimSpace(string(ldContent))
		installedPkg.BuildInfo.LDFLAGS = installedPkg.LDFLAGS
	}

	return nil
}

// loadBuildTime loads the BUILD_TIME file.
func (vl *VarDBLoader) loadBuildTime(pkgPath string, installedPkg *InstalledPackage) error {
	buildTimePath := filepath.Join(pkgPath, "BUILD_TIME")
	content, err := os.ReadFile(buildTimePath)
	if err != nil {
		return err
	}

	timestamp, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return err
	}

	installedPkg.InstallTime = time.Unix(timestamp, 0)
	installedPkg.BuildInfo.BuildDate = installedPkg.InstallTime

	return nil
}

// loadSize loads the SIZE file.
func (vl *VarDBLoader) loadSize(pkgPath string, installedPkg *InstalledPackage) error {
	sizePath := filepath.Join(pkgPath, "SIZE")
	content, err := os.ReadFile(sizePath)
	if err != nil {
		return err
	}

	size, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return err
	}

	installedPkg.Size = size

	return nil
}

// loadEAPI loads the EAPI file.
func (vl *VarDBLoader) loadEAPI(pkgPath string, installedPkg *InstalledPackage) error {
	eapiPath := filepath.Join(pkgPath, "EAPI")
	content, err := os.ReadFile(eapiPath)
	if err != nil {
		return err
	}

	installedPkg.BuildInfo.EAPI = strings.TrimSpace(string(content))

	return nil
}

// VarDBWriter writes packages to Gentoo VarDB format.
type VarDBWriter struct {
	root string
}

// NewVarDBWriter creates a new VarDB writer.
func NewVarDBWriter(root string) *VarDBWriter {
	return &VarDBWriter{
		root: root,
	}
}

// Write writes a package to VarDB.
//
// This creates the directory structure and writes all metadata files:
//   - CONTENTS (file list)
//   - USE (USE flags)
//   - CFLAGS, CXXFLAGS, LDFLAGS
//   - BUILD_TIME (installation timestamp)
//   - SIZE (package size)
//   - EAPI
func (vw *VarDBWriter) Write(installedPkg *InstalledPackage) error {
	if installedPkg == nil || installedPkg.Package == nil {
		return fmt.Errorf("invalid package")
	}

	// Create package directory
	// Extract category from package name (e.g., "sys-libs/zlib" -> category="sys-libs")
	parts := strings.Split(installedPkg.Package.Name, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid package name format: %s", installedPkg.Package.Name)
	}
	category := parts[0]
	pkgName := strings.Join(parts[1:], "/")

	// VarDB format: /var/db/pkg/category/packagename-version/
	pkgDirName := fmt.Sprintf("%s-%s", pkgName, installedPkg.Package.Version)
	pkgDir := filepath.Join(vw.root, category, pkgDirName)
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		return fmt.Errorf("failed to create package directory: %w", err)
	}

	// Write CONTENTS
	if err := vw.writeContents(pkgDir, installedPkg); err != nil {
		return err
	}

	// Write USE
	if err := vw.writeUSE(pkgDir, installedPkg); err != nil {
		return err
	}

	// Write CFLAGS
	if err := vw.writeCFLAGS(pkgDir, installedPkg); err != nil {
		return err
	}

	// Write BUILD_TIME
	if err := vw.writeBuildTime(pkgDir, installedPkg); err != nil {
		return err
	}

	// Write SIZE
	if err := vw.writeSize(pkgDir, installedPkg); err != nil {
		return err
	}

	// Write EAPI
	if err := vw.writeEAPI(pkgDir, installedPkg); err != nil {
		return err
	}

	return nil
}

// writeContents writes the CONTENTS file.
func (vw *VarDBWriter) writeContents(pkgDir string, installedPkg *InstalledPackage) error {
	contentsPath := filepath.Join(pkgDir, "CONTENTS")
	file, err := os.Create(contentsPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	for _, f := range installedPkg.Files {
		line := formatContentsLine(f)
		if _, err := file.WriteString(line + "\n"); err != nil {
			return err
		}
	}

	return nil
}

// formatContentsLine formats an InstalledFile to CONTENTS format.
func formatContentsLine(file InstalledFile) string {
	switch file.Type {
	case FileTypeRegular:
		return fmt.Sprintf("obj %s %s %04o %d", file.Path, file.Hash, file.Mode, file.MTime)
	case FileTypeDirectory:
		return fmt.Sprintf("dir %s", file.Path)
	case FileTypeSymlink:
		return fmt.Sprintf("sym %s -> %s %d", file.Path, file.Target, file.MTime)
	default:
		return ""
	}
}

// writeUSE writes the USE file.
func (vw *VarDBWriter) writeUSE(pkgDir string, installedPkg *InstalledPackage) error {
	usePath := filepath.Join(pkgDir, "USE")
	content := strings.Join(installedPkg.USE, " ")
	return os.WriteFile(usePath, []byte(content), 0644)
}

// writeCFLAGS writes the CFLAGS, CXXFLAGS, LDFLAGS files.
func (vw *VarDBWriter) writeCFLAGS(pkgDir string, installedPkg *InstalledPackage) error {
	if installedPkg.CFLAGS != "" {
		cflagsPath := filepath.Join(pkgDir, "CFLAGS")
		if err := os.WriteFile(cflagsPath, []byte(installedPkg.CFLAGS), 0644); err != nil {
			return err
		}
	}

	if installedPkg.CXXFLAGS != "" {
		cxxflagsPath := filepath.Join(pkgDir, "CXXFLAGS")
		if err := os.WriteFile(cxxflagsPath, []byte(installedPkg.CXXFLAGS), 0644); err != nil {
			return err
		}
	}

	if installedPkg.LDFLAGS != "" {
		ldflagsPath := filepath.Join(pkgDir, "LDFLAGS")
		if err := os.WriteFile(ldflagsPath, []byte(installedPkg.LDFLAGS), 0644); err != nil {
			return err
		}
	}

	return nil
}

// writeBuildTime writes the BUILD_TIME file.
func (vw *VarDBWriter) writeBuildTime(pkgDir string, installedPkg *InstalledPackage) error {
	buildTimePath := filepath.Join(pkgDir, "BUILD_TIME")
	timestamp := fmt.Sprintf("%d", installedPkg.InstallTime.Unix())
	return os.WriteFile(buildTimePath, []byte(timestamp), 0644)
}

// writeSize writes the SIZE file.
func (vw *VarDBWriter) writeSize(pkgDir string, installedPkg *InstalledPackage) error {
	sizePath := filepath.Join(pkgDir, "SIZE")
	size := fmt.Sprintf("%d", installedPkg.Size)
	return os.WriteFile(sizePath, []byte(size), 0644)
}

// writeEAPI writes the EAPI file.
func (vw *VarDBWriter) writeEAPI(pkgDir string, installedPkg *InstalledPackage) error {
	if installedPkg.BuildInfo.EAPI == "" {
		installedPkg.BuildInfo.EAPI = "0" // Default
	}

	eapiPath := filepath.Join(pkgDir, "EAPI")
	return os.WriteFile(eapiPath, []byte(installedPkg.BuildInfo.EAPI), 0644)
}
