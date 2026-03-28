//go:build js && wasm

package main

import (
	"fmt"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"

	progressbar "github.com/collin/progress-bar"
)

// demoProvider is a DataProvider that simulates progress for the WASM demo.
type demoProvider struct {
	current atomic.Int64
	total   int
}

func (d *demoProvider) Progress() (int, int) {
	return int(d.current.Load()), d.total
}

func (d *demoProvider) KeyValues() []progressbar.KeyValue {
	cur := int(d.current.Load())
	return []progressbar.KeyValue{
		{Key: "Status", Value: "Running"},
		{Key: "Items", Value: fmt.Sprintf("%d/%d", cur, d.total)},
	}
}

func (d *demoProvider) Sections() []progressbar.Section {
	return nil
}

func main() {
	provider := &demoProvider{total: 200}

	// Simulate progress in background.
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
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithoutSignalHandler(),
		tea.WithoutRenderer(),
		tea.WithWindowSize(80, 24),
	)

	RegisterBridge(prog)

	// Run in a goroutine so main doesn't exit (WASM needs main alive).
	go func() {
		_, _ = prog.Run()
	}()

	// Block forever.
	select {}
}
