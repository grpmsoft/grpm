package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/grpmsoft/grpm/internal/cli"
	"github.com/grpmsoft/grpm/internal/daemon"
)

// Version information (injected via ldflags)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func main() {
	// Mode detection: check first argument
	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		// Run as daemon
		if err := runDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "Daemon error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Run as CLI
		if err := runCLI(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

// runDaemon starts the daemon service
func runDaemon() error {
	log.SetPrefix("[daemon] ")
	log.Printf("GRPM daemon version %s (commit: %s, built: %s)", Version, GitCommit, BuildDate)

	// Load configuration (use defaults for Phase 1)
	config := daemon.DefaultConfig()

	// Create daemon
	d := daemon.New(config)

	// Run daemon (blocks until shutdown signal)
	if err := d.Run(); err != nil {
		return fmt.Errorf("daemon error: %w", err)
	}

	return nil
}

// runCLI runs the CLI application
func runCLI() error {
	// Handle version flag (-V for version, -v reserved for verbose)
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-V") {
		fmt.Printf("grpm version %s\n", Version)
		fmt.Printf("Commit: %s\n", GitCommit)
		fmt.Printf("Built: %s\n", BuildDate)
		return nil
	}

	// Parse verbose level from args (-v, -vv, -vvv or --verbose)
	verboseLevel := 0
	if v := os.Getenv("GRPM_VERBOSE"); v != "" {
		verboseLevel = 1 // Environment variable sets level 1
	}

	// Count -v flags or check --verbose, and filter them from args
	args := os.Args[1:]
	filteredArgs := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "-v":
			verboseLevel = 1
		case "-vv":
			verboseLevel = 2
		case "-vvv":
			verboseLevel = 3
		case "--verbose":
			verboseLevel = 1
		default:
			filteredArgs = append(filteredArgs, arg)
		}
	}

	// Create CLI application
	app := cli.NewApp(&cli.AppConfig{
		Version:      Version,
		Verbose:      verboseLevel > 0,
		VerboseLevel: verboseLevel,
		SocketPath:   os.Getenv("GRPM_SOCKET"), // Allow override via env
	})
	defer func() { _ = app.Close() }()

	// Special handling for daemon subcommands
	if len(filteredArgs) > 0 {
		switch filteredArgs[0] {
		case "status":
			return cmdStatus(app)
		case "version":
			app.PrintVersion()
			return nil
		}
	}

	// Run application with filtered args (verbose flags removed)
	if err := app.Run(filteredArgs); err != nil {
		return err
	}

	return nil
}

// cmdStatus shows daemon status
func cmdStatus(app *cli.App) error {
	// Get daemon info via repository
	repo := daemon.NewDaemonRepository()
	info, err := repo.GetInfo()
	if err != nil {
		return fmt.Errorf("failed to get daemon info: %w", err)
	}

	fmt.Println("📊 GRPM Daemon Status")
	fmt.Println("==========================================")

	// Status icon
	statusIcon := map[daemon.DaemonState]string{
		daemon.StateRunning:  "🟢",
		daemon.StateStopped:  "⚪",
		daemon.StateStarting: "🟡",
		daemon.StateStopping: "🟡",
	}[info.Status]

	fmt.Printf("\n%s Status: %s\n", statusIcon, info.Status.String())

	if info.Status == daemon.StateRunning {
		fmt.Printf("   PID: %d\n", info.PID)
		fmt.Printf("   Socket: %s\n", app.GetClient().GetSocketPath())

		if !info.StartTime.IsZero() {
			fmt.Printf("   Started: %s\n", info.StartTime.Format("2006-01-02 15:04:05"))
			fmt.Printf("   Uptime: %v\n", info.Uptime.Round(1*time.Second))
		}

		// Try to ping
		if err := app.GetClient().Ping(); err != nil {
			fmt.Printf("\n⚠️  Warning: daemon not responsive: %v\n", err)
		} else {
			fmt.Println("\n✅ Status: healthy")
		}

		// Show PID file location
		fmt.Printf("\n📝 PID file: %s\n", repo.GetPIDFile())
	} else {
		fmt.Println("\n💡 Daemon is not running")
		fmt.Println("   Use 'grpm daemon' to start the daemon")
	}

	return nil
}
