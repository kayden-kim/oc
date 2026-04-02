package app

import (
	"fmt"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/session"
	"github.com/kayden-kim/oc/internal/tui"
)

type iterationState struct {
	cwd                  string
	projectConfigPath    string
	sessions             []tui.SessionItem
	selectedSession      tui.SessionItem
	statsConfig          config.StatsConfig
	userSource           *configSource
	projectSource        *configSource
	mergedItems          []tui.PluginItem
	mergedPlugins        []mergedPlugin
	configEditor         string
	effectivePortsRange  string
	allowMultiplePlugins bool
}

func loadIterationState(args []string, deps RuntimeDeps, paths runtimePaths, selectedSession tui.SessionItem) (iterationState, error) {
	cwd, err := deps.Getwd()
	if err != nil {
		return iterationState{}, fmt.Errorf("failed to get current directory: %w", err)
	}

	paths = withProjectConfigPath(paths, cwd)

	sessions, err := deps.ListSessions(cwd)
	if err != nil {
		sessions = nil
	}
	if selectedSession.ID == "" {
		selectedSession = session.Latest(sessions)
	}

	ocConfig, err := deps.LoadOcConfig(paths.ocConfigPath)
	if err != nil {
		return iterationState{}, fmt.Errorf("failed to load whitelist: %w", err)
	}

	whitelist, configEditor, effectivePortsRange, allowMultiplePlugins, statsConfig := extractRuntimeConfig(args, ocConfig)
	userSource, projectSource, err := loadPluginSources(deps, paths, whitelist)
	if err != nil {
		return iterationState{}, err
	}
	mergedItems, mergedPlugins := mergePlugins(userSource, projectSource)

	return iterationState{
		cwd:                  cwd,
		projectConfigPath:    paths.projectConfigPath,
		sessions:             sessions,
		selectedSession:      selectedSession,
		statsConfig:          statsConfig,
		userSource:           userSource,
		projectSource:        projectSource,
		mergedItems:          mergedItems,
		mergedPlugins:        mergedPlugins,
		configEditor:         configEditor,
		effectivePortsRange:  effectivePortsRange,
		allowMultiplePlugins: allowMultiplePlugins,
	}, nil
}

func refreshSelectedSession(deps RuntimeDeps, cwd string, current tui.SessionItem) tui.SessionItem {
	sessions, err := deps.ListSessions(cwd)
	if err != nil {
		return current
	}
	if len(sessions) == 0 {
		return tui.SessionItem{}
	}
	return session.Latest(sessions)
}
