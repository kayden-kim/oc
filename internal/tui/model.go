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
	plugins                     []PluginItem
	editChoices                 []EditChoice
	version                     string
	allowMultiplePlugins        bool
	sessions                    []SessionItem
	session                     SessionItem
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
	dailyDetailMode            bool
	monthlyDetailMode          bool
	editTarget                 string
	width                      int
	height                     int
	sessionOffset              int
	statsOffset                int
	dailyMonthAnchor           time.Time
	dailySelectedDate          time.Time
	dailyListOffset            int
	dailyDetailOffset          int
	monthlySelectedMonth       time.Time
	monthlyListOffset          int
	monthlyDetailOffset        int
	globalDailyDate            time.Time
	projectDailyDate           time.Time
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
	helpKeyStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true)
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
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "scroll") + helpBgTextStyle.Render(" • ") + helpEntry("pgup/pgdn", "page") + helpBgTextStyle.Render(" • ") + helpEntry("ctrl+u/d", "half") + helpBgTextStyle.Render(" • ") + helpEntry("home/end", "top/bottom"),
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
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "navigate") + helpBgTextStyle.Render(" • ") + helpEntry("pgup/pgdn", "page") + helpBgTextStyle.Render(" • ") + helpEntry("ctrl+u/d", "half") + helpBgTextStyle.Render(" • ") + helpEntry("home/end", "top/bottom"),
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

	now := time.Now()
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
		dailyMonthAnchor:     statsMonthStart(now),
		dailySelectedDate:    startOfStatsDay(now),
		monthlySelectedMonth: statsMonthStart(now),
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
	return loadWindowCmdWithRange(project, label, time.Time{}, time.Time{}, loader)
}

