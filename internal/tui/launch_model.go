package tui

import tea "charm.land/bubbletea/v2"

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
	ready    *LaunchReadyMsg
	executor launchExecutor
	msgCh    chan tea.Msg
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

func (m LaunchModel) Init() tea.Cmd {
	if m.executor == nil {
		return nil
	}

	return tea.Batch(
		func() tea.Msg {
			go m.executor(m.msgCh)
			return waitForLaunchMsg(m.msgCh)()
		},
	)
}

func (m LaunchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LaunchLogMsg:
		return m, waitForLaunchMsg(m.msgCh)
	case LaunchReadyMsg:
		ready := msg
		m.ready = &ready
		return m, tea.Quit
	case tea.KeyPressMsg:
		return m, nil
	}

	return m, nil
}

func (m LaunchModel) View() tea.View {
	return tea.NewView("")
}

func (m LaunchModel) PortArgs() []string {
	if m.ready == nil {
		return nil
	}

	return append([]string(nil), m.ready.PortArgs...)
}
