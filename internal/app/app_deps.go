package app

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/editor"
	"github.com/kayden-kim/oc/internal/launch"
	"github.com/kayden-kim/oc/internal/plugin"
	"github.com/kayden-kim/oc/internal/port"
	"github.com/kayden-kim/oc/internal/runner"
	"github.com/kayden-kim/oc/internal/session"
	"github.com/kayden-kim/oc/internal/stats"
	"github.com/kayden-kim/oc/internal/tui"
)

type RunnerAPI interface {
	CheckAvailable() error
	Run(args []string, onStart func(context.Context)) error
}

type RuntimeDeps struct {
	Version                string
	NewRunner              func() RunnerAPI
	UserHomeDir            func() (string, error)
	ReadFile               func(string) ([]byte, error)
	LoadOcConfig           func(string) (*config.OcConfig, error)
	ParsePlugins           func([]byte) ([]config.Plugin, string, error)
	FilterByWhitelist      func([]config.Plugin, []string) ([]config.Plugin, []config.Plugin)
	Getwd                  func() (string, error)
	ListSessions           func(string) ([]tui.SessionItem, error)
	LoadGlobalStats        func(config.StatsConfig) (stats.Report, error)
	LoadProjectStats       func(string, config.StatsConfig) (stats.Report, error)
	LoadGlobalWindow       func(string, time.Time, time.Time) (stats.WindowReport, error)
	LoadProjectWindow      func(string, string, time.Time, time.Time) (stats.WindowReport, error)
	LoadGlobalYearMonthly  func(time.Time) (stats.YearMonthlyReport, error)
	LoadProjectYearMonthly func(string, time.Time) (stats.YearMonthlyReport, error)
	LoadGlobalMonthDaily   func(time.Time) (stats.MonthDailyReport, error)
	LoadProjectMonthDaily  func(string, time.Time) (stats.MonthDailyReport, error)
	RunTUI                 func([]tui.PluginItem, []tui.EditChoice, []tui.SessionItem, tui.SessionItem, stats.Report, stats.Report, config.StatsConfig, string, bool) (map[string]bool, bool, string, []string, tui.SessionItem, error)
	ApplySelections        func([]byte, map[string]bool) ([]byte, error)
	WriteConfigFile        func(string, []byte) error
	OpenEditor             func(string, string) error
	ParsePortRange         func(string) (int, int, error)
	SelectPort             func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, port int, available bool)) port.SelectResult
	IsPortAvailable        func(int) bool
	SendToast              func(context.Context, int, []string) error
}

const DefaultPortsRange = "55500-55555"

