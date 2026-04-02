package tui

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/session"
	"github.com/kayden-kim/oc/internal/stats"
)

// PluginItem represents a plugin that can be selected in the TUI
type PluginItem struct {
	Name             string
	InitiallyEnabled bool
	SourceLabel      string
}

// EditChoice represents a config file that can be opened from the TUI.
type EditChoice struct {
	Label string
	Path  string
}

type SessionItem = session.SessionItem

type statsState struct {
	globalStats                 stats.Report
	projectStats                stats.Report
	globalStatsLoaded           bool
	projectStatsLoaded          bool
	globalStatsLoading          bool
	projectStatsLoading         bool
	globalStatsUpdatedAt        time.Time
	projectStatsUpdatedAt       time.Time
	loadGlobalStats             func() (stats.Report, error)
	loadProjectStats            func() (stats.Report, error)
	globalDaily                 stats.WindowReport
	projectDaily                stats.WindowReport
	globalMonthly               stats.WindowReport
	projectMonthly              stats.WindowReport
	globalYearMonthly           stats.YearMonthlyReport
	projectYearMonthly          stats.YearMonthlyReport
	globalDailyLoaded           bool
	projectDailyLoaded          bool
	globalMonthlyLoaded         bool
	projectMonthlyLoaded        bool
	globalYearMonthlyLoaded     bool
	projectYearMonthlyLoaded    bool
	globalDailyLoading          bool
	projectDailyLoading         bool
	globalMonthlyLoading        bool
	projectMonthlyLoading       bool
	globalYearMonthlyLoading    bool
	projectYearMonthlyLoading   bool
	globalDailyUpdatedAt        time.Time
	projectDailyUpdatedAt       time.Time
	globalMonthlyUpdatedAt      time.Time
	projectMonthlyUpdatedAt     time.Time
	globalYearMonthlyUpdatedAt  time.Time
	projectYearMonthlyUpdatedAt time.Time
	loadGlobalWindow            func(string, time.Time, time.Time) (stats.WindowReport, error)
	loadProjectWindow           func(string, time.Time, time.Time) (stats.WindowReport, error)
	loadGlobalYearMonthly       func(time.Time) (stats.YearMonthlyReport, error)
	loadProjectYearMonthly      func(time.Time) (stats.YearMonthlyReport, error)
	globalMonthDaily            stats.MonthDailyReport
	projectMonthDaily           stats.MonthDailyReport
	globalMonthDailyLoaded      bool
	projectMonthDailyLoaded     bool
	globalMonthDailyLoading     bool
	projectMonthDailyLoading    bool
	globalMonthDailyMonth       time.Time
	projectMonthDailyMonth      time.Time
	globalMonthDailyUpdatedAt   time.Time
	projectMonthDailyUpdatedAt  time.Time
	loadGlobalMonthDaily        func(time.Time) (stats.MonthDailyReport, error)
	loadProjectMonthDaily       func(time.Time) (stats.MonthDailyReport, error)
	statsConfig                 config.StatsConfig
	projectScope                bool
	statsTab                    int
	statsMode                   bool
	dailyDetailMode             bool
	monthlyDetailMode           bool
	statsOffset                 int
	dailyMonthAnchor            time.Time
	dailySelectedDate           time.Time
	dailyListOffset             int
	dailyDetailOffset           int
	monthlySelectedMonth        time.Time
	monthlyListOffset           int
	monthlyDetailOffset         int
	globalDailyDate             time.Time
	projectDailyDate            time.Time
}

// Model holds the state of the multi-select TUI
type Model struct {
	plugins              []PluginItem
	editChoices          []EditChoice
	version              string
	allowMultiplePlugins bool
	sessions             []SessionItem
	session              SessionItem
	statsState
	cursor        int
	editCursor    int
	sessionCursor int
	selected      map[int]struct{}
	cancelled     bool
	confirmed     bool
	edit          bool
	editMode      bool
	sessionMode   bool
	editTarget    string
	width         int
	height        int
	sessionOffset int
}

