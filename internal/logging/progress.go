// Package logging provides progress indicators for long-running operations.
package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Spinner provides an animated progress indicator.
type Spinner struct {
	out      io.Writer
	frames   []string
	current  int
	message  string
	running  bool
	done     chan struct{}
	mu       sync.Mutex
	color    bool
	interval time.Duration
}

// SpinnerFrames defines available spinner styles.
var SpinnerFrames = map[string][]string{
	"dots":    {"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	"line":    {"-", "\\", "|", "/"},
	"arrow":   {"←", "↖", "↑", "↗", "→", "↘", "↓", "↙"},
	"simple":  {".", "..", "...", "...."},
	"braille": {"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
	"bounce":  {"⠁", "⠂", "⠄", "⡀", "⢀", "⠠", "⠐", "⠈"},
}

// NewSpinner creates a new spinner with the specified style.
func NewSpinner(style string) *Spinner {
	frames := SpinnerFrames["dots"]
	if f, ok := SpinnerFrames[style]; ok {
		frames = f
	}

	return &Spinner{
		out:      os.Stdout,
		frames:   frames,
		color:    isTerminal(os.Stdout),
		interval: 80 * time.Millisecond,
	}
}

// SetOutput sets the output writer.
func (s *Spinner) SetOutput(w io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.out = w
	s.color = isTerminal(w)
}

// SetInterval sets the animation interval.
func (s *Spinner) SetInterval(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interval = d
}

// Start begins the spinner animation with a message.
func (s *Spinner) Start(message string) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.message = message
	s.running = true
	s.done = make(chan struct{})
	s.mu.Unlock()

	go s.animate()
}

// animate runs the spinner animation loop.
func (s *Spinner) animate() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			frame := s.frames[s.current]
			msg := s.message
			s.current = (s.current + 1) % len(s.frames)
			color := s.color
			s.mu.Unlock()

			if color {
				_, _ = fmt.Fprintf(s.out, "\r\033[K%s%s%s %s", colorCyan, frame, colorReset, msg)
			} else {
				_, _ = fmt.Fprintf(s.out, "\r%s %s", frame, msg)
			}
		}
	}
}

// Update changes the spinner message.
func (s *Spinner) Update(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = message
}

// Stop stops the spinner and optionally prints a final message.
func (s *Spinner) Stop(message string) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.done)
	color := s.color
	s.mu.Unlock()

	// Clear line and print final message
	if message != "" {
		if color {
			_, _ = fmt.Fprintf(s.out, "\r\033[K%s✓%s %s\n", colorGreen, colorReset, message)
		} else {
			_, _ = fmt.Fprintf(s.out, "\r* %s\n", message)
		}
	} else {
		_, _ = fmt.Fprintf(s.out, "\r\033[K")
	}
}

// StopFail stops the spinner with a failure indicator.
func (s *Spinner) StopFail(message string) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.done)
	color := s.color
	s.mu.Unlock()

	if message != "" {
		if color {
			_, _ = fmt.Fprintf(s.out, "\r\033[K%s✗%s %s\n", colorRed, colorReset, message)
		} else {
			_, _ = fmt.Fprintf(s.out, "\r!!! %s\n", message)
		}
	} else {
		_, _ = fmt.Fprintf(s.out, "\r\033[K")
	}
}

// ProgressBar provides a progress bar with percentage.
type ProgressBar struct {
	out       io.Writer
	total     int
	current   int
	width     int
	message   string
	color     bool
	startTime time.Time
	mu        sync.Mutex
}

// NewProgressBar creates a new progress bar.
func NewProgressBar(total int) *ProgressBar {
	return &ProgressBar{
		out:       os.Stdout,
		total:     total,
		width:     40,
		color:     isTerminal(os.Stdout),
		startTime: time.Now(),
	}
}

// SetOutput sets the output writer.
func (p *ProgressBar) SetOutput(w io.Writer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.out = w
	p.color = isTerminal(w)
}

// SetWidth sets the progress bar width.
func (p *ProgressBar) SetWidth(width int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.width = width
}

