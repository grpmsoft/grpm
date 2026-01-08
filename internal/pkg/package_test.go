package pkg

import (
	"testing"
)

// ==================== Slot Value Object Tests ====================

// TestNewSlot tests Slot creation
func TestNewSlot(t *testing.T) {
	tests := []struct {
		name            string
		slotName        string
		subslot         string
		expectedName    string
		expectedSubslot string
	}{
		{
			name:            "Slot without subslot",
			slotName:        "0",
			subslot:         "",
			expectedName:    "0",
			expectedSubslot: "",
		},
		{
			name:            "Slot with subslot",
			slotName:        "0",
			subslot:         "1",
			expectedName:    "0",
			expectedSubslot: "1",
		},
		{
			name:            "Custom slot name",
			slotName:        "2.4",
			subslot:         "2.4.1",
			expectedName:    "2.4",
			expectedSubslot: "2.4.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slot := NewSlot(tt.slotName, tt.subslot)

			if slot.Name != tt.expectedName {
				t.Errorf("NewSlot().Name = %q, expected %q", slot.Name, tt.expectedName)
			}

			if slot.Subslot != tt.expectedSubslot {
				t.Errorf("NewSlot().Subslot = %q, expected %q", slot.Subslot, tt.expectedSubslot)
			}
		})
	}
}

