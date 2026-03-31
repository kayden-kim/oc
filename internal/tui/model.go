package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
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

type SessionItem struct {
	ID        string
	Title     string
	UpdatedAt time.Time
}

// Model holds the state of the multi-select TUI
type Model struct {
	plugins                 []PluginItem
	editChoices             []EditChoice
	version                 string
	allowMultiplePlugins    bool
	sessions                []SessionItem
	session                 SessionItem
	globalStats             stats.Report
	projectStats            stats.Report
	globalStatsLoaded       bool
	projectStatsLoaded      bool
	globalStatsLoading      bool
	projectStatsLoading     bool
	globalStatsUpdatedAt    time.Time
	projectStatsUpdatedAt   time.Time
	loadGlobalStats         func() (stats.Report, error)
	loadProjectStats        func() (stats.Report, error)
	globalDaily             stats.WindowReport
	projectDaily            stats.WindowReport
	globalMonthly           stats.WindowReport
	projectMonthly          stats.WindowReport
	globalDailyLoaded       bool
	projectDailyLoaded      bool
	globalMonthlyLoaded     bool
	projectMonthlyLoaded    bool
	globalDailyLoading      bool
	projectDailyLoading     bool
	globalMonthlyLoading    bool
	projectMonthlyLoading   bool
	globalDailyUpdatedAt    time.Time
	projectDailyUpdatedAt   time.Time
	globalMonthlyUpdatedAt  time.Time
	projectMonthlyUpdatedAt time.Time
	loadGlobalWindow        func(string, time.Time, time.Time) (stats.WindowReport, error)
	loadProjectWindow       func(string, time.Time, time.Time) (stats.WindowReport, error)
	// Month-daily report caches (for month-list/day-detail view)
	globalMonthDaily           stats.MonthDailyReport
	projectMonthDaily          stats.MonthDailyReport
	globalMonthDailyLoaded     bool
	projectMonthDailyLoaded    bool
	globalMonthDailyLoading    bool
	projectMonthDailyLoading   bool
	globalMonthDailyMonth      time.Time // Track which month is cached
	projectMonthDailyMonth     time.Time // Track which month is cached
	globalMonthDailyUpdatedAt  time.Time
	projectMonthDailyUpdatedAt time.Time
	loadGlobalMonthDaily       func(time.Time) (stats.MonthDailyReport, error)
	loadProjectMonthDaily      func(time.Time) (stats.MonthDailyReport, error)
	statsConfig                config.StatsConfig
	projectScope               bool
	cursor                     int
	editCursor                 int
	sessionCursor              int
	statsTab                   int
	selected                   map[int]struct{}
	cancelled                  bool
	confirmed                  bool
	edit                       bool
	editMode                   bool
	sessionMode                bool
	statsMode                  bool
	editTarget                 string
	width                      int
	height                     int
	sessionOffset              int
	statsOffset                int
}

type statsLoadedMsg struct {
	project bool
	report  stats.Report
	err     error
}

type windowReportLoadedMsg struct {
	project bool
	label   string
	report  stats.WindowReport
	err     error
}

type monthDailyReportLoadedMsg struct {
	project    bool
	monthStart time.Time
	report     stats.MonthDailyReport
	err        error
}

const sessionChromeHeight = 6
const statsViewTTL = 5 * time.Minute

type scrollTarget int

const (
	scrollTargetTop scrollTarget = iota
	scrollTargetBottom
)

