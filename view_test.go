package progressbar

import (
	"strings"
	"testing"
)

func TestRenderProgressBar(t *testing.T) {
	m := Model{
		current: 50,
		total:   100,
		width:   40,
	}
	bar := m.renderBar()
	if !strings.Contains(bar, "50%") {
		t.Fatalf("expected bar to contain '50%%', got: %s", bar)
	}
}

func TestRenderBarZeroTotal(t *testing.T) {
	m := Model{
		current: 0,
		total:   0,
		width:   40,
	}
	bar := m.renderBar()
	if !strings.Contains(bar, "0%") {
		t.Fatalf("expected bar to contain '0%%', got: %s", bar)
	}
}

func TestRenderKeyValues(t *testing.T) {
	m := Model{
		kvs: []KeyValue{
			{Key: "Elapsed", Value: "1m 30s"},
			{Key: "Rate", Value: "500/s"},
		},
		width: 60,
	}
	out := m.renderKeyValues()
	if !strings.Contains(out, "Elapsed") || !strings.Contains(out, "1m 30s") {
		t.Fatalf("expected KVs in output, got: %s", out)
	}
	if !strings.Contains(out, "Rate") || !strings.Contains(out, "500/s") {
		t.Fatalf("expected KVs in output, got: %s", out)
	}
}

func TestRenderSections(t *testing.T) {
	m := Model{
		sections: []Section{
			{Title: "Status", Content: "Processing batch 3 of 10"},
		},
		width: 60,
	}
	out := m.renderSections()
	if !strings.Contains(out, "Status") || !strings.Contains(out, "Processing batch 3") {
		t.Fatalf("expected section in output, got: %s", out)
	}
}

func TestRenderViewLayoutBarBottom(t *testing.T) {
	m := Model{
		opts:    Options{Layout: LayoutBarBottom},
		current: 25,
		total:   100,
		kvs:     []KeyValue{{Key: "Elapsed", Value: "30s"}},
		width:   40,
	}
	out := m.renderView()
	barIdx := strings.Index(out, "25%")
	kvIdx := strings.Index(out, "Elapsed")
	if kvIdx > barIdx {
		t.Fatal("LayoutBarBottom: KVs should appear before bar")
	}
}

func TestRenderViewLayoutBarTop(t *testing.T) {
	m := Model{
		opts:    Options{Layout: LayoutBarTop},
		current: 25,
		total:   100,
		kvs:     []KeyValue{{Key: "Elapsed", Value: "30s"}},
		width:   40,
	}
	out := m.renderView()
	barIdx := strings.Index(out, "25%")
	kvIdx := strings.Index(out, "Elapsed")
	if barIdx > kvIdx {
		t.Fatal("LayoutBarTop: bar should appear before KVs")
	}
}
