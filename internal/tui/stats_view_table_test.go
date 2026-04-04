package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

func TestRenderStatsTable_RespectsMaxWidth(t *testing.T) {
	lines := renderStatsTable(
		[]statsTableColumn{{Header: "name", MinWidth: 4, Style: defaultTextStyle}, {Header: "value", MinWidth: 4, AlignRight: true, Style: statsValueTextStyle}, {Header: "share", MinWidth: 4, Style: statsValueTextStyle}},
		[]statsTableRow{{Cells: []string{"very-long-entry-name", "123456", "99%"}}, {Divider: true}, {Cells: []string{"Total", "123456", "100%"}}},
		statsTableMaxWidth,
	)
	if len(lines) != 5 {
		t.Fatalf("expected 5 table lines, got %d", len(lines))
	}
	for i, line := range lines {
		plain := stripANSI(line)
		if width := utf8.RuneCountInString(strings.TrimPrefix(plain, "    ")); width > statsTableMaxWidth {
			t.Fatalf("expected line %d width <= %d, got %d in %q", i, statsTableMaxWidth, width, plain)
		}
	}
	if !strings.Contains(stripANSI(lines[0]), "name") || !strings.Contains(stripANSI(lines[0]), "value") || !strings.Contains(stripANSI(lines[0]), "share") {
		t.Fatalf("expected header row, got %q", stripANSI(lines[0]))
	}
	if !strings.Contains(stripANSI(lines[1]), strings.Repeat("┈", 10)) {
		t.Fatalf("expected header divider, got %q", stripANSI(lines[1]))
	}
}

func TestRenderStatsTable_UsesDisplayWidthForWideGlyphs(t *testing.T) {
	lines := renderStatsTable(
		[]statsTableColumn{{Header: "name", MinWidth: 4, Style: defaultTextStyle}, {Header: "value", MinWidth: 4, AlignRight: true, Style: statsValueTextStyle}},
		[]statsTableRow{{Cells: []string{"모델이름이아주길어요테스트모델이름이아주길어요테스트모델이름이아주길어요테스트", "123456"}}, {Cells: []string{"🙂emoji-wide-name-with-extra-extra-extra-width", "42"}}},
		statsTableMaxWidth,
	)
	for i, line := range lines {
		plain := stripANSI(line)
		if width := lipgloss.Width(strings.TrimPrefix(plain, "    ")); width > statsTableMaxWidth {
			t.Fatalf("expected display width <= %d, got %d for line %d: %q", statsTableMaxWidth, width, i, plain)
		}
	}
	if !strings.Contains(stripANSI(lines[2]), "…") && !strings.Contains(stripANSI(lines[3]), "…") {
		t.Fatalf("expected truncation ellipsis for wide content, got %q", strings.Join([]string{stripANSI(lines[2]), stripANSI(lines[3])}, " | "))
	}
}

func TestRenderStatsTable_DoesNotPathTruncateModelNamesByDefault(t *testing.T) {
	lines := renderStatsTable(
		[]statsTableColumn{{Header: "Agent", MinWidth: 6, Style: defaultTextStyle}, {Header: "Model", MinWidth: 10, Style: defaultTextStyle}},
		[]statsTableRow{{Cells: []string{"explore", "qwen/qwen3-coder-super-long-model"}}},
		24,
	)
	plain := stripANSI(lines[2])
	if strings.Contains(plain, "..") {
		t.Fatalf("expected model names to use plain truncation, got %q", plain)
	}
	if !strings.Contains(plain, "qwen/") {
		t.Fatalf("expected model prefix to stay visible, got %q", plain)
	}
	if !strings.Contains(plain, "…") {
		t.Fatalf("expected truncated model ellipsis, got %q", plain)
	}
}

func TestRenderStatsTable_PathAwareColumnsUsePathTruncation(t *testing.T) {
	lines := renderStatsTable(
		[]statsTableColumn{{Header: "Project", MinWidth: 10, PathAware: true, Style: defaultTextStyle}},
		[]statsTableRow{{Cells: []string{"/Users/kayden/workspace/super-long-project-name"}}},
		18,
	)
	plain := stripANSI(lines[2])
	if !strings.Contains(plain, "..") {
		t.Fatalf("expected path-aware truncation, got %q", plain)
	}
}

func TestStatsTableColumnWidths_PrefersExpandableColumnsForRemainingWidth(t *testing.T) {
	columns := []statsTableColumn{{Header: "Model", MinWidth: 10, Expand: true, Style: defaultTextStyle}, {Header: "Cost", MinWidth: 4, AlignRight: true, Style: statsValueTextStyle}}
	rows := []statsTableRow{{Cells: []string{"gpt-5.4", "$1"}}}
	widths := statsTableColumnWidths(columns, rows, 20)
	if got, want := widths[0], 14; got != want {
		t.Fatalf("expected expandable first column width %d, got %d", want, got)
	}
	if got, want := widths[1], 4; got != want {
		t.Fatalf("expected non-expandable second column width %d, got %d", want, got)
	}
}

func TestShortenPathMiddle_PreservesPathEnds(t *testing.T) {
	for _, tt := range []struct {
		name  string
		path  string
		width int
		want  []string
	}{{name: "unix path", path: "/Users/kayden/workspace/super-long-project-name", width: 24, want: []string{"/Users", "..", "project-name"}}, {name: "windows path", path: `C:\Users\kayden\workspace\super-long-project-name`, width: 26, want: []string{`C:\Users`, "..", "project-name"}}, {name: "non path fallback", path: "very-long-non-path-value", width: 10, want: []string{"…"}}} {
		t.Run(tt.name, func(t *testing.T) {
			got := truncatePathAware(tt.path, tt.width)
			if lipgloss.Width(got) > tt.width {
				t.Fatalf("expected width <= %d, got %d in %q", tt.width, lipgloss.Width(got), got)
			}
			for _, snippet := range tt.want {
				if !strings.Contains(got, snippet) {
					t.Fatalf("expected %q in %q", snippet, got)
				}
			}
		})
	}
}
