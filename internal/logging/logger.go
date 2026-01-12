// Package logging provides Portage-style logging for GRPM.
//
// Output styles match the familiar Portage format:
//
//	>>> Actions and phases
//	* Informational messages
//	!!! Errors and warnings
//
// Colors are automatically disabled when output is not a terminal.
package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// Level represents logging verbosity levels.
type Level int

const (
	// LevelQuiet suppresses most output.
	LevelQuiet Level = iota
	// LevelNormal shows standard output.
	LevelNormal
	// LevelVerbose shows additional details.
	LevelVerbose
	// LevelDebug shows all debug information.
	LevelDebug
)

// Logger provides Portage-style logging output.
type Logger struct {
	out       io.Writer
	err       io.Writer
	level     Level
	color     bool
	prefix    string
	mu        sync.Mutex
	startTime time.Time
	file      *os.File
}

// ANSI color codes.
const (
	colorReset   = "\033[0m"
	colorBold    = "\033[1m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorWhite   = "\033[37m"
)

// Portage-style prefixes.
const (
	prefixAction  = ">>>" // Actions: emerging, installing, etc.
	prefixInfo    = " * " // Informational messages
	prefixWarn    = " ! " // Warnings
	prefixError   = "!!!" // Errors
	prefixSuccess = " * " // Success (with green)
	prefixDebug   = "   " // Debug (indented)
	prefixPhase   = " * " // Build phases
)

// New creates a new Logger with default settings.
func New() *Logger {
	return &Logger{
		out:       os.Stdout,
		err:       os.Stderr,
		level:     LevelNormal,
		color:     isTerminal(os.Stdout),
		startTime: time.Now(),
	}
}

// isTerminal checks if the writer is a terminal.
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

// SetOutput sets the output writer.
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = w
	l.color = isTerminal(w)
}

// SetErrorOutput sets the error output writer.
func (l *Logger) SetErrorOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.err = w
}

// SetLevel sets the logging verbosity level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetColor enables or disables color output.
func (l *Logger) SetColor(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.color = enabled
}

// SetPrefix sets an optional prefix for all messages.
func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

// SetLogFile enables logging to a file in addition to console.
func (l *Logger) SetLogFile(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	l.file = f
	return nil
}

// Close closes the log file if one was opened.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// colorize applies color if enabled.
func (l *Logger) colorize(color, text string) string {
	if !l.color {
		return text
	}
	return color + text + colorReset
}

// write outputs a formatted message.
func (l *Logger) write(w io.Writer, prefix, color, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)

	// Build the line
	var line string
	if l.prefix != "" {
		line = fmt.Sprintf("%s%s %s\n", l.colorize(color, prefix), l.prefix, msg)
	} else {
		line = fmt.Sprintf("%s %s\n", l.colorize(color, prefix), msg)
	}

	// Write to console (error ignored - logging should not fail)
	_, _ = fmt.Fprint(w, line)

	// Write to file (without colors)
	if l.file != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		plainLine := fmt.Sprintf("[%s] %s %s\n", timestamp, prefix, msg)
		_, _ = fmt.Fprint(l.file, plainLine)
	}
}

// Action logs an action message (>>> style).
// Used for: emerging, installing, syncing, etc.
func (l *Logger) Action(format string, args ...interface{}) {
	l.write(l.out, prefixAction, colorGreen+colorBold, format, args...)
}

// Info logs an informational message (* style).
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level < LevelNormal {
		return
	}
	l.write(l.out, prefixInfo, colorGreen, format, args...)
}

// Warn logs a warning message (! style).
func (l *Logger) Warn(format string, args ...interface{}) {
	l.write(l.err, prefixWarn, colorYellow+colorBold, format, args...)
}

// Error logs an error message (!!! style).
func (l *Logger) Error(format string, args ...interface{}) {
	l.write(l.err, prefixError, colorRed+colorBold, format, args...)
}

// Success logs a success message (* style in green).
func (l *Logger) Success(format string, args ...interface{}) {
	l.write(l.out, prefixSuccess, colorGreen+colorBold, format, args...)
}

// Phase logs a build phase message.
func (l *Logger) Phase(format string, args ...interface{}) {
	if l.level < LevelVerbose {
		return
	}
	l.write(l.out, prefixPhase, colorCyan, format, args...)
}

