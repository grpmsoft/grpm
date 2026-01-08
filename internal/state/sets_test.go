package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
)

// Ensure mockRepo implements repo.Repository interface (compile-time check)
var _ repo.Repository = (*mockRepo)(nil)

func TestNewPackageSet(t *testing.T) {
	tests := []struct {
		name       string
		setName    SetName
		atoms      []string
		wantLen    int
		wantAtoms  []string
		wantUnique bool
	}{
		{
			name:      "empty set",
			setName:   SetSelected,
			atoms:     []string{},
			wantLen:   0,
			wantAtoms: []string{},
		},
		{
			name:      "single atom",
			setName:   SetSelected,
			atoms:     []string{"app-misc/hello"},
			wantLen:   1,
			wantAtoms: []string{"app-misc/hello"},
		},
		{
			name:      "multiple atoms",
			setName:   SetWorld,
			atoms:     []string{"sys-libs/zlib", "app-misc/hello", "dev-lang/go"},
			wantLen:   3,
			wantAtoms: []string{"app-misc/hello", "dev-lang/go", "sys-libs/zlib"}, // sorted
		},
		{
			name:      "duplicates removed",
			setName:   SetSystem,
			atoms:     []string{"sys-libs/zlib", "app-misc/hello", "sys-libs/zlib"},
			wantLen:   2,
			wantAtoms: []string{"app-misc/hello", "sys-libs/zlib"},
		},
		{
			name:      "whitespace trimmed",
			setName:   SetSelected,
			atoms:     []string{"  sys-libs/zlib  ", " app-misc/hello ", ""},
			wantLen:   2,
			wantAtoms: []string{"app-misc/hello", "sys-libs/zlib"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := NewPackageSet(tt.setName, tt.atoms)

			if set.Name() != tt.setName {
				t.Errorf("Name() = %v, want %v", set.Name(), tt.setName)
			}

			if set.Len() != tt.wantLen {
				t.Errorf("Len() = %v, want %v", set.Len(), tt.wantLen)
			}

			atoms := set.Atoms()
			if len(atoms) != len(tt.wantAtoms) {
				t.Errorf("Atoms() len = %v, want %v", len(atoms), len(tt.wantAtoms))
				return
			}

			for i, atom := range atoms {
				if atom != tt.wantAtoms[i] {
					t.Errorf("Atoms()[%d] = %v, want %v", i, atom, tt.wantAtoms[i])
				}
			}
		})
	}
}

func TestPackageSet_Contains(t *testing.T) {
	set := NewPackageSet(SetWorld, []string{
		"sys-libs/zlib",
		"app-misc/hello",
		"dev-lang/go",
	})

	tests := []struct {
		atom string
		want bool
	}{
		{"sys-libs/zlib", true},
		{"app-misc/hello", true},
		{"dev-lang/go", true},
		{"app-editors/vim", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.atom, func(t *testing.T) {
			if got := set.Contains(tt.atom); got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.atom, got, tt.want)
			}
		})
	}
}

func TestPackageSet_Union(t *testing.T) {
	setA := NewPackageSet(SetSelected, []string{
		"app-misc/hello",
		"sys-libs/zlib",
	})

	setB := NewPackageSet(SetSystem, []string{
		"sys-apps/baselayout",
		"sys-libs/zlib", // duplicate
		"virtual/libc",
	})

	union := setA.Union(setB)

	// Should have 4 unique atoms
	if union.Len() != 4 {
		t.Errorf("Union().Len() = %v, want 4", union.Len())
	}

	// Check all atoms are present
	expectedAtoms := []string{
		"app-misc/hello",
		"sys-apps/baselayout",
		"sys-libs/zlib",
		"virtual/libc",
	}

	for _, atom := range expectedAtoms {
		if !union.Contains(atom) {
			t.Errorf("Union should contain %q", atom)
		}
	}
}

func TestParseWorldFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
		wantErr bool
	}{
		{
			name:    "empty file",
			content: "",
			want:    []string{},
		},
		{
			name:    "single package",
			content: "app-misc/hello\n",
			want:    []string{"app-misc/hello"},
		},
		{
			name: "multiple packages",
			content: `app-misc/hello
sys-libs/zlib
dev-lang/go
`,
			want: []string{"app-misc/hello", "sys-libs/zlib", "dev-lang/go"},
		},
		{
			name: "with comments",
			content: `# This is a comment
app-misc/hello
# Another comment
sys-libs/zlib
`,
			want: []string{"app-misc/hello", "sys-libs/zlib"},
		},
		{
			name: "with empty lines",
			content: `app-misc/hello

sys-libs/zlib

dev-lang/go
`,
			want: []string{"app-misc/hello", "sys-libs/zlib", "dev-lang/go"},
		},
		{
			name: "with whitespace",
			content: `  app-misc/hello
	sys-libs/zlib
`,
			want: []string{"app-misc/hello", "sys-libs/zlib"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			worldPath := filepath.Join(tmpDir, "world")

			if err := os.WriteFile(worldPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			got, err := parseWorldFile(worldPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseWorldFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("parseWorldFile() = %v, want %v", got, tt.want)
				return
			}

			for i, atom := range got {
				if atom != tt.want[i] {
					t.Errorf("parseWorldFile()[%d] = %v, want %v", i, atom, tt.want[i])
				}
			}
		})
	}
}

func TestParseWorldFile_NotExists(t *testing.T) {
	atoms, err := parseWorldFile("/nonexistent/world")
	if err != nil {
		t.Errorf("parseWorldFile() should return empty list for non-existent file, got error: %v", err)
	}
	if len(atoms) != 0 {
		t.Errorf("parseWorldFile() = %v, want empty slice", atoms)
	}
}

func TestExtractAtom(t *testing.T) {
	tests := []struct {
		fullName string
		want     string
	}{
		{"sys-libs/zlib-1.2.13", "sys-libs/zlib"},
		{"app-misc/hello-2.10", "app-misc/hello"},
		{"dev-lang/go-1.21.0", "dev-lang/go"},
		{"sys-libs/glibc-2.38-r1", "sys-libs/glibc"},
		{"app-editors/vim-9.0.1000_alpha", "app-editors/vim"},
		{"sys-libs/zlib", "sys-libs/zlib"}, // no version
		{"category/pkg-name-1.0", "category/pkg-name"},
	}

	for _, tt := range tests {
		t.Run(tt.fullName, func(t *testing.T) {
			if got := extractAtom(tt.fullName); got != tt.want {
				t.Errorf("extractAtom(%q) = %v, want %v", tt.fullName, got, tt.want)
			}
		})
	}
}

