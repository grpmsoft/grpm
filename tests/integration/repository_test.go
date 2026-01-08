package integration

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
)

// TestRepository_Integration tests Repository interface with MockRepository
func TestRepository_Integration(t *testing.T) {
	mockRepo := repo.NewMockRepository()

	// Test Count
	count, err := mockRepo.Count()
	if err != nil {
		t.Errorf("Count() error: %v", err)
	}
	if count != 3 {
		t.Errorf("Count() = %d, expected 3", count)
	}

	// Test FindBySpecification - find sys-libs packages
	spec := repo.NewCategorySpecification("sys-libs")
	packages, err := mockRepo.FindBySpecification(spec)
	if err != nil {
		t.Errorf("FindBySpecification() error: %v", err)
	}
	if len(packages) != 1 {
		t.Errorf("FindBySpecification(sys-libs) returned %d packages, expected 1", len(packages))
	}

	// Test FindBySpecification with version constraint
	versionSpec := repo.NewAndSpecification(
		repo.NewNameSpecification("sys-libs/zlib"),
		repo.NewVersionSpecification(pkg.NewMinVersionConstraint("1.2.0")),
	)
	packages, err = mockRepo.FindBySpecification(versionSpec)
	if err != nil {
		t.Errorf("FindBySpecification() with version error: %v", err)
	}
	if len(packages) != 1 {
		t.Errorf("FindBySpecification(zlib>=1.2.0) returned %d packages, expected 1", len(packages))
	}
}

// TestDependencyService_Integration tests DependencyService with MockRepository
func TestDependencyService_Integration(t *testing.T) {
	mockRepo := repo.NewMockRepository()
	service := pkg.NewDependencyService()

	// Load hello package (depends on zlib)
	hello, err := mockRepo.LoadPackage("app-misc/hello")
	if err != nil {
		t.Fatalf("LoadPackage(hello) error: %v", err)
	}

	// Resolve dependency tree
	packageLoader := func(name string) (*pkg.Package, error) {
		return mockRepo.LoadPackage(name)
	}

	allPackages, err := service.ResolveDependencyTree(hello, packageLoader)
	if err != nil {
		t.Errorf("ResolveDependencyTree() error: %v", err)
	}

	// Should contain hello and zlib (2 packages)
	if len(allPackages) != 2 {
		t.Errorf("ResolveDependencyTree() returned %d packages, expected 2", len(allPackages))
	}

	// Verify zlib is included
	if _, exists := allPackages["sys-libs/zlib"]; !exists {
		t.Error("ResolveDependencyTree() missing dependency sys-libs/zlib")
	}
}

// TestSpecificationComposition_Integration tests complex specification queries
func TestSpecificationComposition_Integration(t *testing.T) {
	mockRepo := repo.NewMockRepository()

	// Complex query: (category=sys-libs OR category=app-misc) AND version>=1.0
	spec := repo.NewAndSpecification(
		repo.NewOrSpecification(
			repo.NewCategorySpecification("sys-libs"),
			repo.NewCategorySpecification("app-misc"),
		),
		repo.NewVersionSpecification(pkg.NewMinVersionConstraint("1.0")),
	)

	packages, err := mockRepo.FindBySpecification(spec)
	if err != nil {
		t.Errorf("FindBySpecification() complex query error: %v", err)
	}

	// Should find both zlib (sys-libs) and hello (app-misc)
	if len(packages) != 2 {
		t.Errorf("Complex specification returned %d packages, expected 2", len(packages))
	}
}