// TestSlot_String tests Slot string representation
func TestSlot_String(t *testing.T) {
	tests := []struct {
		name     string
		slot     Slot
		expected string
	}{
		{
			name:     "Slot without subslot",
			slot:     NewSlot("0", ""),
			expected: "0",
		},
		{
			name:     "Slot with subslot",
			slot:     NewSlot("0", "1"),
			expected: "0/1",
		},
		{
			name:     "Complex slot with subslot",
			slot:     NewSlot("2.4", "2.4.1"),
			expected: "2.4/2.4.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.slot.String()
			if result != tt.expected {
				t.Errorf("Slot.String() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

// TestSlot_Equals tests Slot equality (Value Object semantics)
func TestSlot_Equals(t *testing.T) {
	tests := []struct {
		name     string
		slot1    Slot
		slot2    Slot
		expected bool
	}{
		{
			name:     "Equal slots without subslot",
			slot1:    NewSlot("0", ""),
			slot2:    NewSlot("0", ""),
			expected: true,
		},
		{
			name:     "Equal slots with subslot",
			slot1:    NewSlot("0", "1"),
			slot2:    NewSlot("0", "1"),
			expected: true,
		},
		{
			name:     "Different slot names",
			slot1:    NewSlot("0", ""),
			slot2:    NewSlot("1", ""),
			expected: false,
		},
		{
			name:     "Same slot different subslots",
			slot1:    NewSlot("0", "1"),
			slot2:    NewSlot("0", "2"),
			expected: false,
		},
		{
			name:     "One has subslot other doesn't",
			slot1:    NewSlot("0", "1"),
			slot2:    NewSlot("0", ""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.slot1.Equals(tt.slot2)
			if result != tt.expected {
				t.Errorf("Slot(%v).Equals(%v) = %v, expected %v", tt.slot1, tt.slot2, result, tt.expected)
			}

			// Test symmetry: a.Equals(b) == b.Equals(a)
			reverse := tt.slot2.Equals(tt.slot1)
			if result != reverse {
				t.Errorf("Equals() not symmetric: %v.Equals(%v)=%v, but %v.Equals(%v)=%v",
					tt.slot1, tt.slot2, result, tt.slot2, tt.slot1, reverse)
			}
		})
	}
}

// TestSlot_IsCompatibleWith tests slot compatibility logic
func TestSlot_IsCompatibleWith(t *testing.T) {
	tests := []struct {
		name     string
		slot1    Slot
		slot2    Slot
		expected bool
	}{
		{
			name:     "Different slot names always compatible",
			slot1:    NewSlot("0", ""),
			slot2:    NewSlot("1", ""),
			expected: true,
		},
		{
			name:     "Same slot no subslots - compatible",
			slot1:    NewSlot("0", ""),
			slot2:    NewSlot("0", ""),
			expected: true,
		},
		{
			name:     "Same slot one has subslot - compatible",
			slot1:    NewSlot("0", "1"),
			slot2:    NewSlot("0", ""),
			expected: true,
		},
		{
			name:     "Same slot both empty subslot - compatible",
			slot1:    NewSlot("0", ""),
			slot2:    NewSlot("0", ""),
			expected: true,
		},
		{
			name:     "Same slot same subslot - compatible",
			slot1:    NewSlot("0", "1"),
			slot2:    NewSlot("0", "1"),
			expected: true,
		},
		{
			name:     "Same slot different subslots - incompatible",
			slot1:    NewSlot("0", "1"),
			slot2:    NewSlot("0", "2"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.slot1.IsCompatibleWith(tt.slot2)
			if result != tt.expected {
				t.Errorf("Slot(%v).IsCompatibleWith(%v) = %v, expected %v", tt.slot1, tt.slot2, result, tt.expected)
			}

			// Test symmetry: a.IsCompatibleWith(b) == b.IsCompatibleWith(a)
			reverse := tt.slot2.IsCompatibleWith(tt.slot1)
			if result != reverse {
				t.Errorf("IsCompatibleWith() not symmetric: %v.IsCompatibleWith(%v)=%v, but %v.IsCompatibleWith(%v)=%v",
					tt.slot1, tt.slot2, result, tt.slot2, tt.slot1, reverse)
			}
		})
	}
}

// TestParseSlot tests slot parsing from string
func TestParseSlot(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedName    string
		expectedSubslot string
	}{
		{
			name:            "Simple slot",
			input:           "0",
			expectedName:    "0",
			expectedSubslot: "",
		},
		{
			name:            "Slot with subslot",
			input:           "0/1",
			expectedName:    "0",
			expectedSubslot: "1",
		},
		{
			name:            "Complex slot with subslot",
			input:           "2.4/2.4.1",
			expectedName:    "2.4",
			expectedSubslot: "2.4.1",
		},
		{
			name:            "Empty slot",
			input:           "",
			expectedName:    "",
			expectedSubslot: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slot := ParseSlot(tt.input)

			if slot.Name != tt.expectedName {
				t.Errorf("ParseSlot(%q).Name = %q, expected %q", tt.input, slot.Name, tt.expectedName)
			}

			if slot.Subslot != tt.expectedSubslot {
				t.Errorf("ParseSlot(%q).Subslot = %q, expected %q", tt.input, slot.Subslot, tt.expectedSubslot)
			}
		})
	}
}

// ==================== Package Aggregate Root Tests ====================

// TestNewPackage tests Package creation
func TestNewPackage(t *testing.T) {
	tests := []struct {
		name            string
		packageName     string
		version         string
		slotStr         string
		expectedName    string
		expectedVersion string
		expectedSlot    string
	}{
		{
			name:            "Simple package",
			packageName:     "sys-libs/zlib",
			version:         "1.2.13",
			slotStr:         "0",
			expectedName:    "sys-libs/zlib",
			expectedVersion: "1.2.13",
			expectedSlot:    "0",
		},
		{
			name:            "Package with subslot",
			packageName:     "dev-lang/python",
			version:         "3.11.5",
			slotStr:         "3.11/3.11",
			expectedName:    "dev-lang/python",
			expectedVersion: "3.11.5",
			expectedSlot:    "3.11/3.11",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := NewPackage(tt.packageName, tt.version, tt.slotStr)

			if pkg.Name != tt.expectedName {
				t.Errorf("NewPackage().Name = %q, expected %q", pkg.Name, tt.expectedName)
			}

			if pkg.Version != tt.expectedVersion {
				t.Errorf("NewPackage().Version = %q, expected %q", pkg.Version, tt.expectedVersion)
			}

			if pkg.Slot.String() != tt.expectedSlot {
				t.Errorf("NewPackage().Slot = %q, expected %q", pkg.Slot.String(), tt.expectedSlot)
			}

			// Verify maps and slices are initialized
			if pkg.UseFlags == nil {
				t.Error("NewPackage().UseFlags should be initialized")
			}

			if pkg.Deps == nil {
				t.Error("NewPackage().Deps should be initialized")
			}

			if pkg.Provides == nil {
				t.Error("NewPackage().Provides should be initialized")
			}
		})
	}
}

// TestPackage_ID tests Aggregate Root identity
func TestPackage_ID(t *testing.T) {
	tests := []struct {
		name        string
		packageName string
		version     string
		expectedID  string
	}{
		{
			name:        "Standard package",
			packageName: "sys-libs/zlib",
			version:     "1.2.13",
			expectedID:  "sys-libs/zlib-1.2.13",
		},
		{
			name:        "Package with complex version",
			packageName: "dev-lang/python",
			version:     "3.11.5-r1",
			expectedID:  "dev-lang/python-3.11.5-r1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := NewPackage(tt.packageName, tt.version, "0")
			id := pkg.ID()

			if id != tt.expectedID {
				t.Errorf("Package.ID() = %q, expected %q", id, tt.expectedID)
			}
		})
	}
}

// TestPackage_FullName tests canonical package name
func TestPackage_FullName(t *testing.T) {
	pkg := NewPackage("sys-libs/zlib", "1.2.13", "0")
	expected := "sys-libs/zlib-1.2.13"

	if pkg.FullName() != expected {
		t.Errorf("Package.FullName() = %q, expected %q", pkg.FullName(), expected)
	}
}

// TestPackage_AddDependency tests adding dependencies
func TestPackage_AddDependency(t *testing.T) {
	pkg := NewPackage("app-misc/hello", "2.10", "0")

	// Initially no dependencies
	if len(pkg.Deps) != 0 {
		t.Errorf("New package should have 0 dependencies, got %d", len(pkg.Deps))
	}

	// Add first dependency
	dep1 := Constraint{
		Type: ConstraintTypeVersion,
		Name: "sys-libs/zlib",
	}
	pkg.AddDependency(dep1)

	if len(pkg.Deps) != 1 {
		t.Errorf("After adding 1 dependency, got %d dependencies", len(pkg.Deps))
	}

	// Add second dependency
	dep2 := Constraint{
		Type:    ConstraintTypeVersion,
		Name:    "dev-libs/openssl",
		Version: NewMinVersionConstraint("1.1.0"),
	}
	pkg.AddDependency(dep2)

	if len(pkg.Deps) != 2 {
		t.Errorf("After adding 2 dependencies, got %d dependencies", len(pkg.Deps))
	}

	// Verify dependencies are stored
	if pkg.Deps[0].Name != "sys-libs/zlib" {
		t.Errorf("First dependency name = %q, expected \"sys-libs/zlib\"", pkg.Deps[0].Name)
	}

	if pkg.Deps[1].Name != "dev-libs/openssl" {
		t.Errorf("Second dependency name = %q, expected \"dev-libs/openssl\"", pkg.Deps[1].Name)
	}
}

// TestPackage_ConflictsWith tests package conflict detection
func TestPackage_ConflictsWith(t *testing.T) {
	tests := []struct {
		name     string
		pkg1     *Package
		pkg2     *Package
		expected bool
	}{
		{
			name:     "Same package name - no conflict",
			pkg1:     NewPackage("sys-libs/zlib", "1.2.13", "0"),
			pkg2:     NewPackage("sys-libs/zlib", "1.3.0", "0"),
			expected: false,
		},
		{
			name:     "Different packages different slots - no conflict",
			pkg1:     NewPackage("sys-libs/zlib", "1.2.13", "0"),
			pkg2:     NewPackage("dev-libs/openssl", "1.1.1", "0"),
			expected: false,
		},
		{
			name:     "Different packages same slot no subslot - no conflict",
			pkg1:     NewPackage("virtual/libffi", "3.4", "0"),
			pkg2:     NewPackage("dev-libs/libffi", "3.4.4", "0"),
			expected: false,
		},
		{
			name:     "Different packages same slot different subslots - conflict",
			pkg1:     NewPackage("dev-lang/python", "3.11.5", "3.11/3.11"),
			pkg2:     NewPackage("dev-lang/python-exec", "2.4.10", "3.11/3.10"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pkg1.ConflictsWith(tt.pkg2)
			if result != tt.expected {
				t.Errorf("Package(%s).ConflictsWith(%s) = %v, expected %v",
					tt.pkg1.Name, tt.pkg2.Name, result, tt.expected)
			}
		})
	}
}

// TestPackage_SatisfiesConstraint tests constraint satisfaction
func TestPackage_SatisfiesConstraint(t *testing.T) {
	tests := []struct {
		name       string
		pkg        *Package
		constraint Constraint
		expected   bool
	}{
		{
			name: "Package name matches, no version constraint",
			pkg:  NewPackage("sys-libs/zlib", "1.2.13", "0"),
			constraint: Constraint{
				Type: ConstraintTypeVersion,
				Name: "sys-libs/zlib",
			},
			expected: true,
		},
		{
			name: "Package name doesn't match",
			pkg:  NewPackage("sys-libs/zlib", "1.2.13", "0"),
			constraint: Constraint{
				Type: ConstraintTypeVersion,
				Name: "dev-libs/openssl",
			},
			expected: false,
		},
		{
			name: "Package satisfies exact version constraint",
			pkg:  NewPackage("sys-libs/zlib", "1.2.13", "0"),
			constraint: Constraint{
				Type:    ConstraintTypeVersion,
				Name:    "sys-libs/zlib",
				Version: NewExactVersionConstraint("1.2.13"),
			},
			expected: true,
		},
		{
			name: "Package doesn't satisfy exact version constraint",
			pkg:  NewPackage("sys-libs/zlib", "1.2.13", "0"),
			constraint: Constraint{
				Type:    ConstraintTypeVersion,
				Name:    "sys-libs/zlib",
				Version: NewExactVersionConstraint("1.3.0"),
			},
			expected: false,
		},
		{
			name: "Package satisfies >= constraint",
			pkg:  NewPackage("sys-libs/zlib", "1.2.13", "0"),
			constraint: Constraint{
				Type:    ConstraintTypeVersion,
				Name:    "sys-libs/zlib",
				Version: NewMinVersionConstraint("1.2.0"),
			},
			expected: true,
		},
		{
			name: "Package doesn't satisfy >= constraint",
			pkg:  NewPackage("sys-libs/zlib", "1.2.13", "0"),
			constraint: Constraint{
				Type:    ConstraintTypeVersion,
				Name:    "sys-libs/zlib",
				Version: NewMinVersionConstraint("2.0.0"),
			},
			expected: false,
		},
		{
			name: "Package satisfies slot constraint",
			pkg:  NewPackage("dev-lang/python", "3.11.5", "3.11"),
			constraint: Constraint{
				Type: ConstraintTypeSlot,
				Name: "dev-lang/python",
				Slot: "3.11",
			},
			expected: true,
		},
		{
			name: "Package doesn't satisfy slot constraint",
			pkg:  NewPackage("dev-lang/python", "3.11.5", "3.11"),
			constraint: Constraint{
				Type: ConstraintTypeSlot,
				Name: "dev-lang/python",
				Slot: "3.10",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pkg.SatisfiesConstraint(tt.constraint)
			if result != tt.expected {
				t.Errorf("Package(%s).SatisfiesConstraint(%v) = %v, expected %v",
					tt.pkg.Name, tt.constraint, result, tt.expected)
			}
		})
	}
}

// TestPackage_IsCompatibleWith tests package compatibility
func TestPackage_IsCompatibleWith(t *testing.T) {
	tests := []struct {
		name     string
		pkg1     *Package
		pkg2     *Package
		expected bool
	}{
		{
			name:     "Same package different version same slot - compatible",
			pkg1:     NewPackage("sys-libs/zlib", "1.2.13", "0"),
			pkg2:     NewPackage("sys-libs/zlib", "1.3.0", "0"),
			expected: true,
		},
		{
			name:     "Same package different version different slots - compatible",
			pkg1:     NewPackage("dev-lang/python", "3.11.5", "3.11"),
			pkg2:     NewPackage("dev-lang/python", "3.10.13", "3.10"),
			expected: true,
		},
		{
			name:     "Same package different slots with incompatible subslots - incompatible",
			pkg1:     NewPackage("sys-libs/zlib", "1.2.13", "0/1"),
			pkg2:     NewPackage("sys-libs/zlib", "1.3.0", "0/2"),
			expected: false,
		},
		{
			name:     "Different packages no conflict - compatible",
			pkg1:     NewPackage("sys-libs/zlib", "1.2.13", "0"),
			pkg2:     NewPackage("dev-libs/openssl", "1.1.1", "0"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pkg1.IsCompatibleWith(tt.pkg2)
			if result != tt.expected {
				t.Errorf("Package(%s).IsCompatibleWith(%s) = %v, expected %v",
					tt.pkg1.Name, tt.pkg2.Name, result, tt.expected)
			}

			// Test symmetry
			reverse := tt.pkg2.IsCompatibleWith(tt.pkg1)
			if result != reverse {
				t.Errorf("IsCompatibleWith() not symmetric")
			}
		})
	}
}

// TestPackage_HasDependency tests dependency checking
func TestPackage_HasDependency(t *testing.T) {
	pkg := NewPackage("app-misc/hello", "2.10", "0")

	// No dependencies initially
	if pkg.HasDependency("sys-libs/zlib") {
		t.Error("Package should not have zlib dependency initially")
	}

	// Add dependency
	pkg.AddDependency(Constraint{
		Type: ConstraintTypeVersion,
		Name: "sys-libs/zlib",
	})

	// Now should have dependency
	if !pkg.HasDependency("sys-libs/zlib") {
		t.Error("Package should have zlib dependency after adding")
	}

	// Should not have other dependencies
	if pkg.HasDependency("dev-libs/openssl") {
		t.Error("Package should not have openssl dependency")
	}
}

// TestPackage_GetDependenciesByType tests dependency filtering
func TestPackage_GetDependenciesByType(t *testing.T) {
	pkg := NewPackage("app-misc/hello", "2.10", "0")

	// Add different types of dependencies
	pkg.AddDependency(Constraint{
		Type: ConstraintTypeVersion,
		Name: "sys-libs/zlib",
	})

	pkg.AddDependency(Constraint{
		Type: ConstraintTypeSlot,
		Name: "dev-lang/python",
		Slot: "3.11",
	})

	pkg.AddDependency(Constraint{
		Type: ConstraintTypeUseFlag,
		Name: "dev-libs/openssl",
		Flag: "ssl",
	})

	pkg.AddDependency(Constraint{
		Type:    ConstraintTypeVersion,
		Name:    "dev-libs/libffi",
		Version: NewMinVersionConstraint("3.4"),
	})

	// Get version constraints
	versionDeps := pkg.GetDependenciesByType(ConstraintTypeVersion)
	if len(versionDeps) != 2 {
		t.Errorf("Expected 2 version dependencies, got %d", len(versionDeps))
	}

	// Get slot constraints
	slotDeps := pkg.GetDependenciesByType(ConstraintTypeSlot)
	if len(slotDeps) != 1 {
		t.Errorf("Expected 1 slot dependency, got %d", len(slotDeps))
	}

	// Get USE flag constraints
	useFlagDeps := pkg.GetDependenciesByType(ConstraintTypeUseFlag)
	if len(useFlagDeps) != 1 {
		t.Errorf("Expected 1 USE flag dependency, got %d", len(useFlagDeps))
	}
}

// BenchmarkNewPackage benchmarks package creation
func BenchmarkNewPackage(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewPackage("sys-libs/zlib", "1.2.13", "0/1")
	}
}

// BenchmarkPackage_SatisfiesConstraint benchmarks constraint checking
func BenchmarkPackage_SatisfiesConstraint(b *testing.B) {
	pkg := NewPackage("sys-libs/zlib", "1.2.13", "0")
	constraint := Constraint{
		Type:    ConstraintTypeVersion,
		Name:    "sys-libs/zlib",
		Version: NewMinVersionConstraint("1.2.0"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pkg.SatisfiesConstraint(constraint)
	}
}
