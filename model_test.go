package progressbar

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// stubProvider implements DataProvider for tests.
type stubProvider struct {
	current, total int
	kvs            []KeyValue
	sections       []Section
}

func (s *stubProvider) Progress() (int, int)  { return s.current, s.total }
func (s *stubProvider) KeyValues() []KeyValue { return s.kvs }
func (s *stubProvider) Sections() []Section   { return s.sections }

func TestNewModel(t *testing.T) {
	p := &stubProvider{current: 0, total: 100}
	m := New(Options{Provider: p})
	if m.opts.Provider == nil {
		t.Fatal("provider should be set")
	}
}

func TestModelImplementsTeaModel(t *testing.T) {
	p := &stubProvider{current: 0, total: 100}
	m := New(Options{Provider: p})
	var _ tea.Model = m
}

func TestModelInit(t *testing.T) {
	p := &stubProvider{current: 0, total: 100}
	m := New(Options{Provider: p})
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return a tick command")
	}
}
