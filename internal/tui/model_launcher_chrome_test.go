package tui

import (
	"strings"
	"testing"
	"time"
)

func TestRenderHelpLine_IncludesStyledKeyTokens(t *testing.T) {
	helpLine := renderHelpLine(maxLayoutWidth)
	for _, token := range []string{"↑/↓", "space", "enter", "s", "c", "q"} {
		if !strings.Contains(helpLine, helpBgKeyStyle.Render(token)) {
			t.Fatalf("expected styled help token %q in %q", token, helpLine)
		}
	}
	for _, action := range []string{"navigate", "toggle", "confirm", "sessions", "config", "quit"} {
		if !strings.Contains(helpLine, action) || strings.Contains(helpLine, helpBgKeyStyle.Render(action)) {
			t.Fatalf("expected unstyled action %q in %q", action, helpLine)
		}
	}
	if !strings.Contains(helpLine, helpBgTextStyle.Render(": quit")) {
		t.Fatalf("expected default text color on help copy, got %q", helpLine)
	}
}

func TestView_RendersStyledHeaderLine(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true).View().Content
	headerLine := strings.Split(view, "\n")[0]
	expected := Model{version: testVersion}.renderTopBadge()
	if headerLine != expected {
		t.Fatalf("expected top badge %q, got %q", expected, headerLine)
	}
}

func TestView_RendersPluginSelectionPrompt(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true).View().Content
	expected := renderSectionHeader("📋 Choose plugins", maxLayoutWidth)
	if !strings.Contains(view, expected) {
		t.Fatalf("expected plugin prompt line %q in %q", expected, view)
	}
}

func TestViewLauncher_MatchesDefaultView(t *testing.T) {
	m := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)
	if got, want := m.viewLauncher().Content, m.View().Content; got != want {
		t.Fatalf("expected launcher helper to match default view\nhelper: %q\nview:   %q", got, want)
	}
}

func TestView_RendersStyledHelpLine(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true).View().Content
	if !strings.Contains(view, renderHelpLine(maxLayoutWidth)) {
		t.Fatalf("expected help line %q in %q", renderHelpLine(maxLayoutWidth), view)
	}
}

func TestRenderTopBadge_ContainsBrandAndVersion(t *testing.T) {
	rendered := Model{version: testVersion}.renderTopBadge()
	expected := expectedTopBadge(testVersion, SessionItem{})
	if rendered != expected {
		t.Fatalf("expected top badge %q, got %q", expected, rendered)
	}
}

func TestRenderTopBadge_IncludesSelectedSessionInfoWithMetaBackground(t *testing.T) {
	session := SessionItem{ID: "ses_latest", Title: "Latest session", UpdatedAt: time.Now()}
	rendered := Model{version: testVersion, session: session}.renderTopBadge()
	expected := expectedTopBadge(testVersion, session)
	if rendered != expected {
		t.Fatalf("expected top badge %q, got %q", expected, rendered)
	}
}