var (
	defaultTextStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#7A7A7A"))
	statsValueTextStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	cursorStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Bold(true)
	cursorSelectedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Bold(true)
	helpKeyStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true)
	helpBgKeyStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Background(lipgloss.Color("#191919")).Bold(true)
	helpBgTextStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#7A7A7A")).Background(lipgloss.Color("#191919"))
	helpBarStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("#191919"))
	helpBlockStyle         = lipgloss.NewStyle().Background(lipgloss.Color("#191919"))
	sessionContainerStyle  = lipgloss.NewStyle()
	sessionLabelStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#FF9900")).Bold(true).Padding(0, 1)
	sessionContentStyle    = lipgloss.NewStyle().Background(lipgloss.Color("#292929")).Padding(0, 1)
	sessionValueStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")).Background(lipgloss.Color("#292929")).Bold(false)
	sessionMetaStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Background(lipgloss.Color("#393939")).Bold(false).Padding(0, 1)
	habitSectionTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")).Background(lipgloss.Color("#191919")).Bold(false).Padding(0, 1)
	todaySectionTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")).Background(lipgloss.Color("#191919")).Bold(false).Bold(true).Padding(0, 1)
	instructionTitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")).Background(lipgloss.Color("#292929")).Bold(false).Padding(0, 1)
	statsTabActiveStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Background(lipgloss.Color("#393939")).Bold(true).Padding(0, 1)
	statsTabStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Background(lipgloss.Color("#1F1F1F")).Padding(0, 1)
	statsTabIndicatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#60C5F1"))
	statsTabMetaStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	dimmedLabelStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
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
	sectionBarStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900"))
	instructionBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#60C5F1"))
)

func renderSectionHeader(text string, targetWidth int) string {
	bar := instructionBarStyle.Render("┃")
	barWidth := lipgloss.Width(bar)
	return bar + instructionTitleStyle.Width(max(0, targetWidth-barWidth)).Render(text)
}

func renderSubSectionHeader(text string, style lipgloss.Style) string {
	return "  " + sectionBarStyle.Render("┃") + style.Render(text)
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

func renderHelpBlock(lines []string, targetWidth int) string {
	bar := helpBarStyle.Render("┃")
	barWidth := lipgloss.Width(bar)
	contentWidth := max(0, targetWidth-barWidth)
	rendered := make([]string, len(lines))
	for i, line := range lines {
		rendered[i] = bar + helpBlockStyle.Width(contentWidth).Render(" "+line)
	}
	return strings.Join(rendered, "\n")
}

func helpEntry(key string, action string) string {
	return helpBgKeyStyle.Render(key) + helpBgTextStyle.Render(": "+action)
}

func renderHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "navigate") + helpBgTextStyle.Render(" • ") + helpEntry("space", "toggle") + helpBgTextStyle.Render(" • ") + helpEntry("enter", "confirm") + helpBgTextStyle.Render(" • ") + helpEntry("q", "quit"),
		helpBgTextStyle.Render("   ") + helpEntry("tab", "stats") + helpBgTextStyle.Render(" • ") + helpEntry("g", "scope") + helpBgTextStyle.Render(" • ") + helpEntry("s", "sessions") + helpBgTextStyle.Render(" • ") + helpEntry("c", "config"),
	}, targetWidth)
}

func renderStatsHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "scroll") + helpBgTextStyle.Render(" • ") + helpEntry("PgUp/PgDn", "page") + helpBgTextStyle.Render(" • ") + helpEntry("Ctrl+U/D", "half") + helpBgTextStyle.Render(" • ") + helpEntry("Home/End", "top/bottom"),
		helpBgTextStyle.Render("   ") + helpEntry("tab", "launcher") + helpBgTextStyle.Render(" • ") + helpEntry("g", "scope") + helpBgTextStyle.Render(" • ") + helpEntry("←/→", "tabs") + helpBgTextStyle.Render(" • ") + helpEntry("esc", "back"),
	}, targetWidth)
}

func renderEditHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "navigate") + helpBgTextStyle.Render(" • ") + helpEntry("enter", "edit") + helpBgTextStyle.Render(" • ") + helpEntry("esc", "back"),
	}, targetWidth)
}

func renderSessionHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "navigate") + helpBgTextStyle.Render(" • ") + helpEntry("PgUp/PgDn", "page") + helpBgTextStyle.Render(" • ") + helpEntry("Ctrl+U/D", "half") + helpBgTextStyle.Render(" • ") + helpEntry("Home/End", "top/bottom"),
		helpBgTextStyle.Render("   ") + helpEntry("enter", "select") + helpBgTextStyle.Render(" • ") + helpEntry("esc", "back"),
	}, targetWidth)
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
		elapsed := localNow.Sub(localUpdated)
		if elapsed < 0 {
			elapsed = 0
		}

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

