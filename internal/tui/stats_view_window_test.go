package tui

import (
	"testing"

	"github.com/kayden-kim/oc/internal/stats"
)

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

func TestSessionTableRows_MarksCurrentSessionAndFallsBackWhenEmpty(t *testing.T) {
	rows := sessionTableRows([]stats.SessionUsage{{ID: "ses_1", Title: "Current", Messages: 12, Tokens: 3000, Cost: 4.56}}, "ses_1")
	if len(rows) != 1 {
		t.Fatalf("expected one session row, got %d", len(rows))
	}
	if rows[0].Cells[0] != "*" || rows[0].Cells[1] != "ses_1" || rows[0].Cells[5] != "Current" {
		t.Fatalf("expected marked current session row, got %#v", rows[0].Cells)
	}

	empty := sessionTableRows(nil, "")
	if len(empty) != 1 || empty[0].Cells[1] != "-" {
		t.Fatalf("expected empty fallback row, got %#v", empty)
	}
}
