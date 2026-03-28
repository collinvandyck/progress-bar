package progressbar

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

const defaultTickInterval = 100 * time.Millisecond

// TickMsg triggers a re-read from the DataProvider.
type TickMsg time.Time

// Model is a BubbleTea v2 model that renders a progress bar widget.
type Model struct {
	opts Options

	// Cached state from last provider read
	current  int
	total    int
	kvs      []KeyValue
	sections []Section

	width int
}

// New creates a new progress bar Model.
func New(opts Options) Model {
	return Model{opts: opts}
}

func doTick() tea.Cmd {
	return tea.Tick(defaultTickInterval, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Init starts the tick loop.
func (m Model) Init() tea.Cmd {
	return doTick()
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case TickMsg:
		if m.opts.Provider != nil {
			m.current, m.total = m.opts.Provider.Progress()
			m.kvs = m.opts.Provider.KeyValues()
			m.sections = m.opts.Provider.Sections()
		}
		return m, doTick()
	case tea.WindowSizeMsg:
		wsm := msg.(tea.WindowSizeMsg)
		m.width = wsm.Width
		return m, nil
	}
	return m, nil
}

// View renders the widget.
func (m Model) View() tea.View {
	return tea.NewView(m.renderView())
}
