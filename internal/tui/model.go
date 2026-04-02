package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
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
