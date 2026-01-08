package commands

import (
	"github.com/spf13/cobra"
)

// RootCmd is the root command for grpm.
var RootCmd = &cobra.Command{
	Use:   "grpm",
	Short: "Next-generation package manager for Gentoo",
	Long: `GRPM is a modern, high-performance package manager for Gentoo Linux,
written in Go with a focus on reliability, speed, and maintainability.

Features:
  - Advanced dependency resolution with SAT solver
  - Transaction safety with snapshots
  - Binary package support
  - Parallel builds
  - Profile system integration`,
}

func init() {
	// Add subcommands
	RootCmd.AddCommand(resolveCmd)
	RootCmd.AddCommand(installCmd)
}