func loadWindowCmdWithRange(project bool, label string, start, end time.Time, loader func() (stats.WindowReport, error)) tea.Cmd {
	if loader == nil {
		return nil
	}
	return func() tea.Msg {
		report, err := loader()
		return windowReportLoadedMsg{project: project, label: label, start: start, end: end, report: report, err: err}
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

func loadYearMonthlyCmd(project bool, endMonth time.Time, loader func(time.Time) (stats.YearMonthlyReport, error)) tea.Cmd {
	if loader == nil {
		return nil
	}
	return func() tea.Msg {
		report, err := loader(endMonth)
		return yearMonthlyReportLoadedMsg{project: project, endMonth: endMonth, report: report, err: err}
	}
}

func (m Model) currentDailyMonth() time.Time {
	if !m.dailyMonthAnchor.IsZero() {
		return statsMonthStart(m.dailyMonthAnchor)
	}
	now := time.Now()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
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

func (m Model) currentDailyDate() time.Time {
	if !m.dailySelectedDate.IsZero() {
		return startOfStatsDay(m.dailySelectedDate)
	}
	month := m.currentDailyMonth()
	if !month.IsZero() {
		return month
	}
	return startOfStatsDay(time.Now())
}

func (m Model) currentDailyReportDate(project bool) time.Time {
	if project {
		return startOfStatsDay(m.projectDailyDate)
	}
	return startOfStatsDay(m.globalDailyDate)
}

func (m Model) currentMonthlySelection() time.Time {
	if !m.monthlySelectedMonth.IsZero() {
		return statsMonthStart(m.monthlySelectedMonth)
	}
	if report := m.currentYearMonthly(); len(report.Months) > 0 {
		return statsMonthStart(report.Months[len(report.Months)-1].MonthStart)
	}
	return statsMonthStart(time.Now())
}

func (m Model) currentYearMonthlyEnd() time.Time {
	return statsMonthStart(time.Now())
}

func (m Model) currentYearMonthly() stats.YearMonthlyReport {
	if m.projectScope {
		return m.projectYearMonthly
	}
	return m.globalYearMonthly
}

func (m Model) currentYearMonthlyLoading() bool {
	if m.projectScope {
		return m.projectYearMonthlyLoading
	}
	return m.globalYearMonthlyLoading
}

func (m Model) currentYearMonthlyFresh(now time.Time) bool {
	if m.projectScope {
		return m.projectYearMonthlyLoaded && now.Sub(m.projectYearMonthlyUpdatedAt) < statsViewTTL
	}
	return m.globalYearMonthlyLoaded && now.Sub(m.globalYearMonthlyUpdatedAt) < statsViewTTL
}

func (m *Model) setYearMonthlyLoading(project bool, loading bool) {
	if project {
		m.projectYearMonthlyLoading = loading
		return
	}
	m.globalYearMonthlyLoading = loading
}

func (m *Model) setYearMonthlyReport(project bool, report stats.YearMonthlyReport) {
	if project {
		m.projectYearMonthly = report
		m.projectYearMonthlyLoaded = true
		m.projectYearMonthlyUpdatedAt = time.Now()
		return
	}
	m.globalYearMonthly = report
	m.globalYearMonthlyLoaded = true
	m.globalYearMonthlyUpdatedAt = time.Now()
}

func (m Model) yearMonthlyRows() []stats.MonthlySummary {
	report := m.currentYearMonthly()
	rows := make([]stats.MonthlySummary, 0, len(report.Months))
	for i := len(report.Months) - 1; i >= 0; i-- {
		rows = append(rows, report.Months[i])
	}
	return rows
}

func (m Model) monthlySelectionIndex() int {
	rows := m.yearMonthlyRows()
	if len(rows) == 0 {
		return -1
	}
	selected := m.currentMonthlySelection()
	for i, row := range rows {
		if statsMonthStart(row.MonthStart).Equal(selected) {
			return i
		}
	}
	return 0
}

func (m *Model) syncMonthlySelectionToYear() {
	rows := m.yearMonthlyRows()
	if len(rows) == 0 {
		m.monthlySelectedMonth = time.Time{}
		return
	}
	idx := m.monthlySelectionIndex()
	if idx < 0 || idx >= len(rows) {
		idx = 0
	}
	m.monthlySelectedMonth = statsMonthStart(rows[idx].MonthStart)
}

func (m *Model) ensureMonthlySelectionVisible() {
	total := len(m.statsContentLines())
	if total <= 0 {
		m.monthlyListOffset = 0
		m.statsOffset = 0
		return
	}
	idx := m.monthlySelectionIndex()
	if idx < 0 {
		m.monthlyListOffset = 0
		m.statsOffset = 0
		return
	}
	lineIndex := m.yearMonthlyRowLineOffset() + idx
	visibleRows := m.availableStatsRows()
	if visibleRows <= 0 || visibleRows >= total {
		m.monthlyListOffset = 0
		m.statsOffset = 0
		return
	}
	if m.monthlyListOffset > total-visibleRows {
		m.monthlyListOffset = max(0, total-visibleRows)
	}
	if lineIndex < m.monthlyListOffset {
		m.monthlyListOffset = lineIndex
	}
	if lineIndex >= m.monthlyListOffset+visibleRows {
		m.monthlyListOffset = lineIndex - visibleRows + 1
	}
	if m.monthlyListOffset < 0 {
		m.monthlyListOffset = 0
	}
	if maxOffset := max(0, total-visibleRows); m.monthlyListOffset > maxOffset {
		m.monthlyListOffset = maxOffset
	}
	m.statsOffset = m.monthlyListOffset
}

func (m Model) yearMonthlyRowLineOffset() int {
	if m.isNarrowLayout() {
		return 3
	}
	return 13
}

func (m *Model) moveMonthlySelection(delta int) {
	rows := m.yearMonthlyRows()
	if len(rows) == 0 {
		return
	}
	idx := m.monthlySelectionIndex()
	if idx < 0 {
		idx = 0
	}
	idx = clampCursor(idx+delta, len(rows))
	m.monthlySelectedMonth = statsMonthStart(rows[idx].MonthStart)
	m.ensureMonthlySelectionVisible()
}

func (m *Model) jumpMonthlySelection(target scrollTarget) {
	rows := m.yearMonthlyRows()
	if len(rows) == 0 {
		return
	}
	idx := 0
	if target == scrollTargetBottom {
		idx = len(rows) - 1
	}
	m.monthlySelectedMonth = statsMonthStart(rows[idx].MonthStart)
	m.ensureMonthlySelectionVisible()
}

func (m *Model) pageMonthlySelection(delta int) {
	m.moveMonthlySelection(delta * pageStep(m.availableStatsRows()))
}

func (m *Model) halfPageMonthlySelection(delta int) {
	m.moveMonthlySelection(delta * halfPageStep(m.availableStatsRows()))
}

func (m *Model) enterMonthlyDetail() {
	m.syncMonthlySelectionToYear()
	m.monthlyDetailMode = true
	m.monthlyDetailOffset = 0
	m.statsOffset = 0
}

func (m *Model) exitMonthlyDetail() {
	m.monthlyDetailMode = false
	m.statsOffset = m.monthlyListOffset
	m.ensureMonthlySelectionVisible()
}

func (m Model) currentMonthDailyFresh(month, now time.Time) bool {
	month = statsMonthStart(month)
	if m.projectScope {
		return m.projectMonthDailyLoaded && statsMonthStart(m.projectMonthDailyMonth).Equal(month) && now.Sub(m.projectMonthDailyUpdatedAt) < statsViewTTL
	}
	return m.globalMonthDailyLoaded && statsMonthStart(m.globalMonthDailyMonth).Equal(month) && now.Sub(m.globalMonthDailyUpdatedAt) < statsViewTTL
}

func (m *Model) setMonthDailyLoading(project bool, loading bool) {
	if project {
		m.projectMonthDailyLoading = loading
		return
	}
	m.globalMonthDailyLoading = loading
}

func (m *Model) setMonthDailyReport(project bool, month time.Time, report stats.MonthDailyReport) {
	month = statsMonthStart(month)
	if project {
		m.projectMonthDaily = report
		m.projectMonthDailyLoaded = true
		m.projectMonthDailyMonth = month
		m.projectMonthDailyUpdatedAt = time.Now()
		return
	}
	m.globalMonthDaily = report
	m.globalMonthDailyLoaded = true
	m.globalMonthDailyMonth = month
	m.globalMonthDailyUpdatedAt = time.Now()
}

func (m Model) monthDailyRows() []stats.DailySummary {
	return m.currentMonthDaily().Days
}

func (m Model) dailySelectionIndex() int {
	rows := m.monthDailyRows()
	if len(rows) == 0 {
		return -1
	}
	selected := m.currentDailyDate()
	for i, row := range rows {
		if startOfStatsDay(row.Date).Equal(selected) {
			return i
		}
	}
	return 0
}

func (m *Model) syncDailySelectionToMonth() {
	rows := m.monthDailyRows()
	if len(rows) == 0 {
		m.dailySelectedDate = time.Time{}
		return
	}
	idx := m.dailySelectionIndex()
	if idx < 0 || idx >= len(rows) {
		idx = 0
	}
	m.dailySelectedDate = startOfStatsDay(rows[idx].Date)
}

func (m *Model) ensureDailySelectionVisible() {
	total := len(m.statsContentLines())
	if total <= 0 {
		m.dailyListOffset = 0
		m.statsOffset = 0
		return
	}
	idx := m.dailySelectionIndex()
	if idx < 0 {
		m.dailyListOffset = 0
		m.statsOffset = 0
		return
	}
	lineIndex := m.monthDailyRowLineOffset() + idx
	visibleRows := m.availableStatsRows()
	if visibleRows <= 0 || visibleRows >= total {
		m.dailyListOffset = 0
		m.statsOffset = 0
		return
	}
	if m.dailyListOffset > total-visibleRows {
		m.dailyListOffset = max(0, total-visibleRows)
	}
	if lineIndex < m.dailyListOffset {
		m.dailyListOffset = lineIndex
	}
	if lineIndex >= m.dailyListOffset+visibleRows {
		m.dailyListOffset = lineIndex - visibleRows + 1
	}
	if m.dailyListOffset < 0 {
		m.dailyListOffset = 0
	}
	if maxOffset := max(0, total-visibleRows); m.dailyListOffset > maxOffset {
		m.dailyListOffset = maxOffset
	}
	m.statsOffset = m.dailyListOffset
}

func (m Model) monthDailyRowLineOffset() int {
	if m.isNarrowLayout() {
		return 4
	}
	return 4
}

func (m *Model) moveDailySelection(delta int) {
	rows := m.monthDailyRows()
	if len(rows) == 0 {
		return
	}
	idx := m.dailySelectionIndex()
	if idx < 0 {
		idx = 0
	}
	idx = clampCursor(idx+delta, len(rows))
	m.dailySelectedDate = startOfStatsDay(rows[idx].Date)
	m.ensureDailySelectionVisible()
}

func (m *Model) jumpDailySelection(target scrollTarget) {
	rows := m.monthDailyRows()
	if len(rows) == 0 {
		return
	}
	idx := 0
	if target == scrollTargetBottom {
		idx = len(rows) - 1
	}
	m.dailySelectedDate = startOfStatsDay(rows[idx].Date)
	m.ensureDailySelectionVisible()
}

func (m *Model) pageDailySelection(delta int) {
	m.moveDailySelection(delta * pageStep(m.availableStatsRows()))
}

func (m *Model) halfPageDailySelection(delta int) {
	m.moveDailySelection(delta * halfPageStep(m.availableStatsRows()))
}

func (m *Model) enterDailyDetail() {
	m.syncDailySelectionToMonth()
	m.dailyDetailMode = true
	m.dailyDetailOffset = 0
	m.statsOffset = 0
}

func (m *Model) exitDailyDetail() {
	m.dailyDetailMode = false
	m.statsOffset = m.dailyListOffset
	m.ensureDailySelectionVisible()
}

func (m *Model) navigateDailyMonth(delta int) {
	month := m.currentDailyMonth()
	if month.IsZero() {
		month = statsMonthStart(time.Now())
	}
	currentMonth := statsMonthStart(time.Now())
	selected := m.currentDailyDate()
	preferredDay := selected.Day()
	newMonth := month.AddDate(0, delta, 0)
	if newMonth.After(currentMonth) {
		newMonth = currentMonth
	}
	lastDay := newMonth.AddDate(0, 1, -1).Day()
	if preferredDay > lastDay {
		preferredDay = lastDay
	}
	m.dailyMonthAnchor = newMonth
	m.dailySelectedDate = time.Date(newMonth.Year(), newMonth.Month(), preferredDay, 0, 0, 0, 0, newMonth.Location())
	m.dailyListOffset = 0
	m.statsOffset = 0
}

func (m Model) loadCurrentScopeCmd() tea.Cmd {
	now := time.Now()
	if m.statsMode && m.statsTab > 0 {
		if m.statsTab == 1 && !m.dailyDetailMode {
			return m.loadMonthDailyReportCmd(m.currentDailyMonth(), now)
		}
		if m.statsTab == 2 && !m.monthlyDetailMode {
			return m.loadYearMonthlyReportCmd(m.currentYearMonthlyEnd(), now)
		}
		label, start, end := m.currentWindowSpec(now)
		if m.statsTab == 2 && m.monthlyDetailMode {
			return tea.Batch(
				m.loadMonthDailyReportCmd(m.currentMonthlySelection(), now),
				m.loadWindowReportCmd(label, start, end, now),
			)
		}
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
		date := m.currentDailyDate()
		if date.IsZero() {
			date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		}
		start := startOfStatsDay(date)
		return "Daily", start, start.AddDate(0, 0, 1)
	}
	start := m.currentMonthlySelection()
	if start.IsZero() {
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}
	return "Monthly", start, start.AddDate(0, 1, 0)
}

func (m Model) loadYearMonthlyReportCmd(endMonth, now time.Time) tea.Cmd {
	endMonth = statsMonthStart(endMonth)
	if m.projectScope {
		if m.currentYearMonthlyFresh(now) || m.projectYearMonthlyLoading || m.loadProjectYearMonthly == nil {
			return nil
		}
		m.setYearMonthlyLoading(true, true)
		return loadYearMonthlyCmd(true, endMonth, m.loadProjectYearMonthly)
	}
	if m.currentYearMonthlyFresh(now) || m.globalYearMonthlyLoading || m.loadGlobalYearMonthly == nil {
		return nil
	}
	m.setYearMonthlyLoading(false, true)
	return loadYearMonthlyCmd(false, endMonth, m.loadGlobalYearMonthly)
}

func (m Model) loadMonthDailyReportCmd(monthStart, now time.Time) tea.Cmd {
	monthStart = statsMonthStart(monthStart)
	if m.projectScope {
		if m.currentMonthDailyFresh(monthStart, now) || m.projectMonthDailyLoading || m.loadProjectMonthDaily == nil {
			return nil
		}
		m.setMonthDailyLoading(true, true)
		return loadMonthDailyCmd(true, monthStart, m.loadProjectMonthDaily)
	}
	if m.currentMonthDailyFresh(monthStart, now) || m.globalMonthDailyLoading || m.loadGlobalMonthDaily == nil {
		return nil
	}
	m.setMonthDailyLoading(false, true)
	return loadMonthDailyCmd(false, monthStart, m.loadGlobalMonthDaily)
}

func (m Model) loadWindowReportCmd(label string, start, end, now time.Time) tea.Cmd {
	if m.projectScope {
		if m.windowFresh(true, label, now) || m.windowLoading(true, label) || m.loadProjectWindow == nil {
			return nil
		}
		m.setWindowLoading(true, label, true)
		return loadWindowCmdWithRange(true, label, start, end, func() (stats.WindowReport, error) {
			return m.loadProjectWindow(label, start, end)
		})
	}
	if m.windowFresh(false, label, now) || m.windowLoading(false, label) || m.loadGlobalWindow == nil {
		return nil
	}
	m.setWindowLoading(false, label, true)
	return loadWindowCmdWithRange(false, label, start, end, func() (stats.WindowReport, error) {
		return m.loadGlobalWindow(label, start, end)
	})
}

func (m Model) windowFresh(project bool, label string, now time.Time) bool {
	switch {
	case project && label == "Daily":
		return m.projectDailyLoaded && startOfStatsDay(m.projectDailyDate).Equal(m.currentDailyDate()) && now.Sub(m.projectDailyUpdatedAt) < statsViewTTL
	case project && label == "Monthly":
		return m.projectMonthlyLoaded && statsMonthStart(m.projectMonthly.Start).Equal(m.currentMonthlySelection()) && now.Sub(m.projectMonthlyUpdatedAt) < statsViewTTL
	case !project && label == "Daily":
		return m.globalDailyLoaded && startOfStatsDay(m.globalDailyDate).Equal(m.currentDailyDate()) && now.Sub(m.globalDailyUpdatedAt) < statsViewTTL
	default:
		return m.globalMonthlyLoaded && statsMonthStart(m.globalMonthly.Start).Equal(m.currentMonthlySelection()) && now.Sub(m.globalMonthlyUpdatedAt) < statsViewTTL
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
		m.projectDailyDate = startOfStatsDay(report.Start)
	case project && label == "Monthly":
		m.projectMonthly = report
		m.projectMonthlyLoaded = true
	case !project && label == "Daily":
		m.globalDaily = report
		m.globalDailyLoaded = true
		m.globalDailyDate = startOfStatsDay(report.Start)
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
			if msg.label == "Daily" {
				if m.dailyDetailMode == false {
					return m, nil
				}
				if msg.project != m.projectScope || !startOfStatsDay(msg.start).Equal(m.currentDailyDate()) {
					return m, nil
				}
			}
			m.setWindowReport(msg.project, msg.label, msg.report)
			m.setWindowUpdatedAt(msg.project, msg.label, time.Now())
		}
		return m, nil
	case monthDailyReportLoadedMsg:
		m.setMonthDailyLoading(msg.project, false)
		if msg.err == nil {
			expectedMonth := m.currentDailyMonth()
			if m.statsTab == 2 && m.monthlyDetailMode {
				expectedMonth = m.currentMonthlySelection()
			}
			if msg.project != m.projectScope || !statsMonthStart(msg.monthStart).Equal(statsMonthStart(expectedMonth)) {
				return m, nil
			}
			m.setMonthDailyReport(msg.project, msg.monthStart, msg.report)
			if m.statsTab == 1 && !m.dailyDetailMode {
				m.syncDailySelectionToMonth()
				m.ensureDailySelectionVisible()
			}
		}
		return m, nil
	case yearMonthlyReportLoadedMsg:
		m.setYearMonthlyLoading(msg.project, false)
		if msg.err == nil {
			if msg.project != m.projectScope {
				return m, nil
			}
			m.setYearMonthlyReport(msg.project, msg.report)
			m.syncMonthlySelectionToYear()
			m.ensureMonthlySelectionVisible()
		}
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "tab":
			if !m.editMode && !m.sessionMode {
				m.statsMode = !m.statsMode
				if m.statsMode {
					m.resetDailyState(time.Now())
					m.resetMonthlyState(time.Now())
					return m, m.loadCurrentScopeCmd()
				}
				m.statsOffset = 0
			}
		case "g":
			if !m.editMode && !m.sessionMode {
				m.projectScope = !m.projectScope
				if m.statsTab == 1 && !m.dailyDetailMode {
					m.statsOffset = m.dailyListOffset
				} else if m.statsTab == 1 {
					m.statsOffset = m.dailyDetailOffset
				} else if m.statsTab == 2 && !m.monthlyDetailMode {
					m.statsOffset = m.monthlyListOffset
				} else if m.statsTab == 2 {
					m.statsOffset = m.monthlyDetailOffset
				} else {
					m.statsOffset = 0
				}
				return m, m.loadCurrentScopeCmd()
			}
		case "up", "k":
			if m.statsMode {
				if m.statsTab == 1 && !m.dailyDetailMode {
					m.moveDailySelection(-1)
					return m, nil
				}
				if m.statsTab == 2 && !m.monthlyDetailMode {
					m.moveMonthlySelection(-1)
					return m, nil
				}
				m.scrollStats(-1, len(m.statsContentLines()))
				if m.statsTab == 1 && m.dailyDetailMode {
					m.dailyDetailOffset = m.statsOffset
				} else if m.statsTab == 2 && m.monthlyDetailMode {
					m.monthlyDetailOffset = m.statsOffset
				}
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
				if m.statsTab == 1 && !m.dailyDetailMode {
					m.moveDailySelection(1)
					return m, nil
				}
				if m.statsTab == 2 && !m.monthlyDetailMode {
					m.moveMonthlySelection(1)
					return m, nil
				}
				m.scrollStats(1, len(m.statsContentLines()))
				if m.statsTab == 1 && m.dailyDetailMode {
					m.dailyDetailOffset = m.statsOffset
				} else if m.statsTab == 2 && m.monthlyDetailMode {
					m.monthlyDetailOffset = m.statsOffset
				}
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
				if m.statsTab == 1 && !m.dailyDetailMode {
					m.pageDailySelection(-1)
					return m, nil
				}
				if m.statsTab == 2 && !m.monthlyDetailMode {
					m.pageMonthlySelection(-1)
					return m, nil
				}
				m.pageStats(-1, len(m.statsContentLines()))
				if m.statsTab == 1 && m.dailyDetailMode {
					m.dailyDetailOffset = m.statsOffset
				} else if m.statsTab == 2 && m.monthlyDetailMode {
					m.monthlyDetailOffset = m.statsOffset
				}
				return m, nil
			}
			if m.sessionMode {
				m.pageSession(-1)
				return m, nil
			}
		case "pgdown":
			if m.statsMode {
				if m.statsTab == 1 && !m.dailyDetailMode {
					m.pageDailySelection(1)
					return m, nil
				}
				if m.statsTab == 2 && !m.monthlyDetailMode {
					m.pageMonthlySelection(1)
					return m, nil
				}
				m.pageStats(1, len(m.statsContentLines()))
				if m.statsTab == 1 && m.dailyDetailMode {
					m.dailyDetailOffset = m.statsOffset
				} else if m.statsTab == 2 && m.monthlyDetailMode {
					m.monthlyDetailOffset = m.statsOffset
				}
				return m, nil
			}
			if m.sessionMode {
				m.pageSession(1)
				return m, nil
			}
		case "ctrl+u":
			if m.statsMode {
				if m.statsTab == 1 && !m.dailyDetailMode {
					m.halfPageDailySelection(-1)
					return m, nil
				}
				if m.statsTab == 2 && !m.monthlyDetailMode {
					m.halfPageMonthlySelection(-1)
					return m, nil
				}
				m.halfPageStats(-1, len(m.statsContentLines()))
				if m.statsTab == 1 && m.dailyDetailMode {
					m.dailyDetailOffset = m.statsOffset
				} else if m.statsTab == 2 && m.monthlyDetailMode {
					m.monthlyDetailOffset = m.statsOffset
				}
				return m, nil
			}
			if m.sessionMode {
				m.halfPageSession(-1)
				return m, nil
			}
		case "ctrl+d":
			if m.statsMode {
				if m.statsTab == 1 && !m.dailyDetailMode {
					m.halfPageDailySelection(1)
					return m, nil
				}
				if m.statsTab == 2 && !m.monthlyDetailMode {
					m.halfPageMonthlySelection(1)
					return m, nil
				}
				m.halfPageStats(1, len(m.statsContentLines()))
				if m.statsTab == 1 && m.dailyDetailMode {
					m.dailyDetailOffset = m.statsOffset
				} else if m.statsTab == 2 && m.monthlyDetailMode {
					m.monthlyDetailOffset = m.statsOffset
				}
				return m, nil
			}
			if m.sessionMode {
				m.halfPageSession(1)
				return m, nil
			}
		case "home":
			if m.statsMode {
				if m.statsTab == 1 && !m.dailyDetailMode {
					m.jumpDailySelection(scrollTargetTop)
					return m, nil
				}
				if m.statsTab == 2 && !m.monthlyDetailMode {
					m.jumpMonthlySelection(scrollTargetTop)
					return m, nil
				}
				m.jumpStatsTo(scrollTargetTop, len(m.statsContentLines()))
				if m.statsTab == 1 && m.dailyDetailMode {
					m.dailyDetailOffset = m.statsOffset
				} else if m.statsTab == 2 && m.monthlyDetailMode {
					m.monthlyDetailOffset = m.statsOffset
				}
				return m, nil
			}
			if m.sessionMode {
				m.jumpSessionTo(scrollTargetTop)
				return m, nil
			}
		case "end":
			if m.statsMode {
				if m.statsTab == 1 && !m.dailyDetailMode {
					m.jumpDailySelection(scrollTargetBottom)
					return m, nil
				}
				if m.statsTab == 2 && !m.monthlyDetailMode {
					m.jumpMonthlySelection(scrollTargetBottom)
					return m, nil
				}
				m.jumpStatsTo(scrollTargetBottom, len(m.statsContentLines()))
				if m.statsTab == 1 && m.dailyDetailMode {
					m.dailyDetailOffset = m.statsOffset
				} else if m.statsTab == 2 && m.monthlyDetailMode {
					m.monthlyDetailOffset = m.statsOffset
				}
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
				if m.statsTab == 1 && !m.dailyDetailMode {
					m.enterDailyDetail()
					return m, m.loadCurrentScopeCmd()
				}
				if m.statsTab == 2 && !m.monthlyDetailMode {
					m.enterMonthlyDetail()
					return m, m.loadCurrentScopeCmd()
				}
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
		case "[":
			if m.statsMode && m.statsTab == 1 && !m.dailyDetailMode {
				m.navigateDailyMonth(-1)
				return m, m.loadCurrentScopeCmd()
			}
		case "]":
			if m.statsMode && m.statsTab == 1 && !m.dailyDetailMode {
				m.navigateDailyMonth(1)
				return m, m.loadCurrentScopeCmd()
			}
		case "left", "h":
			if m.statsMode && m.statsTab > 0 {
				m.statsTab--
				if m.statsTab == 1 && !m.dailyDetailMode {
					m.statsOffset = m.dailyListOffset
				} else if m.statsTab == 1 {
					m.statsOffset = m.dailyDetailOffset
				} else if m.statsTab == 2 && !m.monthlyDetailMode {
					m.statsOffset = m.monthlyListOffset
				} else if m.statsTab == 2 {
					m.statsOffset = m.monthlyDetailOffset
				} else {
					m.statsOffset = 0
				}
				return m, m.loadCurrentScopeCmd()
			}
		case "right", "l":
			if m.statsMode && m.statsTab < len(statsTabTitles())-1 {
				m.statsTab++
				if m.statsTab == 1 && !m.dailyDetailMode {
					m.statsOffset = m.dailyListOffset
				} else if m.statsTab == 1 {
					m.statsOffset = m.dailyDetailOffset
				} else if m.statsTab == 2 && !m.monthlyDetailMode {
					m.statsOffset = m.monthlyListOffset
				} else if m.statsTab == 2 {
					m.statsOffset = m.monthlyDetailOffset
				} else {
					m.statsOffset = 0
				}
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
				if m.statsTab == 1 && m.dailyDetailMode {
					m.exitDailyDetail()
					return m, nil
				}
				if m.statsTab == 2 && m.monthlyDetailMode {
					m.exitMonthlyDetail()
					return m, nil
				}
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
