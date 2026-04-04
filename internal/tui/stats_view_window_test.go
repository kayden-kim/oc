package tui

import "testing"

func TestWindowModelColumns_UsesReasonHeaderAndExpandableNameColumn(t *testing.T) {
	columns := windowModelColumns()
	if len(columns) < 6 {
		t.Fatalf("expected model columns, got %#v", columns)
	}
	if !columns[0].Expand {
		t.Fatalf("expected first model column to absorb remaining width, got %#v", columns[0])
	}
	if columns[5].Header != "reason" {
		t.Fatalf("expected reasoning column header to be renamed to reason, got %q", columns[5].Header)
	}
}

func TestWindowModelDisplayName_UsesProviderAbbreviationPrefix(t *testing.T) {
	if got := windowModelDisplayName("openai\x00gpt-5.4"); got != "oa| gpt-5.4" {
		t.Fatalf("expected provider abbreviation prefix, got %q", got)
	}
	if got := windowModelDisplayName("gpt-5.4"); got != "gpt-5.4" {
		t.Fatalf("expected plain model unchanged, got %q", got)
	}
}
