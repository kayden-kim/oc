package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	launchTitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true)
	launchLabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Bold(true)
	launchDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA4B2"))
	launchSpinStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true)
	launchLogNewStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Bold(true)
	launchLogMidStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#B7C0CC"))
	launchLogOldStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7A8594"))
)

var launchFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type launchTickMsg struct{}

type LaunchLogMsg struct {
	Line string
}

type LaunchReadyMsg struct {
	PortArgs []string
}

type launchExecutor func(chan<- tea.Msg)

type LaunchModel struct {
	version  string
	plugins  []string
	session  SessionItem
	logs     []string
	ready    *LaunchReadyMsg
	clearing bool
	executor launchExecutor
	msgCh    chan tea.Msg
	frame    int
}

func NewLaunchModel(plugins []string, session SessionItem, version string, executor launchExecutor) LaunchModel {
	return LaunchModel{
		version:  version,
		plugins:  append([]string(nil), plugins...),
		session:  session,
		executor: executor,
		msgCh:    make(chan tea.Msg, 32),
	}
}

func waitForLaunchMsg(msgCh <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-msgCh
		if !ok {
			return LaunchReadyMsg{}
		}
		return msg
	}
}

func launchTickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return launchTickMsg{}
	})
}

func styleLaunchLog(line string, index int, total int) string {
	switch {
	case total <= 1 || index == total-1:
		return launchLogNewStyle.Render(line)
	case index == total-2:
		return launchLogMidStyle.Render(line)
	default:
		return launchLogOldStyle.Render(line)
	}
}

func (m LaunchModel) Init() tea.Cmd {
	if m.executor == nil {
		return nil
	}

	return tea.Batch(
		func() tea.Msg {
			go m.executor(m.msgCh)
			return waitForLaunchMsg(m.msgCh)()
		},
		launchTickCmd(),
	)
}

func (m LaunchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LaunchLogMsg:
		m.logs = append(m.logs, msg.Line)
		return m, waitForLaunchMsg(m.msgCh)
	case LaunchReadyMsg:
		ready := msg
		m.ready = &ready
		m.clearing = true
		return m, tea.Quit
	case launchTickMsg:
		m.frame = (m.frame + 1) % len(launchFrames)
		return m, launchTickCmd()
	case tea.KeyPressMsg:
		return m, nil
	}

	return m, nil
}

func (m LaunchModel) View() tea.View {
	if m.clearing {
		return tea.NewView("")
	}

	var sections []string
	sections = append(sections, Model{version: m.version}.renderHeader())
	sections = append(sections, renderSelectedSession(m.session))
	sections = append(sections, launchTitleStyle.Render(fmt.Sprintf("%s Launching opencode", launchFrames[m.frame])))

	if len(m.plugins) == 0 {
		sections = append(sections, launchLabelStyle.Render("Plugins")+"\n"+launchDimStyle.Render("No selectable plugins in this view; continuing with the current configuration."))
	} else {
		pluginLines := []string{launchLabelStyle.Render("Plugins")}
		for _, plugin := range m.plugins {
			pluginLines = append(pluginLines, defaultTextStyle.Render(fmt.Sprintf("  - %s", plugin)))
		}
		sections = append(sections, strings.Join(pluginLines, "\n"))
	}

	if len(m.logs) > 0 {
		styledLogs := make([]string, len(m.logs))
		for i, line := range m.logs {
			styledLogs[i] = styleLaunchLog(line, i, len(m.logs))
		}
		sections = append(sections, launchLabelStyle.Render("Progress")+"\n"+strings.Join(styledLogs, "\n"))
	} else {
		sections = append(sections, launchLabelStyle.Render("Progress")+"\n"+launchSpinStyle.Render(launchFrames[m.frame])+" "+launchDimStyle.Render("Preparing launch..."))
	}

	return tea.NewView(strings.Join(sections, "\n\n"))
}

func (m LaunchModel) PortArgs() []string {
	if m.ready == nil {
		return nil
	}

	return append([]string(nil), m.ready.PortArgs...)
}
