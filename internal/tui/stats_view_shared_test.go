package tui

import (
	"testing"

	"github.com/kayden-kim/oc/internal/stats"
)

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

func TestSessionTableRows_GroupsMessageCounts(t *testing.T) {
	rows := sessionTableRows([]stats.SessionUsage{{ID: "ses_big", Messages: 12345}}, "")
	if got := rows[0].Cells[2]; got != "12,345" {
		t.Fatalf("expected grouped session message count, got %q", got)
	}
}

func TestSessionTableRows_DoesNotInsertMissingCurrentSessionRow(t *testing.T) {
	rows := sessionTableRows([]stats.SessionUsage{{ID: "ses_other", Messages: 1}}, "ses_current")
	if len(rows) != 1 {
		t.Fatalf("expected only actual session rows, got %+v", rows)
	}
}

func TestCompactSessionRowText_JoinsAndTrimsCells(t *testing.T) {
	row := statsTableRow{Cells: []string{"*", "ses_1", "12", "3k", "$4.56", "Current"}}
	if got := compactSessionRowText(row); got != "* ses_1 12 3k $4.56 Current" {
		t.Fatalf("expected compact session row text, got %q", got)
	}

	empty := statsTableRow{Cells: []string{"", "-", "-", "-", "-", "-"}}
	if got := compactSessionRowText(empty); got != "- - - - -" {
		t.Fatalf("expected trimmed compact fallback row, got %q", got)
	}
}