// Debug logs a debug message.
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level < LevelDebug {
		return
	}
	l.write(l.out, prefixDebug, colorWhite, format, args...)
}

// Verbose logs a verbose message.
func (l *Logger) Verbose(format string, args ...interface{}) {
	if l.level < LevelVerbose {
		return
	}
	l.write(l.out, prefixDebug, colorWhite, format, args...)
}

// Progress logs a progress update (no newline, overwrites previous).
func (l *Logger) Progress(format string, args ...interface{}) {
	if l.level < LevelNormal {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	if l.color {
		// Clear line and print progress
		_, _ = fmt.Fprintf(l.out, "\r\033[K%s %s", l.colorize(colorCyan, "   "), msg)
	} else {
		_, _ = fmt.Fprintf(l.out, "\r%s", msg)
	}
}

// ProgressDone finishes a progress line with a newline.
func (l *Logger) ProgressDone() {
	if l.level < LevelNormal {
		return
	}
	_, _ = fmt.Fprintln(l.out)
}

// Emerge logs package emergence in Portage style.
// Example: ">>> Emerging (1 of 5) app-misc/hello-2.10"
func (l *Logger) Emerge(current, total int, atom string) {
	l.Action("Emerging (%d of %d) %s", current, total, atom)
}

// Installing logs package installation in Portage style.
func (l *Logger) Installing(current, total int, atom string) {
	l.Action("Installing (%d of %d) %s", current, total, atom)
}

// Unmerging logs package removal in Portage style.
func (l *Logger) Unmerging(atom string) {
	l.Action("Unmerging %s", atom)
}

// Syncing logs repository sync start.
func (l *Logger) Syncing(repo string) {
	l.Action("Syncing repository: %s", repo)
}

// SyncProgress logs sync progress with file counts.
func (l *Logger) SyncProgress(filesReceived, totalFiles int) {
	if l.level < LevelVerbose {
		return
	}
	if totalFiles > 0 {
		pct := float64(filesReceived) / float64(totalFiles) * 100
		l.Progress("Receiving files: %d/%d (%.1f%%)", filesReceived, totalFiles, pct)
	} else {
		l.Progress("Receiving files: %d...", filesReceived)
	}
}

// SyncComplete logs successful sync completion.
func (l *Logger) SyncComplete(duration time.Duration, filesChanged int) {
	l.Success("Sync complete in %s (%d files changed)", duration.Round(time.Millisecond), filesChanged)
}

// Mirror logs mirror selection.
func (l *Logger) Mirror(index, total int, host string) {
	l.Info("Trying mirror %d/%d: %s", index, total, host)
}

// MirrorFailed logs mirror failure.
func (l *Logger) MirrorFailed(host string, err error) {
	l.Warn("Mirror %s failed: %v", host, err)
}

// Retry logs a retry attempt.
func (l *Logger) Retry(attempt, max int, delay time.Duration) {
	l.Info("Retry %d/%d after %v...", attempt, max, delay)
}

// Separator logs a visual separator line.
func (l *Logger) Separator() {
	l.mu.Lock()
	defer l.mu.Unlock()

	line := strings.Repeat("-", 70)
	if l.color {
		_, _ = fmt.Fprintln(l.out, colorCyan+line+colorReset)
	} else {
		_, _ = fmt.Fprintln(l.out, line)
	}
}

// Elapsed returns the time since logger creation.
func (l *Logger) Elapsed() time.Duration {
	return time.Since(l.startTime)
}

// Default is the default logger instance.
var Default = New()

// Package-level convenience functions.

// Action logs an action message.
func Action(format string, args ...interface{}) {
	Default.Action(format, args...)
}

// Info logs an informational message.
func Info(format string, args ...interface{}) {
	Default.Info(format, args...)
}

// Warn logs a warning message.
func Warn(format string, args ...interface{}) {
	Default.Warn(format, args...)
}

// Error logs an error message.
func Error(format string, args ...interface{}) {
	Default.Error(format, args...)
}

// Success logs a success message.
func Success(format string, args ...interface{}) {
	Default.Success(format, args...)
}

// Debug logs a debug message.
func Debug(format string, args ...interface{}) {
	Default.Debug(format, args...)
}

// SetLevel sets the default logger's level.
func SetLevel(level Level) {
	Default.SetLevel(level)
}

// SetColor enables/disables colors on default logger.
func SetColor(enabled bool) {
	Default.SetColor(enabled)
}
