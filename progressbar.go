package progressbar

import "io"

// DataProvider is the interface a host implements to feed data to the widget.
// The widget reads from it on every tick cycle.
type DataProvider interface {
	// Progress returns the current count and total.
	Progress() (current, total int)

	// KeyValues returns labeled data pairs for the standard KV area.
	KeyValues() []KeyValue

	// Sections returns freeform named content blocks.
	Sections() []Section
}

// KeyValue is a labeled data point displayed in the KV area.
type KeyValue struct {
	Key   string
	Value string
}

// Section is a named freeform content block.
type Section struct {
	Title   string
	Content string
}

// Layout controls the arrangement of widget components.
type Layout int

const (
	LayoutBarBottom Layout = iota // kvs → sections → bar (default, zero value)
	LayoutBarTop                  // bar → kvs → sections
	LayoutCompact                 // bar+% inline → kvs → sections
)

// Options configures the progress bar widget.
type Options struct {
	Provider DataProvider
	Output   io.Reader // optional — widget reads lines from this
	Layout   Layout
}
