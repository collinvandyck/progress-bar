package main

import (
	"fmt"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	progressbar "github.com/collin/progress-bar"
)

const total = 500

// provider implements progressbar.DataProvider.
type provider struct {
	counter atomic.Int64
	errors  atomic.Int64
	start   time.Time
}

func newProvider() *provider {
	return &provider{start: time.Now()}
}

// simulate does the "work": increments the counter once per 20ms until done.
func (p *provider) simulate() {
	for {
		current := p.counter.Load()
		if current >= total {
			return
		}
		// Occasionally log a fake error to make the display interesting.
		if current > 0 && current%73 == 0 {
			p.errors.Add(1)
		}
		p.counter.Add(1)
		time.Sleep(20 * time.Millisecond)
	}
}

func (p *provider) Progress() (current, total int) {
	return int(p.counter.Load()), total
}

func (p *provider) KeyValues() []progressbar.KeyValue {
	current := p.counter.Load()
	elapsed := time.Since(p.start)

	var rate float64
	if elapsed.Seconds() > 0 {
		rate = float64(current) / elapsed.Seconds()
	}

	return []progressbar.KeyValue{
		{Key: "Elapsed", Value: elapsed.Round(time.Millisecond).String()},
		{Key: "Rate", Value: fmt.Sprintf("%.1f items/s", rate)},
		{Key: "Items", Value: fmt.Sprintf("%d / %d", current, total)},
		{Key: "Errors", Value: fmt.Sprintf("%d", p.errors.Load())},
	}
}

func (p *provider) Sections() []progressbar.Section {
	current := p.counter.Load()
	var msg string
	if current >= total {
		msg = "Done."
	} else {
		msg = fmt.Sprintf("Processing item %d...", current)
	}
	return []progressbar.Section{
		{Title: "Status", Content: msg},
	}
}

// parentModel wraps the progress bar and handles quit keys.
type parentModel struct {
	progress progressbar.Model
}

func (m parentModel) Init() tea.Cmd {
	return m.progress.Init()
}

func (m parentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	updated, cmd := m.progress.Update(msg)
	m.progress = updated.(progressbar.Model)
	return m, cmd
}

func (m parentModel) View() tea.View {
	return m.progress.View()
}

func main() {
	p := newProvider()
	go p.simulate()

	model := parentModel{
		progress: progressbar.New(progressbar.Options{
			Provider: p,
			// Layout defaults to LayoutBarBottom — no need to set it.
		}),
	}

	prog := tea.NewProgram(model)
	if _, err := prog.Run(); err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
