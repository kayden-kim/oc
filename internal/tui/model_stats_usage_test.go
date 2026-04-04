package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kayden-kim/oc/internal/stats"
)

func TestRenderUsageLines_AlignsBarsToLongestLabel(t *testing.T) {
	lines := (Model{}).renderUsageLines("count", []stats.UsageCount{{Name: "bash", Count: 21}, {Name: "very-long-tool-name", Count: 11}, {Name: "go", Count: 8}}, 42)
	if len(lines) != 7 {
		t.Fatalf("expected 7 usage lines, got %d", len(lines))
	}
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = stripANSI(line)
	}
	if strings.Contains(plain[0], "tool") || strings.Contains(plain[0], "bar") || !strings.Contains(plain[0], "count") || !strings.Contains(plain[0], "share") {
		t.Fatalf("expected usage table header, got %q", plain[0])
	}
	if !strings.Contains(plain[2], "bash") || !strings.Contains(plain[2], "████████ 50%") || !strings.Contains(plain[2], "21") {
		t.Fatalf("expected first usage row, got %q", plain[2])
	}
	if !strings.Contains(plain[3], "very-long-tool-name") || !strings.Contains(plain[3], "████···· 26%") {
		t.Fatalf("expected second usage row, got %q", plain[3])
	}
	if !strings.Contains(plain[6], "Total") || !strings.Contains(plain[6], "········ 100%") || !strings.Contains(plain[6], "42") {
		t.Fatalf("expected total usage row, got %q", plain[6])
	}
}
func TestRenderUsageLines_AlignsOthersAndTotalToLongestLabel(t *testing.T) {
	items := make([]stats.UsageCount, 0, 16)
	for i := range 16 {
		items = append(items, stats.UsageCount{Name: fmt.Sprintf("t%d", i+1), Count: 20 - i})
	}
	lines := (Model{}).renderUsageLines("count", items, 200)
	if len(lines) < 3 {
		t.Fatalf("expected usage lines, got %v", lines)
	}
	othersLine, totalLine := stripANSI(lines[len(lines)-3]), stripANSI(lines[len(lines)-1])
	if !strings.Contains(othersLine, "others") || !strings.Contains(totalLine, "Total") {
		t.Fatalf("expected others and total lines, got others=%q total=%q", othersLine, totalLine)
	}
	othersColumn, totalColumn := strings.Index(othersLine, "others"), strings.Index(totalLine, "Total")
	if othersColumn != totalColumn {
		t.Fatalf("expected aligned first column, got others=%d total=%d", othersColumn, totalColumn)
	}
}
func TestRenderUsageLines_GroupsLargeCounts(t *testing.T) {
	lines := (Model{}).renderUsageLines("count", []stats.UsageCount{{Name: "bash", Count: 12345}}, 23456)
	if len(lines) != 5 {
		t.Fatalf("expected 5 usage lines, got %d", len(lines))
	}
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = stripANSI(line)
	}
	if !strings.Contains(plain[2], "12,345") {
		t.Fatalf("expected grouped usage count, got %q", plain[2])
	}
	if !strings.Contains(plain[4], "23,456") || !strings.Contains(plain[4], "100%") {
		t.Fatalf("expected grouped total usage count, got %q", plain[4])
	}
	if strings.Contains(plain[2], "• 1 bash ") || !strings.Contains(plain[4], "········") || strings.Contains(plain[4], "████") {
		t.Fatalf("unexpected grouped row formatting, row=%q total=%q", plain[2], plain[4])
	}
}
func TestRenderUsageLines_ShowsPlaceholderOnlyWhenTotalIsZero(t *testing.T) {
	lines := (Model{}).renderUsageLines("count", nil, 0)
	if len(lines) != 3 {
		t.Fatalf("expected 3 usage lines, got %d", len(lines))
	}
	if !strings.Contains(stripANSI(lines[2]), "-") {
		t.Fatalf("expected placeholder row, got %q", stripANSI(lines[2]))
	}
	if strings.Contains(stripANSI(strings.Join(lines, "\n")), "Total") {
		t.Fatalf("expected no total row for zero totals, got %q", stripANSI(strings.Join(lines, "\n")))
	}
}
func TestRenderUsageLines_ShowsTotalWhenItemsMissingButTotalExists(t *testing.T) {
	lines := (Model{}).renderUsageLines("count", nil, 42)
	plain := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(plain, "-") || !strings.Contains(plain, "Total") || !strings.Contains(plain, "42") || !strings.Contains(plain, "········ 100%") {
		t.Fatalf("expected placeholder and total rows when aggregate total exists, got %q", plain)
	}
	if strings.Count(plain, strings.Repeat("┈", 10)) < 2 {
		t.Fatalf("expected header and total dividers, got %q", plain)
	}
}
func TestRenderUsageLines_FormatsModelAmountsCompactly(t *testing.T) {
	lines := (Model{}).renderUsageLines("tokens", []stats.UsageCount{{Name: "gpt-5.4", Amount: 1_250_000}}, 1_500_000)
	if len(lines) != 5 {
		t.Fatalf("expected 5 usage lines, got %d", len(lines))
	}
	if !strings.Contains(stripANSI(lines[2]), "1.2M") {
		t.Fatalf("expected compact model amount in usage row, got %q", stripANSI(lines[2]))
	}
	if !strings.Contains(stripANSI(lines[4]), "1.5M") || !strings.Contains(stripANSI(lines[4]), "100%") {
		t.Fatalf("expected compact model amount in total row, got %q", stripANSI(lines[4]))
	}
}
func TestRenderProjectUsageLines_ShowsCostColumn(t *testing.T) {
	lines := (Model{}).renderProjectUsageLines([]stats.UsageCount{{Name: "/tmp/work-a", Amount: 1_250_000, Cost: 12.34}}, 1_500_000, 15.67)
	if len(lines) != 5 {
		t.Fatalf("expected 5 project usage lines, got %d", len(lines))
	}
	plain := stripANSI(strings.Join(lines, "\n"))
	for _, snippet := range []string{"tokens", "cost", "share", "/tmp/work-a", "1.2M", "$12.34", "$15.67", "83%", "100%"} {
		if !strings.Contains(plain, snippet) {
			t.Fatalf("expected project usage snippet %q, got %q", snippet, plain)
		}
	}
	if strings.Contains(plain, "████") || strings.Contains(plain, "····") {
		t.Fatalf("expected project usage share graph removed, got %q", plain)
	}
}
func TestRenderModelUsageLines_ShowsCostColumn(t *testing.T) {
	lines := (Model{}).renderModelUsageLines([]stats.UsageCount{{Name: "openai\x00gpt-5.4", Amount: 1_250_000, Cost: 12.34}}, 1_500_000, 15.67)
	if len(lines) != 5 {
		t.Fatalf("expected 5 model usage lines, got %d", len(lines))
	}
	plain := stripANSI(strings.Join(lines, "\n"))
	for _, snippet := range []string{"provider", "tokens", "cost", "share", "openai", "gpt-5.4", "1.2M", "$12.34", "$15.67", "83%", "100%"} {
		if !strings.Contains(plain, snippet) {
			t.Fatalf("expected model usage snippet %q, got %q", snippet, plain)
		}
	}
	if strings.Contains(plain, "████") || strings.Contains(plain, "····") {
		t.Fatalf("expected model usage share graph removed, got %q", plain)
	}
}
func TestRenderUsageLines_GroupsRemainderIntoOthersAfterTop15(t *testing.T) {
	items := make([]stats.UsageCount, 0, 17)
	total := int64(0)
	for i := range 17 {
		count := 20 - i
		items = append(items, stats.UsageCount{Name: fmt.Sprintf("tool-%02d", i+1), Count: count})
		total += int64(count)
	}
	lines := (Model{}).renderUsageLines("count", items, total)
	if len(lines) != 20 {
		t.Fatalf("expected 20 usage lines including header/dividers/others/total, got %d", len(lines))
	}
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = stripANSI(line)
	}
	if !strings.Contains(plain[17], "others") {
		t.Fatalf("expected others row at index 17, got %q", plain[17])
	}
	if !strings.Contains(plain[17], "9") || !strings.Contains(plain[17], "4%") {
		t.Fatalf("expected others row to aggregate hidden items, got %q", plain[17])
	}
	if !strings.Contains(plain[19], "204") || !strings.Contains(plain[19], "100%") {
		t.Fatalf("expected total row to remain at the end, got %q", plain[19])
	}
}
