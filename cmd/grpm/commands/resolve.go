package commands

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/solver"
	"github.com/spf13/cobra"
)

var (
	repoPath    string
	useMockRepo bool
)

var resolveCmd = &cobra.Command{
	Use:   "resolve [package...]",
	Short: "Resolve package dependencies",
	Long: `Resolve package dependencies and display the installation plan.

This command analyzes package dependencies and determines which versions
should be installed to satisfy all requirements.

Example:
  grpm resolve sys-libs/zlib
  grpm resolve --mock app-editors/vim`,
	Args: cobra.MinimumNArgs(1),
	Run:  runResolve,
}

func init() {
	resolveCmd.Flags().StringVar(&repoPath, "repo", "/var/db/repos/gentoo", "Path to Portage repository")
	resolveCmd.Flags().BoolVar(&useMockRepo, "mock", false, "Use mock repository for testing")
}

func runResolve(cmd *cobra.Command, args []string) {
	var r repo.Repository
	var err error

	if !useMockRepo {
		// Convert path to absolute only for real repository
		absRepoPath, absErr := filepath.Abs(repoPath)
		if absErr != nil {
			log.Fatalf("Invalid repository path: %v", absErr)
		}
		log.Printf("Using repository: %s", absRepoPath)

		r, err = repo.NewPortageRepository(absRepoPath)
		if err != nil {
			log.Fatalf("Repository error: %v", err)
		}
	} else {
		log.Printf("Using mock repository")
		r = repo.NewMockRepository()
	}

	resolver := solver.NewResolver(r)
	solution, err := resolver.Resolve(args)
	if err != nil {
		log.Fatalf("Resolution failed: %v", err)
	}

	if len(solution) == 0 {
		log.Println("No packages found in solution")
		return
	}

	fmt.Println("Dependency solution:")
	for name, pkg := range solution {
		fmt.Printf("- %s-%s [slot:%s]\n", name, pkg.Version, pkg.Slot.Name)
	}
}