func TestSetManager_GetSelected(t *testing.T) {
	tmpDir := t.TempDir()
	worldPath := filepath.Join(tmpDir, "world")

	// Write test world file
	content := `app-misc/hello
sys-libs/zlib
dev-lang/go
`
	if err := os.WriteFile(worldPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	manager := NewSetManager(tmpDir, "")
	selected, err := manager.GetSelected()
	if err != nil {
		t.Fatalf("GetSelected() error = %v", err)
	}

	if selected.Name() != SetSelected {
		t.Errorf("Name() = %v, want %v", selected.Name(), SetSelected)
	}

	if selected.Len() != 3 {
		t.Errorf("Len() = %v, want 3", selected.Len())
	}

	expectedAtoms := []string{"app-misc/hello", "dev-lang/go", "sys-libs/zlib"}
	for _, atom := range expectedAtoms {
		if !selected.Contains(atom) {
			t.Errorf("selected should contain %q", atom)
		}
	}
}

func TestSetManager_WriteWorld(t *testing.T) {
	tmpDir := t.TempDir()

	manager := NewSetManager(tmpDir, "")

	// Create a set to write
	set := NewPackageSet(SetSelected, []string{
		"app-misc/hello",
		"sys-libs/zlib",
		"dev-lang/go",
	})

	// Write the set
	if err := manager.WriteWorld(set); err != nil {
		t.Fatalf("WriteWorld() error = %v", err)
	}

	// Read back and verify
	selected, err := manager.GetSelected()
	if err != nil {
		t.Fatalf("GetSelected() error = %v", err)
	}

	if selected.Len() != 3 {
		t.Errorf("Len() = %v, want 3", selected.Len())
	}

	// Atoms should be sorted
	atoms := selected.Atoms()
	expectedOrder := []string{"app-misc/hello", "dev-lang/go", "sys-libs/zlib"}
	for i, atom := range atoms {
		if atom != expectedOrder[i] {
			t.Errorf("Atoms()[%d] = %v, want %v", i, atom, expectedOrder[i])
		}
	}
}

func TestSetManager_AddToWorld(t *testing.T) {
	tmpDir := t.TempDir()

	manager := NewSetManager(tmpDir, "")

	// Add first package
	if err := manager.AddToWorld("app-misc/hello"); err != nil {
		t.Fatalf("AddToWorld() error = %v", err)
	}

	// Add second package
	if err := manager.AddToWorld("sys-libs/zlib"); err != nil {
		t.Fatalf("AddToWorld() error = %v", err)
	}

	// Try to add duplicate
	if err := manager.AddToWorld("app-misc/hello"); err != nil {
		t.Fatalf("AddToWorld() error = %v", err)
	}

	// Verify
	selected, err := manager.GetSelected()
	if err != nil {
		t.Fatalf("GetSelected() error = %v", err)
	}

	if selected.Len() != 2 {
		t.Errorf("Len() = %v, want 2", selected.Len())
	}
}

func TestSetManager_RemoveFromWorld(t *testing.T) {
	tmpDir := t.TempDir()

	manager := NewSetManager(tmpDir, "")

	// Setup initial world file
	set := NewPackageSet(SetSelected, []string{
		"app-misc/hello",
		"sys-libs/zlib",
		"dev-lang/go",
	})
	if err := manager.WriteWorld(set); err != nil {
		t.Fatalf("WriteWorld() error = %v", err)
	}

	// Remove a package
	if err := manager.RemoveFromWorld("sys-libs/zlib"); err != nil {
		t.Fatalf("RemoveFromWorld() error = %v", err)
	}

	// Verify
	selected, err := manager.GetSelected()
	if err != nil {
		t.Fatalf("GetSelected() error = %v", err)
	}

	if selected.Len() != 2 {
		t.Errorf("Len() = %v, want 2", selected.Len())
	}

	if selected.Contains("sys-libs/zlib") {
		t.Error("should not contain removed package")
	}
}

func TestUpdateInfo_String(t *testing.T) {
	tests := []struct {
		name string
		info *UpdateInfo
		want string
	}{
		{
			name: "new package",
			info: &UpdateInfo{
				Atom:             "app-misc/hello",
				AvailableVersion: "2.10",
				IsNew:            true,
			},
			want: "[ebuild     N ] app-misc/hello-2.10",
		},
		{
			name: "upgrade",
			info: &UpdateInfo{
				Atom:             "sys-libs/zlib",
				InstalledVersion: "1.2.12",
				AvailableVersion: "1.2.13",
				IsUpgrade:        true,
			},
			want: "[ebuild     U ] sys-libs/zlib-1.2.13 [1.2.12]",
		},
		{
			name: "downgrade",
			info: &UpdateInfo{
				Atom:             "dev-lang/go",
				InstalledVersion: "1.22.0",
				AvailableVersion: "1.21.0",
				IsDowngrade:      true,
			},
			want: "[ebuild     UD ] dev-lang/go-1.21.0 [1.22.0]",
		},
		{
			name: "use change",
			info: &UpdateInfo{
				Atom:             "app-editors/vim",
				InstalledVersion: "9.0",
				AvailableVersion: "9.0",
				UseChanged:       true,
			},
			want: "[ebuild     U ] app-editors/vim-9.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSlicesEqual(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{"both empty", []string{}, []string{}, true},
		{"both nil", nil, nil, true},
		{"equal", []string{"a", "b"}, []string{"a", "b"}, true},
		{"different order", []string{"a", "b"}, []string{"b", "a"}, false},
		{"different length", []string{"a", "b"}, []string{"a"}, false},
		{"different content", []string{"a", "b"}, []string{"a", "c"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slicesEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("slicesEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetUSEFlags(t *testing.T) {
	tests := []struct {
		name string
		pkg  *pkg.Package
		want []string
	}{
		{
			name: "nil package",
			pkg:  nil,
			want: []string{},
		},
		{
			name: "no use flags",
			pkg:  &pkg.Package{},
			want: []string{},
		},
		{
			name: "with use flags",
			pkg: &pkg.Package{
				UseFlags: map[string]bool{
					"ssl":     true,
					"gtk":     false,
					"unicode": true,
				},
			},
			want: []string{"-gtk", "ssl", "unicode"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getUSEFlags(tt.pkg)
			if len(got) != len(tt.want) {
				t.Errorf("getUSEFlags() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("getUSEFlags()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// mockRepo is a test repository implementation that implements repo.Repository.
type mockRepo struct {
	packages map[string]*pkg.Package
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		packages: make(map[string]*pkg.Package),
	}
}

func (r *mockRepo) AddPackage(name, version string) {
	r.packages[name] = &pkg.Package{
		Name:    name,
		Version: version,
		Slot:    pkg.Slot{Name: "0"},
	}
}

func (r *mockRepo) LoadPackage(name string) (*pkg.Package, error) {
	// Try exact match first
	if p, ok := r.packages[name]; ok {
		return p, nil
	}
	// Try without version
	for pkgName, p := range r.packages {
		if extractAtom(pkgName) == name || pkgName == name {
			return p, nil
		}
	}
	return nil, repo.ErrPackageNotFound
}

func (r *mockRepo) LoadPackages(names []string) ([]*pkg.Package, error) {
	var result []*pkg.Package
	for _, name := range names {
		if p, err := r.LoadPackage(name); err == nil {
			result = append(result, p)
		}
	}
	return result, nil
}

func (r *mockRepo) FindBySpecification(_ repo.Specification) ([]*pkg.Package, error) {
	var result []*pkg.Package
	for _, p := range r.packages {
		result = append(result, p)
	}
	return result, nil
}

func (r *mockRepo) GetAllVersions(packageName string) ([]*pkg.Package, error) {
	var result []*pkg.Package
	for name, p := range r.packages {
		if extractAtom(name) == packageName || name == packageName {
			result = append(result, p)
		}
	}
	return result, nil
}

func (r *mockRepo) Exists(name string) bool {
	_, err := r.LoadPackage(name)
	return err == nil
}

func (r *mockRepo) Count() (int, error) {
	return len(r.packages), nil
}

func TestSetManager_CalculateUpdates(t *testing.T) {
	// Create mock repository
	mockRepository := newMockRepo()
	mockRepository.AddPackage("app-misc/hello", "2.11")
	mockRepository.AddPackage("sys-libs/zlib", "1.2.14")
	mockRepository.AddPackage("dev-lang/go", "1.21.0")

	// Create mock database with installed packages
	db := NewPackageDatabase("/var/db/pkg")
	_ = db.Add(&InstalledPackage{
		Package: &pkg.Package{
			Name:    "sys-libs/zlib",
			Version: "1.2.13",
		},
	})
	_ = db.Add(&InstalledPackage{
		Package: &pkg.Package{
			Name:    "app-misc/hello",
			Version: "2.11",
		},
	})

	// Create set manager
	tmpDir := t.TempDir()
	manager := NewSetManager(tmpDir, "")
	manager.SetDatabase(db)

	// Create test set
	set := NewPackageSet(SetSelected, []string{
		"app-misc/hello",
		"sys-libs/zlib",
		"dev-lang/go",
	})

	// Calculate updates
	opts := DefaultUpdateOptions()
	plan, err := manager.CalculateUpdates(set, mockRepository, opts)
	if err != nil {
		t.Fatalf("CalculateUpdates() error = %v", err)
	}

	// Should have updates
	if plan == nil {
		t.Fatal("plan should not be nil")
	}

	// zlib should be an upgrade (1.2.13 -> 1.2.14)
	// go should be new (not installed)
	// hello should not need update (same version)
	foundUpgrade := false
	foundNew := false

	for _, update := range plan.Updates {
		if update.Atom == "sys-libs/zlib" && update.IsUpgrade {
			foundUpgrade = true
		}
		if update.Atom == "dev-lang/go" && update.IsNew {
			foundNew = true
		}
	}

	if !foundUpgrade {
		t.Error("should find upgrade for sys-libs/zlib")
	}

	if !foundNew {
		t.Error("should find new package dev-lang/go")
	}
}

func TestDefaultUpdateOptions(t *testing.T) {
	opts := DefaultUpdateOptions()

	if opts == nil {
		t.Fatal("DefaultUpdateOptions() should not return nil")
	}

	if !opts.Update {
		t.Error("Update should be true by default")
	}

	if opts.Deep {
		t.Error("Deep should be false by default")
	}

	if opts.NewUse {
		t.Error("NewUse should be false by default")
	}

	if opts.ChangedUse {
		t.Error("ChangedUse should be false by default")
	}
}
