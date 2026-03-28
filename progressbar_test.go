package progressbar

import "testing"

func TestKeyValueStringer(t *testing.T) {
	kv := KeyValue{Key: "Elapsed", Value: "1m 30s"}
	if kv.Key != "Elapsed" || kv.Value != "1m 30s" {
		t.Fatal("KeyValue fields not set correctly")
	}
}

func TestSectionContent(t *testing.T) {
	s := Section{Title: "Status", Content: "Processing batch 3 of 10"}
	if s.Title != "Status" || s.Content != "Processing batch 3 of 10" {
		t.Fatal("Section fields not set correctly")
	}
}

func TestLayoutDefaults(t *testing.T) {
	if LayoutBarBottom != 0 {
		t.Fatal("LayoutBarBottom should be 0 (iota, default)")
	}
	if LayoutBarTop != 1 {
		t.Fatal("LayoutBarTop should be 1")
	}
	if LayoutCompact != 2 {
		t.Fatal("LayoutCompact should be 2")
	}
}

func TestOptionsDefaults(t *testing.T) {
	opts := Options{}
	if opts.Layout != LayoutBarBottom {
		t.Fatal("zero-value Layout should be LayoutBarBottom")
	}
	if opts.Provider != nil {
		t.Fatal("zero-value Provider should be nil")
	}
}
