package repo

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// Quick focused tests for Specification Pattern

// TestAndSpecification tests AND composition
func TestAndSpecification(t *testing.T) {
	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkg2 := pkg.NewPackage("dev-libs/openssl", "1.1.1", "0")

	spec := NewAndSpecification(
		NewNameSpecification("sys-libs/zlib"),
		NewVersionSpecification(pkg.NewMinVersionConstraint("1.2.0")),
	)

	if !spec.IsSatisfiedBy(pkg1) {
		t.Error("AND spec should match zlib 1.2.13")
	}

	if spec.IsSatisfiedBy(pkg2) {
		t.Error("AND spec should not match openssl")
	}
}

// TestOrSpecification tests OR composition
func TestOrSpecification(t *testing.T) {
	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkg2 := pkg.NewPackage("dev-libs/openssl", "1.1.1", "0")
	pkg3 := pkg.NewPackage("app-misc/hello", "2.10", "0")

	spec := NewOrSpecification(
		NewNameSpecification("sys-libs/zlib"),
		NewNameSpecification("dev-libs/openssl"),
	)

	if !spec.IsSatisfiedBy(pkg1) {
		t.Error("OR spec should match zlib")
	}

	if !spec.IsSatisfiedBy(pkg2) {
		t.Error("OR spec should match openssl")
	}

	if spec.IsSatisfiedBy(pkg3) {
		t.Error("OR spec should not match hello")
	}
}

// TestNotSpecification tests NOT negation
func TestNotSpecification(t *testing.T) {
	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkg2 := pkg.NewPackage("dev-libs/openssl", "1.1.1", "0")

	spec := NewNotSpecification(
		NewNameSpecification("sys-libs/zlib"),
	)

	if spec.IsSatisfiedBy(pkg1) {
		t.Error("NOT spec should not match zlib")
	}

	if !spec.IsSatisfiedBy(pkg2) {
		t.Error("NOT spec should match non-zlib packages")
	}
}

// TestNameSpecification tests name filtering
func TestNameSpecification(t *testing.T) {
	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkg2 := pkg.NewPackage("dev-libs/openssl", "1.1.1", "0")

	spec := NewNameSpecification("sys-libs/zlib")

	if !spec.IsSatisfiedBy(pkg1) {
		t.Error("Name spec should match exact name")
	}

	if spec.IsSatisfiedBy(pkg2) {
		t.Error("Name spec should not match different name")
	}
}

// TestVersionSpecification tests version filtering
func TestVersionSpecification(t *testing.T) {
	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkg2 := pkg.NewPackage("sys-libs/zlib", "1.1.0", "0")

	spec := NewVersionSpecification(pkg.NewMinVersionConstraint("1.2.0"))

	if !spec.IsSatisfiedBy(pkg1) {
		t.Error("Version spec should match 1.2.13 >= 1.2.0")
	}

	if spec.IsSatisfiedBy(pkg2) {
		t.Error("Version spec should not match 1.1.0 >= 1.2.0")
	}
}

// TestSlotSpecification tests slot filtering
func TestSlotSpecification(t *testing.T) {
	pkg1 := pkg.NewPackage("dev-lang/python", "3.11.5", "3.11")
	pkg2 := pkg.NewPackage("dev-lang/python", "3.10.13", "3.10")

	spec := NewSlotSpecification("3.11")

	if !spec.IsSatisfiedBy(pkg1) {
		t.Error("Slot spec should match slot 3.11")
	}

	if spec.IsSatisfiedBy(pkg2) {
		t.Error("Slot spec should not match slot 3.10")
	}
}

// TestCategorySpecification tests category filtering
func TestCategorySpecification(t *testing.T) {
	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkg2 := pkg.NewPackage("dev-libs/openssl", "1.1.1", "0")

	spec := NewCategorySpecification("sys-libs")

	if !spec.IsSatisfiedBy(pkg1) {
		t.Error("Category spec should match sys-libs/zlib")
	}

	if spec.IsSatisfiedBy(pkg2) {
		t.Error("Category spec should not match dev-libs/openssl")
	}
}

// TestComplexSpecificationComposition tests real-world scenario
func TestComplexSpecificationComposition(t *testing.T) {
	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkg2 := pkg.NewPackage("sys-libs/glibc", "2.37", "0")
	pkg3 := pkg.NewPackage("dev-libs/openssl", "1.1.1", "0")

	// Find: sys-libs/* with version >= 1.2.0
	spec := NewAndSpecification(
		NewCategorySpecification("sys-libs"),
		NewVersionSpecification(pkg.NewMinVersionConstraint("1.2.0")),
	)

	if !spec.IsSatisfiedBy(pkg1) {
		t.Error("Complex spec should match zlib")
	}

	if !spec.IsSatisfiedBy(pkg2) {
		t.Error("Complex spec should match glibc")
	}

	if spec.IsSatisfiedBy(pkg3) {
		t.Error("Complex spec should not match openssl (wrong category)")
	}
}