func DefaultDeps(version string) RuntimeDeps {
	deps := RuntimeDeps{
		Version:           version,
		NewRunner:         func() RunnerAPI { return runner.NewRunner() },
		UserHomeDir:       os.UserHomeDir,
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: plugin.FilterByWhitelist,
		Getwd:             os.Getwd,
		ListSessions:      session.List,
		LoadGlobalStats: func(statsConfig config.StatsConfig) (stats.Report, error) {
			return stats.LoadGlobalWithOptions(statsOptions(statsConfig))
		},
		LoadProjectStats: func(dir string, statsConfig config.StatsConfig) (stats.Report, error) {
			return stats.LoadForDirWithOptions(dir, statsOptions(statsConfig))
		},
		LoadGlobalWindow: func(label string, start, end time.Time) (stats.WindowReport, error) {
			return stats.LoadWindowReport("", label, start, end)
		},
		LoadProjectWindow: func(dir string, label string, start, end time.Time) (stats.WindowReport, error) {
			return stats.LoadWindowReport(dir, label, start, end)
		},
		LoadGlobalYearMonthly: func(endMonth time.Time) (stats.YearMonthlyReport, error) {
			return stats.LoadYearMonthlyReport("", endMonth)
		},
		LoadProjectYearMonthly: func(dir string, endMonth time.Time) (stats.YearMonthlyReport, error) {
			return stats.LoadYearMonthlyReport(dir, endMonth)
		},
		LoadGlobalMonthDaily: func(monthStart time.Time) (stats.MonthDailyReport, error) {
			return stats.LoadMonthDailyReport("", monthStart)
		},
		LoadProjectMonthDaily: func(dir string, monthStart time.Time) (stats.MonthDailyReport, error) {
			return stats.LoadMonthDailyReport(dir, monthStart)
		},
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      editor.OpenWithConfig,
		ParsePortRange:  port.ParseRange,
		SelectPort:      port.Select,
		IsPortAvailable: port.IsAvailable,
		SendToast:       launch.SendToast,
	}

	deps.RunTUI = func(items []tui.PluginItem, editChoices []tui.EditChoice, sessions []tui.SessionItem, selectedSession tui.SessionItem, globalStats stats.Report, projectStats stats.Report, statsConfig config.StatsConfig, portsRange string, allowMultiplePlugins bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
		model := tui.NewModel(items, editChoices, sessions, selectedSession, globalStats, projectStats, statsConfig, version, allowMultiplePlugins).
			WithStatsLoaders(
				func() (stats.Report, error) {
					return deps.LoadGlobalStats(statsConfig)
				},
				func() (stats.Report, error) {
					cwd, err := deps.Getwd()
					if err != nil {
						return stats.Report{}, err
					}
					return deps.LoadProjectStats(cwd, statsConfig)
				},
				deps.LoadGlobalWindow,
				func(label string, start, end time.Time) (stats.WindowReport, error) {
					cwd, err := deps.Getwd()
					if err != nil {
						return stats.WindowReport{}, err
					}
					return deps.LoadProjectWindow(cwd, label, start, end)
				},
			).
			WithYearMonthlyLoaders(
				deps.LoadGlobalYearMonthly,
				func(endMonth time.Time) (stats.YearMonthlyReport, error) {
					cwd, err := deps.Getwd()
					if err != nil {
						return stats.YearMonthlyReport{}, err
					}
					return deps.LoadProjectYearMonthly(cwd, endMonth)
				},
			).
			WithMonthDailyLoaders(
				deps.LoadGlobalMonthDaily,
				func(monthStart time.Time) (stats.MonthDailyReport, error) {
					cwd, err := deps.Getwd()
					if err != nil {
						return stats.MonthDailyReport{}, err
					}
					return deps.LoadProjectMonthDaily(cwd, monthStart)
				},
			)
		result, err := tea.NewProgram(model).Run()
		if err != nil {
			return nil, false, "", nil, tui.SessionItem{}, err
		}
		finalModel, ok := result.(tui.Model)
		if !ok {
			return nil, false, "", nil, tui.SessionItem{}, fmt.Errorf("unexpected TUI model type %T", result)
		}

		selections := finalModel.Selections()
		if finalModel.Cancelled() || finalModel.EditTarget() != "" {
			return selections, finalModel.Cancelled(), finalModel.EditTarget(), nil, finalModel.SelectedSession(), nil
		}

		portArgs, err := runLaunchTUI(selectedPluginNames(selections), finalModel.SelectedSession(), portsRange, deps, version)
		if err != nil {
			return nil, false, "", nil, tui.SessionItem{}, err
		}

		return selections, false, "", portArgs, finalModel.SelectedSession(), nil
	}

	return deps
}

func normalizeDeps(deps RuntimeDeps) RuntimeDeps {
	if deps.Getwd == nil {
		deps.Getwd = os.Getwd
	}
	if deps.ListSessions == nil {
		deps.ListSessions = func(string) ([]tui.SessionItem, error) { return nil, nil }
	}
	if deps.LoadGlobalStats == nil {
		deps.LoadGlobalStats = func(config.StatsConfig) (stats.Report, error) { return stats.Report{}, nil }
	}
	if deps.LoadProjectStats == nil {
		deps.LoadProjectStats = func(string, config.StatsConfig) (stats.Report, error) { return stats.Report{}, nil }
	}
	if deps.LoadGlobalWindow == nil {
		deps.LoadGlobalWindow = func(string, time.Time, time.Time) (stats.WindowReport, error) { return stats.WindowReport{}, nil }
	}
	if deps.LoadProjectWindow == nil {
		deps.LoadProjectWindow = func(string, string, time.Time, time.Time) (stats.WindowReport, error) {
			return stats.WindowReport{}, nil
		}
	}
	if deps.LoadGlobalYearMonthly == nil {
		deps.LoadGlobalYearMonthly = func(time.Time) (stats.YearMonthlyReport, error) { return stats.YearMonthlyReport{}, nil }
	}
	if deps.LoadProjectYearMonthly == nil {
		deps.LoadProjectYearMonthly = func(string, time.Time) (stats.YearMonthlyReport, error) { return stats.YearMonthlyReport{}, nil }
	}
	if deps.LoadGlobalMonthDaily == nil {
		deps.LoadGlobalMonthDaily = func(time.Time) (stats.MonthDailyReport, error) { return stats.MonthDailyReport{}, nil }
	}
	if deps.LoadProjectMonthDaily == nil {
		deps.LoadProjectMonthDaily = func(string, time.Time) (stats.MonthDailyReport, error) { return stats.MonthDailyReport{}, nil }
	}
	if deps.SendToast == nil {
		deps.SendToast = launch.SendToast
	}
	if deps.ParsePortRange == nil {
		deps.ParsePortRange = port.ParseRange
	}
	if deps.SelectPort == nil {
		deps.SelectPort = port.Select
	}
	if deps.IsPortAvailable == nil {
		deps.IsPortAvailable = port.IsAvailable
	}
	return deps
}

func statsOptions(cfg config.StatsConfig) stats.Options {
	return stats.Options{SessionGapMinutes: cfg.SessionGapMinutes}
}
