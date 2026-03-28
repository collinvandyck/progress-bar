package progressbar

import (
	"fmt"
	"strings"
)

const defaultWidth = 80

func (m Model) effectiveWidth() int {
	if m.width > 0 {
		return m.width
	}
	return defaultWidth
}

// renderBar produces the progress bar line: [█████░░░░░] XX% (current/total)
func (m Model) renderBar() string {
	pct := 0
	if m.total > 0 {
		pct = m.current * 100 / m.total
	}

	suffix := fmt.Sprintf(" %d%% (%d/%d)", pct, m.current, m.total)

	// 2 chars for brackets, rest for suffix
	w := m.effectiveWidth()
	barWidth := w - 2 - len(suffix)
	if barWidth < 1 {
		barWidth = 1
	}

	filled := 0
	if m.total > 0 {
		filled = m.current * barWidth / m.total
	}
	if filled > barWidth {
		filled = barWidth
	}

	var b strings.Builder
	b.WriteRune('[')
	for i := 0; i < barWidth; i++ {
		if i < filled {
			b.WriteRune('█')
		} else {
			b.WriteRune('░')
		}
	}
	b.WriteRune(']')
	b.WriteString(suffix)
	return b.String()
}

// renderKeyValues renders key-value pairs inline separated by double spaces.
func (m Model) renderKeyValues() string {
	if len(m.kvs) == 0 {
		return ""
	}
	parts := make([]string, len(m.kvs))
	for i, kv := range m.kvs {
		parts[i] = kv.Key + ": " + kv.Value
	}
	return strings.Join(parts, "  ")
}

// renderSections renders each section as Title\nContent with blank lines between.
func (m Model) renderSections() string {
	if len(m.sections) == 0 {
		return ""
	}
	parts := make([]string, len(m.sections))
	for i, s := range m.sections {
		parts[i] = s.Title + "\n" + s.Content
	}
	return strings.Join(parts, "\n\n")
}

// renderView assembles the full widget output based on the layout.
func (m Model) renderView() string {
	bar := m.renderBar()
	kvs := m.renderKeyValues()
	sections := m.renderSections()

	var parts []string

	switch m.opts.Layout {
	case LayoutBarTop, LayoutCompact:
		parts = append(parts, bar)
		if kvs != "" {
			parts = append(parts, kvs)
		}
		if sections != "" {
			parts = append(parts, sections)
		}
	default: // LayoutBarBottom
		if kvs != "" {
			parts = append(parts, kvs)
		}
		if sections != "" {
			parts = append(parts, sections)
		}
		parts = append(parts, bar)
	}

	return strings.Join(parts, "\n")
}
