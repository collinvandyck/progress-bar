//go:build js && wasm

package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	progressbar "github.com/collin/progress-bar"
)

// demoProvider is a DataProvider that simulates progress for the WASM demo.
type demoProvider struct {
	current atomic.Int64
	total   int
	start   time.Time
}

func (d *demoProvider) Progress() (int, int) {
	return int(d.current.Load()), d.total
}

func (d *demoProvider) KeyValues() []progressbar.KeyValue {
	cur := int(d.current.Load())
	elapsed := time.Since(d.start).Truncate(time.Second)
	rate := 0.0
	if elapsed.Seconds() > 0 {
		rate = float64(cur) / elapsed.Seconds()
	}
	return []progressbar.KeyValue{
		{Key: "⏱ Elapsed", Value: elapsed.String()},
		{Key: "⚡ Rate", Value: fmt.Sprintf("%.0f/s", rate)},
		{Key: "◇ Items", Value: fmt.Sprintf("%d/%d", cur, d.total)},
		{Key: "✗ Errors", Value: "0"},
	}
}

func (d *demoProvider) Sections() []progressbar.Section {
	cur := int(d.current.Load())
	if cur >= d.total {
		return []progressbar.Section{{Title: "", Content: "✓ Done"}}
	}
	return []progressbar.Section{
		{Title: "", Content: fmt.Sprintf("→ Processing item %d⋯", cur)},
	}
}

// renderFrame renders the widget directly from a DataProvider.
// This bypasses BubbleTea's renderer, which uses cell-level ANSI diffs
// that don't survive polled I/O in WASM. Instead we do full redraws
// with cursor-home + clear, which xterm.js handles cleanly.
func renderFrame(provider progressbar.DataProvider, width int) string {
	current, total := provider.Progress()
	kvs := provider.KeyValues()
	sections := provider.Sections()

	// KVs
	kvParts := make([]string, len(kvs))
	for i, kv := range kvs {
		kvParts[i] = kv.Key + ": " + kv.Value
	}
	kvsLine := strings.Join(kvParts, "  ")

	// Sections
	var sectionLines []string
	for _, s := range sections {
		if s.Title != "" {
			sectionLines = append(sectionLines, s.Title+"\n"+s.Content)
		} else {
			sectionLines = append(sectionLines, s.Content)
		}
	}

	// Bar — Unicode block characters rendered via xterm.js WebGL addon
	// which draws them with vector math instead of font glyphs.
	pct := 0
	if total > 0 {
		pct = current * 100 / total
	}
	suffix := fmt.Sprintf(" %d%% (%d/%d)", pct, current, total)
	barWidth := width - 2 - len(suffix)
	if barWidth < 1 {
		barWidth = 1
	}
	// Cap bar width for readability
	const maxBarWidth = 50
	if barWidth > maxBarWidth {
		barWidth = maxBarWidth
	}
	filled := 0
	if total > 0 {
		filled = current * barWidth / total
	}
	if filled > barWidth {
		filled = barWidth
	}
	var bar strings.Builder
	bar.WriteRune('[')
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar.WriteRune('█')
		} else {
			bar.WriteRune('░')
		}
	}
	bar.WriteRune(']')
	bar.WriteString(suffix)

	// Separator
	sepWidth := 50
	if width > 0 && width < sepWidth {
		sepWidth = width
	}
	sep := strings.Repeat("─", sepWidth)

	// Assemble: LayoutBarBottom
	var lines []string
	if kvsLine != "" {
		lines = append(lines, kvsLine)
	}
	lines = append(lines, sep)
	for _, sl := range sectionLines {
		// Sections can have \n in them (Title\nContent)
		lines = append(lines, strings.Split(sl, "\n")...)
	}
	lines = append(lines, bar.String())

	// Each line ends with \e[K (clear to end of line) to prevent
	// residual characters from previous frames.
	// JS side prepends \e[H (cursor home) and appends \e[J (clear below).
	var frame strings.Builder
	for i, line := range lines {
		frame.WriteString(line)
		frame.WriteString("\x1b[K") // clear to end of line
		if i < len(lines)-1 {
			frame.WriteString("\r\n")
		}
	}
	return frame.String()
}

func main() {
	provider := &demoProvider{total: 200, start: time.Now()}

	// Simulate progress in background.
	go func() {
		for provider.current.Load() < int64(provider.total) {
			time.Sleep(50 * time.Millisecond)
			provider.current.Add(1)
		}
	}()

	// Terminal width, updated by JS resize events.
	var width atomic.Int32
	width.Store(80)

	// JS calls bubbletea_read() on each poll interval.
	// Each call renders a fresh frame — no Go-side render loop needed.
	RegisterBridgeSimple(&width, func() string {
		return renderFrame(provider, int(width.Load()))
	})

	// Block forever.
	select {}
}