func sessionLine(session SessionItem) string {
	if session.ID == "" {
		return "Start without session"
	}

	prefix := sessionTimestampPrefix(session.UpdatedAt, time.Now())

	if session.Title == "" {
		return prefix + session.ID
	}

	return prefix + session.Title + " (" + session.ID + ")"
}

func (m Model) sessionAt(cursor int) SessionItem {
	if cursor <= 0 || cursor > len(m.sessions) {
		return SessionItem{}
	}

	return m.sessions[cursor-1]
}

func (m Model) availableSessionRows() int {
	if m.height <= 0 {
		return len(m.sessions) + 1
	}

	rows := m.height - sessionChromeHeight
	if rows < 0 {
		return 0
	}

	return rows
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

func scrollBy(offset int, delta int, total int, visibleRows int) int {
	if total <= 0 {
		return 0
	}
	if visibleRows <= 0 || visibleRows >= total {
		return 0
	}
	maxOffset := total - visibleRows
	offset += delta
	if offset < 0 {
		return 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
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

func (m *Model) ensureSessionCursorVisible() {
	totalRows := len(m.sessions) + 1
	if totalRows <= 0 {
		m.sessionOffset = 0
		return
	}

	visibleRows := m.availableSessionRows()
	if visibleRows <= 0 || visibleRows >= totalRows {
		m.sessionOffset = 0
		return
	}

	maxOffset := totalRows - visibleRows
	if m.sessionOffset > maxOffset {
		m.sessionOffset = maxOffset
	}
	if m.sessionOffset < 0 {
		m.sessionOffset = 0
	}

	if m.sessionCursor < m.sessionOffset {
		m.sessionOffset = m.sessionCursor
	}
	if m.sessionCursor >= m.sessionOffset+visibleRows {
		m.sessionOffset = m.sessionCursor - visibleRows + 1
	}

	if m.sessionOffset > maxOffset {
		m.sessionOffset = maxOffset
	}
	if m.sessionOffset < 0 {
		m.sessionOffset = 0
	}
}

func (m Model) visibleSessionRange() (int, int) {
	totalRows := len(m.sessions) + 1
	if totalRows <= 0 {
		return 0, 0
	}

	visibleRows := m.availableSessionRows()
	if visibleRows <= 0 {
		return 0, 0
	}
	if visibleRows >= totalRows {
		return 0, totalRows
	}

	start := m.sessionOffset
	if start < 0 {
		start = 0
	}
	maxOffset := totalRows - visibleRows
	if start > maxOffset {
		start = maxOffset
	}

	end := start + visibleRows
	if end > totalRows {
		end = totalRows
	}

	return start, end
}

func (m Model) availableStatsRows() int {
	if m.height <= 0 {
		return 1000
	}
	rows := m.height - m.statsChromeHeight()
	if rows < 0 {
		return 0
	}
	return rows
}

func (m Model) statsChromeHeight() int {
	if m.isNarrowLayout() {
		return 6
	}
	return 7
}

func (m Model) visibleStatsRange(total int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	visibleRows := m.availableStatsRows()
	if visibleRows <= 0 || visibleRows >= total {
		return 0, total
	}
	start := m.statsOffset
	if start < 0 {
		start = 0
	}
	maxOffset := total - visibleRows
	if start > maxOffset {
		start = maxOffset
	}
	end := start + visibleRows
	if end > total {
		end = total
	}
	return start, end
}

func (m *Model) scrollStats(delta int, total int) {
	if total <= 0 {
		m.statsOffset = 0
		return
	}
	visibleRows := m.availableStatsRows()
	if visibleRows <= 0 || visibleRows >= total {
		m.statsOffset = 0
		return
	}
	maxOffset := total - visibleRows
	m.statsOffset += delta
	if m.statsOffset < 0 {
		m.statsOffset = 0
	}
	if m.statsOffset > maxOffset {
		m.statsOffset = maxOffset
	}
}

func (m *Model) moveSessionCursor(delta int) {
	m.sessionCursor = clampCursor(m.sessionCursor+delta, len(m.sessions)+1)
	m.ensureSessionCursorVisible()
}

func (m *Model) pageSession(delta int) {
	m.moveSessionCursor(delta * pageStep(m.availableSessionRows()))
}

func (m *Model) halfPageSession(delta int) {
	m.moveSessionCursor(delta * halfPageStep(m.availableSessionRows()))
}

func (m *Model) jumpSessionTo(target scrollTarget) {
	total := len(m.sessions) + 1
	if target == scrollTargetTop {
		m.sessionCursor = 0
	} else {
		m.sessionCursor = total - 1
	}
	m.ensureSessionCursorVisible()
}

func (m *Model) pageStats(delta int, total int) {
	m.scrollStats(delta*pageStep(m.availableStatsRows()), total)
}

func (m *Model) halfPageStats(delta int, total int) {
	m.scrollStats(delta*halfPageStep(m.availableStatsRows()), total)
}

func (m *Model) jumpStatsTo(target scrollTarget, total int) {
	m.statsOffset = jumpTarget(target, total, m.availableStatsRows())
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

	// Empty list: auto-confirm immediately
	confirmed := len(items) == 0

	sessionCursor := 0
	for i, item := range sessions {
		if item.ID == session.ID {
			sessionCursor = i + 1
			break
		}
	}

	return Model{
		plugins:              items,
		editChoices:          editChoices,
		version:              version,
		allowMultiplePlugins: allowMultiplePlugins,
		sessions:             append([]SessionItem(nil), sessions...),
		session:              session,
		globalStats:          globalStats,
		projectStats:         projectStats,
		globalStatsLoaded:    true,
		projectStatsLoaded:   true,
		statsConfig:          NormalizeStatsConfig(statsConfig),
		projectScope:         NormalizeStatsConfig(statsConfig).DefaultScope == "project",
		cursor:               0,
		editCursor:           0,
		sessionCursor:        sessionCursor,
		selected:             selected,
		confirmed:            confirmed,
	}
}

// Init initializes the model (no initial command needed)
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

func (m Model) WithMonthDailyLoaders(global func(time.Time) (stats.MonthDailyReport, error), project func(time.Time) (stats.MonthDailyReport, error)) Model {
	m.loadGlobalMonthDaily = global
	m.loadProjectMonthDaily = project
	m.globalMonthDailyLoaded = false
	m.projectMonthDailyLoaded = false
	return m
}

func loadStatsCmd(project bool, loader func() (stats.Report, error)) tea.Cmd {
	if loader == nil {
		return nil
	}
	return func() tea.Msg {
		report, err := loader()
		return statsLoadedMsg{project: project, report: report, err: err}
	}
}

func loadWindowCmd(project bool, label string, loader func() (stats.WindowReport, error)) tea.Cmd {
	if loader == nil {
		return nil
	}
	return func() tea.Msg {
		report, err := loader()
		return windowReportLoadedMsg{project: project, label: label, report: report, err: err}
	}
}

func loadMonthDailyCmd(project bool, monthStart time.Time, loader func(time.Time) (stats.MonthDailyReport, error)) tea.Cmd {
	if loader == nil {
		return nil
	}
	return func() tea.Msg {
		report, err := loader(monthStart)
		return monthDailyReportLoadedMsg{project: project, monthStart: monthStart, report: report, err: err}
	}
}

func (m Model) loadCurrentScopeCmd() tea.Cmd {
	now := time.Now()
	if m.statsMode && m.statsTab > 0 {
		label, start, end := m.currentWindowSpec(now)
		return m.loadWindowReportCmd(label, start, end, now)
	}
	return m.loadOverviewCmd(now)
}

func (m Model) loadOverviewCmd(now time.Time) tea.Cmd {
	if m.projectScope {
		if (m.projectStatsLoaded && now.Sub(m.projectStatsUpdatedAt) < statsViewTTL) || m.projectStatsLoading || m.loadProjectStats == nil {
			return nil
		}
		m.projectStatsLoading = true
		return loadStatsCmd(true, m.loadProjectStats)
	}
	if (m.globalStatsLoaded && now.Sub(m.globalStatsUpdatedAt) < statsViewTTL) || m.globalStatsLoading || m.loadGlobalStats == nil {
		return nil
	}
	m.globalStatsLoading = true
	return loadStatsCmd(false, m.loadGlobalStats)
}

func (m Model) currentWindowSpec(now time.Time) (string, time.Time, time.Time) {
	if m.statsTab == 1 {
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return "Daily", start, start.AddDate(0, 0, 1)
	}
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	return "Monthly", start, start.AddDate(0, 1, 0)
}

func (m Model) loadWindowReportCmd(label string, start, end, now time.Time) tea.Cmd {
	if m.projectScope {
		if m.windowFresh(true, label, now) || m.windowLoading(true, label) || m.loadProjectWindow == nil {
			return nil
		}
		m.setWindowLoading(true, label, true)
		return loadWindowCmd(true, label, func() (stats.WindowReport, error) {
			return m.loadProjectWindow(label, start, end)
		})
	}
	if m.windowFresh(false, label, now) || m.windowLoading(false, label) || m.loadGlobalWindow == nil {
		return nil
	}
	m.setWindowLoading(false, label, true)
	return loadWindowCmd(false, label, func() (stats.WindowReport, error) {
		return m.loadGlobalWindow(label, start, end)
	})
}

func (m Model) windowFresh(project bool, label string, now time.Time) bool {
	switch {
	case project && label == "Daily":
		return m.projectDailyLoaded && now.Sub(m.projectDailyUpdatedAt) < statsViewTTL
	case project && label == "Monthly":
		return m.projectMonthlyLoaded && now.Sub(m.projectMonthlyUpdatedAt) < statsViewTTL
	case !project && label == "Daily":
		return m.globalDailyLoaded && now.Sub(m.globalDailyUpdatedAt) < statsViewTTL
	default:
		return m.globalMonthlyLoaded && now.Sub(m.globalMonthlyUpdatedAt) < statsViewTTL
	}
}

func (m Model) windowLoading(project bool, label string) bool {
	switch {
	case project && label == "Daily":
		return m.projectDailyLoading
	case project && label == "Monthly":
		return m.projectMonthlyLoading
	case !project && label == "Daily":
		return m.globalDailyLoading
	default:
		return m.globalMonthlyLoading
	}
}

func (m *Model) setWindowLoading(project bool, label string, loading bool) {
	switch {
	case project && label == "Daily":
		m.projectDailyLoading = loading
	case project && label == "Monthly":
		m.projectMonthlyLoading = loading
	case !project && label == "Daily":
		m.globalDailyLoading = loading
	default:
		m.globalMonthlyLoading = loading
	}
}

func (m *Model) setWindowReport(project bool, label string, report stats.WindowReport) {
	switch {
	case project && label == "Daily":
		m.projectDaily = report
		m.projectDailyLoaded = true
	case project && label == "Monthly":
		m.projectMonthly = report
		m.projectMonthlyLoaded = true
	case !project && label == "Daily":
		m.globalDaily = report
		m.globalDailyLoaded = true
	default:
		m.globalMonthly = report
		m.globalMonthlyLoaded = true
	}
}

func (m *Model) setWindowUpdatedAt(project bool, label string, updatedAt time.Time) {
	switch {
	case project && label == "Daily":
		m.projectDailyUpdatedAt = updatedAt
	case project && label == "Monthly":
		m.projectMonthlyUpdatedAt = updatedAt
	case !project && label == "Daily":
		m.globalDailyUpdatedAt = updatedAt
	default:
		m.globalMonthlyUpdatedAt = updatedAt
	}
}

// Update handles state transitions based on messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureSessionCursorVisible()
	case statsLoadedMsg:
		if msg.project {
			m.projectStatsLoading = false
			if msg.err == nil {
				m.projectStats = msg.report
				m.projectStatsLoaded = true
				m.projectStatsUpdatedAt = time.Now()
			}
			return m, nil
		}
		m.globalStatsLoading = false
		if msg.err == nil {
			m.globalStats = msg.report
			m.globalStatsLoaded = true
			m.globalStatsUpdatedAt = time.Now()
		}
		return m, nil
	case windowReportLoadedMsg:
		m.setWindowLoading(msg.project, msg.label, false)
		if msg.err == nil {
			m.setWindowReport(msg.project, msg.label, msg.report)
			m.setWindowUpdatedAt(msg.project, msg.label, time.Now())
		}
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "tab":
			if !m.editMode && !m.sessionMode {
				m.statsMode = !m.statsMode
				m.statsOffset = 0
				if m.statsMode {
					return m, m.loadCurrentScopeCmd()
				}
			}
		case "g":
			if !m.editMode && !m.sessionMode {
				m.projectScope = !m.projectScope
				m.statsOffset = 0
				return m, m.loadCurrentScopeCmd()
			}
		case "up", "k":
			if m.statsMode {
				m.scrollStats(-1, len(m.statsContentLines()))
				return m, nil
			}
			if m.sessionMode {
				m.moveSessionCursor(-1)
			} else if m.editMode {
				if m.editCursor > 0 {
					m.editCursor--
				}
			} else if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.statsMode {
				m.scrollStats(1, len(m.statsContentLines()))
				return m, nil
			}
			if m.sessionMode {
				m.moveSessionCursor(1)
			} else if m.editMode {
				if m.editCursor < len(m.editChoices)-1 {
					m.editCursor++
				}
			} else if m.cursor < len(m.plugins)-1 {
				m.cursor++
			}
		case "pgup":
			if m.statsMode {
				m.pageStats(-1, len(m.statsContentLines()))
				return m, nil
			}
			if m.sessionMode {
				m.pageSession(-1)
				return m, nil
			}
		case "pgdown":
			if m.statsMode {
				m.pageStats(1, len(m.statsContentLines()))
				return m, nil
			}
			if m.sessionMode {
				m.pageSession(1)
				return m, nil
			}
		case "ctrl+u":
			if m.statsMode {
				m.halfPageStats(-1, len(m.statsContentLines()))
				return m, nil
			}
			if m.sessionMode {
				m.halfPageSession(-1)
				return m, nil
			}
		case "ctrl+d":
			if m.statsMode {
				m.halfPageStats(1, len(m.statsContentLines()))
				return m, nil
			}
			if m.sessionMode {
				m.halfPageSession(1)
				return m, nil
			}
		case "home":
			if m.statsMode {
				m.jumpStatsTo(scrollTargetTop, len(m.statsContentLines()))
				return m, nil
			}
			if m.sessionMode {
				m.jumpSessionTo(scrollTargetTop)
				return m, nil
			}
		case "end":
			if m.statsMode {
				m.jumpStatsTo(scrollTargetBottom, len(m.statsContentLines()))
				return m, nil
			}
			if m.sessionMode {
				m.jumpSessionTo(scrollTargetBottom)
				return m, nil
			}
		case " ", "space": // Space key toggles selection (v2 uses "space")
			if !m.editMode && !m.sessionMode && !m.statsMode {
				if _, ok := m.selected[m.cursor]; ok {
					delete(m.selected, m.cursor)
				} else {
					if !m.allowMultiplePlugins {
						m.selected = map[int]struct{}{}
					}
					m.selected[m.cursor] = struct{}{}
				}
			}
		case "enter":
			if m.statsMode {
				return m, nil
			}
			if m.sessionMode {
				m.session = m.sessionAt(m.sessionCursor)
				m.sessionMode = false
				return m, nil
			}
			if m.editMode {
				m.edit = true
				m.editTarget = m.editChoices[m.editCursor].Path
				return m, tea.Quit
			}
			m.confirmed = true
			return m, tea.Quit
		case "left", "h":
			if m.statsMode && m.statsTab > 0 {
				m.statsTab--
				m.statsOffset = 0
				return m, m.loadCurrentScopeCmd()
			}
		case "right", "l":
			if m.statsMode && m.statsTab < len(statsTabTitles())-1 {
				m.statsTab++
				m.statsOffset = 0
				return m, m.loadCurrentScopeCmd()
			}
		case "s":
			if m.statsMode {
				return m, nil
			}
			m.sessionMode = true
			for i, item := range m.sessions {
				if item.ID == m.session.ID {
					m.sessionCursor = i + 1
					m.ensureSessionCursorVisible()
					return m, nil
				}
			}
			m.sessionCursor = 0
			m.ensureSessionCursorVisible()
		case "c":
			if !m.sessionMode && !m.statsMode && len(m.editChoices) > 0 {
				m.editMode = true
				m.editCursor = 0
			}
		case "ctrl+c", "q", "esc":
			if m.statsMode && msg.String() == "esc" {
				m.statsMode = false
				return m, nil
			}
			if m.sessionMode && (msg.String() == "q" || msg.String() == "esc") {
				m.sessionMode = false
				return m, nil
			}
			if m.editMode && (msg.String() == "q" || msg.String() == "esc") {
				m.editMode = false
				return m, nil
			}
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the TUI
func (m Model) View() tea.View {
	if m.confirmed || m.cancelled || m.edit {
		return tea.NewView("")
	}

	if len(m.plugins) == 0 {
		return tea.NewView("")
	}

	if m.editMode {
		s := m.renderTopBadge() + "\n\n" + renderSectionHeader("📂 Choose config to edit", m.layoutWidth()) + "\n\n"

		for i, choice := range m.editChoices {
			cursor := "  "
			focused := m.editCursor == i
			if focused {
				cursor = "> "
			}

			line := truncateDisplayWidth(fmt.Sprintf("%s%s", cursor, choice.Label), m.layoutWidth())
			line = stylePluginRow(line, focused, false)

			s += line + "\n"
		}

		s += "\n" + renderEditHelpLine(m.layoutWidth())

		return tea.NewView(s)
	}

	if m.sessionMode {
		s := m.renderTopBadge() + "\n\n" + renderSectionHeader("🕘 Choose session", m.layoutWidth()) + "\n\n"
		start, end := m.visibleSessionRange()

		for i := start; i < end; i++ {
			cursor := "  "
			focused := m.sessionCursor == i
			if focused {
				cursor = "> "
			}

			rowText := "Start without session"
			if item := m.sessionAt(i); item.ID != "" {
				rowText = selectedSessionSummary(item, max(0, m.layoutWidth()-lipgloss.Width(cursor)))
			}
			line := fmt.Sprintf("%s%s", cursor, rowText)
			line = truncateDisplayWidth(line, m.layoutWidth())
			line = stylePluginRow(line, focused, m.sessionAt(i).ID == m.session.ID)
			s += line + "\n"
		}

		s += "\n" + renderSessionHelpLine(m.layoutWidth())

		return tea.NewView(s)
	}

	if m.statsMode {
		return tea.NewView(m.renderStatsView())
	}

	sections := []string{m.renderLauncherAnalytics(), renderSectionHeader("📋 Choose plugins", m.layoutWidth())}
	s := m.renderTopBadge() + "\n\n" + strings.Join(filterNonEmpty(sections), "\n\n") + "\n\n"

	for i, p := range m.plugins {
		cursor := "  "
		focused := m.cursor == i
		if focused {
			cursor = "> "
		}

		checked := "   "
		_, selected := m.selected[i]
		if selected {
			checked = "✔  "
		}

		plainLine := fmt.Sprintf("%s%s%s", cursor, checked, p.Name)
		line := truncateDisplayWidth(plainLine, m.layoutWidth())
		if p.SourceLabel != "" {
			plainWithLabel := plainLine + " [" + p.SourceLabel + "]"
			if lipgloss.Width(plainWithLabel) <= m.layoutWidth() {
				line = plainLine + " " + dimmedLabelStyle.Render("["+p.SourceLabel+"]")
			} else {
				line = truncateDisplayWidth(plainWithLabel, m.layoutWidth())
			}
		}
		line = stylePluginRow(line, focused, selected)

		s += line + "\n"
	}

	s += "\n" + renderHelpLine(m.layoutWidth())

	return tea.NewView(s)
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
