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
		{Key: "Elapsed", Value: elapsed.String()},
		{Key: "Rate", Value: fmt.Sprintf("%.0f/s", rate)},
		{Key: "Items", Value: fmt.Sprintf("%d/%d", cur, d.total)},
		{Key: "Errors", Value: "0"},
	}
}

func (d *demoProvider) Sections() []progressbar.Section {
	cur := int(d.current.Load())
	if cur >= d.total {
		return []progressbar.Section{{Title: "Status", Content: "Done!"}}
	}
	return []progressbar.Section{
		{Title: "Status", Content: fmt.Sprintf("Processing item %d...", cur)},
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
		sectionLines = append(sectionLines, s.Title+"\n"+s.Content)
	}

	// Bar
	pct := 0
	if total > 0 {
		pct = current * 100 / total
	}
	suffix := fmt.Sprintf(" %d%% (%d/%d)", pct, current, total)
	barWidth := width - 2 - len(suffix)
	if barWidth < 1 {
		barWidth = 1
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

	// Assemble: LayoutBarBottom
	var parts []string
	if kvsLine != "" {
		parts = append(parts, kvsLine)
	}
	for _, sl := range sectionLines {
		parts = append(parts, sl)
	}
	parts = append(parts, bar.String())

	// Cursor home + clear screen + content
	return "\x1b[H\x1b[2J" + strings.Join(parts, "\n")
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

	RegisterBridgeSimple(&width)

	// Wait for JS bridge + initial resize.
	time.Sleep(300 * time.Millisecond)

	// Render loop: full-frame redraws at 10fps.
	for {
		w := int(width.Load())
		frame := renderFrame(provider, w)
		output.Write([]byte(frame))
		time.Sleep(100 * time.Millisecond)
	}
}