// SetMessage sets the current message.
func (p *ProgressBar) SetMessage(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.message = message
}

// Set sets the current progress value.
func (p *ProgressBar) Set(current int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = current
	p.render()
}

// Increment increases the progress by 1.
func (p *ProgressBar) Increment() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current++
	p.render()
}

// render draws the progress bar.
func (p *ProgressBar) render() {
	if p.total <= 0 {
		return
	}

	percent := float64(p.current) / float64(p.total)
	if percent > 1 {
		percent = 1
	}

	filled := int(percent * float64(p.width))
	empty := p.width - filled

	elapsed := time.Since(p.startTime)
	var eta string
	if p.current > 0 && percent < 1 {
		remaining := time.Duration(float64(elapsed) * (1 - percent) / percent)
		eta = fmt.Sprintf(" ETA: %s", remaining.Round(time.Second))
	}

	bar := strings.Repeat("=", filled) + strings.Repeat(" ", empty)
	pctStr := fmt.Sprintf("%3.0f%%", percent*100)

	if p.color {
		_, _ = fmt.Fprintf(p.out, "\r\033[K%s[%s%s%s]%s %s %d/%d%s",
			colorCyan, colorGreen, bar, colorCyan, colorReset,
			pctStr, p.current, p.total, eta)
	} else {
		_, _ = fmt.Fprintf(p.out, "\r[%s] %s %d/%d%s", bar, pctStr, p.current, p.total, eta)
	}
}

// Finish completes the progress bar.
func (p *ProgressBar) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = p.total
	p.render()
	_, _ = fmt.Fprintln(p.out)
}

// SyncProgress provides specialized progress for rsync-style sync operations.
type SyncProgress struct {
	out           io.Writer
	filesReceived int
	filesTotal    int
	bytesReceived int64
	phase         string
	startTime     time.Time
	lastUpdate    time.Time
	color         bool
	mu            sync.Mutex
}

// NewSyncProgress creates a sync progress tracker.
func NewSyncProgress() *SyncProgress {
	return &SyncProgress{
		out:       os.Stdout,
		color:     isTerminal(os.Stdout),
		startTime: time.Now(),
		phase:     "Connecting",
	}
}

// SetOutput sets the output writer.
func (s *SyncProgress) SetOutput(w io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.out = w
	s.color = isTerminal(w)
}

// SetPhase sets the current sync phase.
func (s *SyncProgress) SetPhase(phase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = phase
	s.render()
}

// SetFilesReceived updates the file count.
func (s *SyncProgress) SetFilesReceived(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Rate limit updates to avoid flooding
	now := time.Now()
	if now.Sub(s.lastUpdate) < 100*time.Millisecond && count-s.filesReceived < 1000 {
		s.filesReceived = count
		return
	}
	s.lastUpdate = now
	s.filesReceived = count
	s.render()
}

// SetFilesTotal sets the expected total file count.
func (s *SyncProgress) SetFilesTotal(total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.filesTotal = total
}

// AddBytes adds to the bytes received counter.
func (s *SyncProgress) AddBytes(bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bytesReceived += bytes
}

// render draws the sync progress.
func (s *SyncProgress) render() {
	elapsed := time.Since(s.startTime).Round(time.Second)

	var line string
	if s.filesTotal > 0 {
		pct := float64(s.filesReceived) / float64(s.filesTotal) * 100
		line = fmt.Sprintf("%s: %d/%d files (%.1f%%) - %s",
			s.phase, s.filesReceived, s.filesTotal, pct, elapsed)
	} else {
		line = fmt.Sprintf("%s: %d files - %s", s.phase, s.filesReceived, elapsed)
	}

	if s.color {
		_, _ = fmt.Fprintf(s.out, "\r\033[K%s>>>%s %s", colorGreen, colorReset, line)
	} else {
		_, _ = fmt.Fprintf(s.out, "\r>>> %s", line)
	}
}

// Finish completes the sync progress display.
func (s *SyncProgress) Finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprintln(s.out)
}
