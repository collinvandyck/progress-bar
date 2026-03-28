//go:build js && wasm

package main

import (
	"fmt"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	progressbar "github.com/collin/progress-bar"
)

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

// outputWriter wraps the shared output buffer as an io.Writer.
type outputWriter struct{}

func (outputWriter) Write(p []byte) (int, error) {
	fromGo.mu.Lock()
	defer fromGo.mu.Unlock()
	return fromGo.buf.Write(p)
}

func main() {
	provider := &demoProvider{total: 200, start: time.Now()}

	go func() {
		for provider.current.Load() < int64(provider.total) {
			time.Sleep(50 * time.Millisecond)
			provider.current.Add(1)
		}
	}()

	model := progressbar.New(progressbar.Options{
		Provider: provider,
	})

	prog := tea.NewProgram(
		model,
		tea.WithInput(fromJS),
		tea.WithOutput(outputWriter{}),
		tea.WithAltScreen(),
	)

	RegisterBridge(prog)

	go func() {
		prog.Run()
	}()

	select {}
}