type statsLoadedMsg struct {
	project bool
	report  stats.Report
	err     error
}

type windowReportLoadedMsg struct {
	project bool
	label   string
	start   time.Time
	end     time.Time
	report  stats.WindowReport
	err     error
}

type monthDailyReportLoadedMsg struct {
	project    bool
	monthStart time.Time
	report     stats.MonthDailyReport
	err        error
}

type yearMonthlyReportLoadedMsg struct {
	project  bool
	endMonth time.Time
	report   stats.YearMonthlyReport
	err      error
}

const sessionChromeHeight = 6
const statsViewTTL = 5 * time.Minute

type scrollTarget int

const (
	scrollTargetTop scrollTarget = iota
	scrollTargetBottom
)

var (
	defaultTextStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#7A7A7A"))
	statsValueTextStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	cursorStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Bold(true)
	cursorSelectedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Bold(true)
	helpBgKeyStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Background(lipgloss.Color("#191919")).Bold(true)
	helpBgTextStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#7A7A7A")).Background(lipgloss.Color("#191919"))
	helpBarStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("#191919"))
	helpBlockStyle          = lipgloss.NewStyle().Background(lipgloss.Color("#191919"))
	sessionContainerStyle   = lipgloss.NewStyle()
	sessionLabelStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#FF9900")).Bold(true).Padding(0, 1)
	sessionContentStyle     = lipgloss.NewStyle().Background(lipgloss.Color("#292929")).Padding(0, 1)
	sessionValueStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")).Background(lipgloss.Color("#292929")).Bold(false)
	sessionMetaStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Background(lipgloss.Color("#393939")).Bold(false).Padding(0, 1)
	habitSectionTitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")).Background(lipgloss.Color("#191919")).Bold(false).Padding(0, 1)
	todaySectionTitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")).Background(lipgloss.Color("#191919")).Bold(false).Bold(true).Padding(0, 1)
	instructionTitleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")).Background(lipgloss.Color("#292929")).Bold(false).Padding(0, 1)
	statsTabActiveStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Background(lipgloss.Color("#393939")).Bold(true).Padding(0, 1)
	statsTabStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Background(lipgloss.Color("#1F1F1F")).Padding(0, 1)
	statsTabIndicatorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#60C5F1"))
	statsTabMetaStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	dimmedLabelStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	sundayTextStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#C97373"))
	selectedSundayTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Bold(true)
)

func (m Model) renderTopBadge() string {
	targetWidth := m.layoutWidth()
	label := sessionLabelStyle.Render("OC")
	version := sessionContentStyle.Render(sessionValueStyle.Render(m.version))
	if m.isNarrowLayout() {
		return sessionContainerStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, label, version))
	}
	metaWidth := max(0, targetWidth-lipgloss.Width(label)-lipgloss.Width(version))
	metaText := selectedSessionSummary(m.session, max(0, metaWidth-2))
	return sessionContainerStyle.Render(lipgloss.JoinHorizontal(
		lipgloss.Top,
		label,
		version,
		sessionMetaStyle.Width(metaWidth).Render(metaText),
	))
}

var (
	sectionBarStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900"))
	instructionBarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#60C5F1"))
	detailSectionBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#60C5F1"))
)

func renderSectionHeader(text string, targetWidth int) string {
	bar := instructionBarStyle.Render("┃")
	barWidth := lipgloss.Width(bar)
	return bar + instructionTitleStyle.Width(max(0, targetWidth-barWidth)).Render(text)
}

func renderSubSectionHeader(text string, style lipgloss.Style) string {
	return "  " + sectionBarStyle.Render("┃") + style.Render(text)
}

func renderDetailSectionHeader(text string, style lipgloss.Style) string {
	return "  " + detailSectionBarStyle.Render("┃") + style.Render(text)
}

func selectedSessionSummary(session SessionItem, maxWidth int) string {
	if session.ID == "" {
		return "none"
	}

	prefix := sessionTimestampPrefix(session.UpdatedAt, time.Now())
	if session.Title == "" {
		return prefix + session.ID
	}
	suffix := " (" + session.ID + ")"
	availableTitleWidth := maxWidth - lipgloss.Width(prefix) - lipgloss.Width(suffix)
	title := session.Title
	if availableTitleWidth > 0 && lipgloss.Width(title) > availableTitleWidth {
		if isPathLike(title) {
			title = shortenPathMiddle(title, availableTitleWidth)
		} else if availableTitleWidth <= 3 {
			title = strings.Repeat(".", max(0, availableTitleWidth))
		} else {
			title = truncateString(title, availableTitleWidth-3) + "..."
		}
	}
	if availableTitleWidth <= 0 {
		return prefix + session.ID
	}
	return prefix + title + suffix
}

func truncateString(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

func stylePluginRow(line string, focused bool, selected bool) string {
	switch {
	case focused && selected:
		return cursorSelectedStyle.Render(line)
	case focused:
		return cursorStyle.Render(line)
	default:
		return defaultTextStyle.Render(line)
	}
}

func sessionTimestampPrefix(updatedAt time.Time, now time.Time) string {
	if updatedAt.IsZero() {
		return ""
	}

	localUpdated := updatedAt.Local()
	localNow := now.Local()
	updatedYear, updatedMonth, updatedDay := localUpdated.Date()
	nowYear, nowMonth, nowDay := localNow.Date()

	if updatedYear == nowYear && updatedMonth == nowMonth && updatedDay == nowDay {
		elapsed := max(localNow.Sub(localUpdated), 0)

		switch {
		case elapsed < time.Minute:
			return "[just now] "
		case elapsed < time.Hour:
			return "[" + strconv.Itoa(int(elapsed/time.Minute)) + "m ago] "
		default:
			return "[" + strconv.Itoa(int(elapsed/time.Hour)) + "h ago] "
		}
	}

	return "[" + localUpdated.Format("2006-01-02 15:04") + "] "
}

func pageStep(visibleRows int) int {
	if visibleRows <= 0 {
		return 1
	}
	return visibleRows
}

func halfPageStep(visibleRows int) int {
	step := visibleRows / 2
	if step < 1 {
		return 1
	}
	return step
}

func clampCursor(cursor int, total int) int {
	if total <= 0 {
		return 0
	}
	if cursor < 0 {
		return 0
	}
	if cursor >= total {
		return total - 1
	}
	return cursor
}

func jumpTarget(target scrollTarget, total int, visibleRows int) int {
	if total <= 0 {
		return 0
	}
	if target == scrollTargetTop {
		return 0
	}
	if visibleRows <= 0 || visibleRows >= total {
		return 0
	}
	return total - visibleRows
}

// NewModel creates a new TUI model with the given plugin items
func NewModel(items []PluginItem, editChoices []EditChoice, sessions []SessionItem, session SessionItem, globalStats stats.Report, projectStats stats.Report, statsConfig config.StatsConfig, version string, allowMultiplePlugins bool) Model {
	selected := make(map[int]struct{})
	for i, item := range items {
		if item.InitiallyEnabled {
			if !allowMultiplePlugins && len(selected) > 0 {
				continue
			}
			selected[i] = struct{}{}
		}
	}

	confirmed := false

	sessionCursor := 0
	for i, item := range sessions {
		if item.ID == session.ID {
			sessionCursor = i + 1
			break
		}
	}

	now := time.Now()
	normalizedStatsConfig := config.NormalizeStatsConfig(statsConfig)
	return Model{
		plugins:              items,
		editChoices:          editChoices,
		version:              version,
		allowMultiplePlugins: allowMultiplePlugins,
		sessions:             append([]SessionItem(nil), sessions...),
		session:              session,
		statsState: statsState{
			globalStats:          globalStats,
			projectStats:         projectStats,
			globalStatsLoaded:    true,
			projectStatsLoaded:   true,
			statsConfig:          normalizedStatsConfig,
			projectScope:         normalizedStatsConfig.DefaultScope == "project",
			dailyMonthAnchor:     statsMonthStart(now),
			dailySelectedDate:    startOfStatsDay(now),
			monthlySelectedMonth: statsMonthStart(now),
		},
		cursor:        0,
		editCursor:    0,
		sessionCursor: sessionCursor,
		selected:      selected,
		confirmed:     confirmed,
	}
}

// Init initializes the model and kicks off the launcher today-summary load.
func (m Model) Init() tea.Cmd {
	return m.loadCurrentScopeCmd()
}

func (m Model) WithStatsLoaders(global func() (stats.Report, error), project func() (stats.Report, error), globalWindow func(string, time.Time, time.Time) (stats.WindowReport, error), projectWindow func(string, time.Time, time.Time) (stats.WindowReport, error)) Model {
	m.loadGlobalStats = global
	m.loadProjectStats = project
	m.loadGlobalWindow = globalWindow
	m.loadProjectWindow = projectWindow
	m.globalStatsLoaded = false
	m.projectStatsLoaded = false
	return m
}

func (m Model) WithYearMonthlyLoaders(global func(time.Time) (stats.YearMonthlyReport, error), project func(time.Time) (stats.YearMonthlyReport, error)) Model {
	m.loadGlobalYearMonthly = global
	m.loadProjectYearMonthly = project
	m.globalYearMonthlyLoaded = false
	m.projectYearMonthlyLoaded = false
	return m
}

func (m Model) WithMonthDailyLoaders(global func(time.Time) (stats.MonthDailyReport, error), project func(time.Time) (stats.MonthDailyReport, error)) Model {
	m.loadGlobalMonthDaily = global
	m.loadProjectMonthDaily = project
	m.globalMonthDailyLoaded = false
	m.projectMonthDailyLoaded = false
	return m
}

func statsMonthStart(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func startOfStatsDay(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func (m *Model) resetDailyState(now time.Time) {
	month := statsMonthStart(now)
	m.dailyDetailMode = false
	m.dailyMonthAnchor = month
	m.dailySelectedDate = startOfStatsDay(now)
	m.dailyListOffset = 0
	m.dailyDetailOffset = 0
	m.statsOffset = 0
	if m.dailySelectedDate.Before(month) || !m.dailySelectedDate.Before(month.AddDate(0, 1, 0)) {
		m.dailySelectedDate = month
	}
}

func (m *Model) resetMonthlyState(now time.Time) {
	month := statsMonthStart(now)
	m.monthlyDetailMode = false
	m.monthlySelectedMonth = month
	m.monthlyListOffset = 0
	m.monthlyDetailOffset = 0
	m.statsOffset = 0
}

// Update handles state transitions based on messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if updated, cmd, handled := m.updateForAsyncMessage(msg); handled {
		return updated, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.updateForWindowSize(msg)
	case tea.KeyPressMsg:
		return m.updateForKey(msg)
	}
	return m, nil
}

// View renders the TUI
func (m Model) View() tea.View {
	if m.confirmed || m.cancelled || m.edit {
		return tea.NewView("")
	}

	switch {
	case m.statsMode:
		return tea.NewView(m.renderStatsView())
	case m.sessionMode:
		return m.viewSessionPicker()
	case m.editMode:
		return m.viewEditPicker()
	default:
		return m.viewLauncher()
	}
}

// Selections returns a map of plugin names to their selection state
func (m Model) Selections() map[string]bool {
	result := make(map[string]bool)
	for i, p := range m.plugins {
		_, isSelected := m.selected[i]
		result[p.Name] = isSelected
	}
	return result
}

// Cancelled returns true if the user cancelled the TUI
func (m Model) Cancelled() bool {
	return m.cancelled
}

// EditRequested returns true if the user chose to open the config in an editor.
func (m Model) EditRequested() bool {
	return m.edit
}

// EditTarget returns the selected config file path when edit was requested.
func (m Model) EditTarget() string {
	return m.editTarget
}

func (m Model) SelectedSession() SessionItem {
	return m.session
}
