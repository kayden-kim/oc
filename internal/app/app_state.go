package app

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/launch"
	"github.com/kayden-kim/oc/internal/session"
	"github.com/kayden-kim/oc/internal/tui"
)

type runtimePaths struct {
	ocConfigPath      string
	configDir         string
	configPath        string
	projectConfigPath string
}

type iterationState struct {
	cwd                  string
	sessions             []tui.SessionItem
	selectedSession      tui.SessionItem
	statsConfig          config.StatsConfig
	content              []byte
	visible              []config.Plugin
	userSource           *configSource
	projectSource        *configSource
	mergedItems          []tui.PluginItem
	mergedPlugins        []mergedPlugin
	configEditor         string
	effectivePortsRange  string
	allowMultiplePlugins bool
}

type configSource struct {
	path    string
	content []byte
	plugins []config.Plugin
}

type mergedPlugin struct {
	name      string
	inUser    bool
	inProject bool
}

func resolveRuntimePaths(homeDir string) runtimePaths {
	return runtimePaths{
		ocConfigPath:      filepath.Join(homeDir, ".oc"),
		configDir:         filepath.Join(homeDir, ".config", "opencode"),
		configPath:        filepath.Join(homeDir, ".config", "opencode", "opencode.json"),
		projectConfigPath: "",
	}
}

func readOptionalConfigContent(deps RuntimeDeps, path string) ([]byte, error) {
	content, err := deps.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return content, nil
}

func loadIterationState(args []string, deps RuntimeDeps, paths runtimePaths, selectedSession tui.SessionItem) (iterationState, error) {
	cwd, err := deps.Getwd()
	if err != nil {
		return iterationState{}, fmt.Errorf("failed to get current directory: %w", err)
	}

	paths.projectConfigPath = filepath.Join(cwd, ".opencode", "opencode.json")

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
	userContent, err := readConfigContent(deps, paths.configPath)
	if err != nil {
		return iterationState{}, err
	}
	projectContent, err := readOptionalConfigContent(deps, paths.projectConfigPath)
	if err != nil {
		return iterationState{}, fmt.Errorf("failed to read project opencode.json: %w", err)
	}

	userPlugins, _, err := deps.ParsePlugins(userContent)
	if err != nil {
		return iterationState{}, fmt.Errorf("failed to parse plugins: %w", err)
	}
	visible, _ := deps.FilterByWhitelist(userPlugins, whitelist)

	userSource := &configSource{path: paths.configPath, content: userContent, plugins: visible}

	var projectSource *configSource
	if projectContent != nil {
		projectPlugins, _, parseErr := deps.ParsePlugins(projectContent)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse project config %s: %v\n", paths.projectConfigPath, parseErr)
		} else {
			projectVisible, _ := deps.FilterByWhitelist(projectPlugins, whitelist)
			projectSource = &configSource{path: paths.projectConfigPath, content: projectContent, plugins: projectVisible}
		}
	}

	mergedItems, mergedPlugins := mergePlugins(userSource, projectSource)

	return iterationState{
		cwd:                  cwd,
		sessions:             sessions,
		selectedSession:      selectedSession,
		statsConfig:          statsConfig,
		content:              userContent,
		visible:              visible,
		userSource:           userSource,
		projectSource:        projectSource,
		mergedItems:          mergedItems,
		mergedPlugins:        mergedPlugins,
		configEditor:         configEditor,
		effectivePortsRange:  effectivePortsRange,
		allowMultiplePlugins: allowMultiplePlugins,
	}, nil
}

func extractRuntimeConfig(args []string, ocConfig *config.OcConfig) ([]string, string, string, bool, config.StatsConfig) {
	var whitelist []string
	var configEditor string
	var portsRange string
	var statsConfig config.StatsConfig
	allowMultiplePlugins := false
	if ocConfig != nil {
		whitelist = ocConfig.Plugins
		configEditor = ocConfig.Editor
		portsRange = ocConfig.Ports
		allowMultiplePlugins = ocConfig.AllowMultiplePlugins
		statsConfig = ocConfig.Stats
	}

	if launch.HasPortFlag(args) {
		portsRange = ""
	} else if portsRange == "" {
		portsRange = DefaultPortsRange
	}

	return whitelist, configEditor, portsRange, allowMultiplePlugins, statsConfig
}

func readConfigContent(deps RuntimeDeps, configPath string) ([]byte, error) {
	content, err := deps.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("opencode.json not found at %s", configPath)
		}
		return nil, fmt.Errorf("failed to read opencode.json: %w", err)
	}
	return content, nil
}

func mergePlugins(userSource, projectSource *configSource) (items []tui.PluginItem, merged []mergedPlugin) {
	if userSource == nil {
		userSource = &configSource{}
	}
	if projectSource == nil {
		items = make([]tui.PluginItem, 0, len(userSource.plugins))
		merged = make([]mergedPlugin, 0, len(userSource.plugins))
		seen := make(map[string]int, len(userSource.plugins))
		for _, p := range userSource.plugins {
			if idx, ok := seen[p.Name]; ok {
				if p.Enabled {
					items[idx].InitiallyEnabled = true
				}
				continue
			}
			seen[p.Name] = len(items)
			items = append(items, tui.PluginItem{Name: p.Name, InitiallyEnabled: p.Enabled})
			merged = append(merged, mergedPlugin{name: p.Name, inUser: true})
		}
		return items, merged
	}
	capacity := len(userSource.plugins) + len(projectSource.plugins)
	items = make([]tui.PluginItem, 0, capacity)
	merged = make([]mergedPlugin, 0, capacity)
	seen := make(map[string]int, capacity)
	for _, p := range userSource.plugins {
		if idx, ok := seen[p.Name]; ok {
			merged[idx].inUser = true
			if p.Enabled {
				items[idx].InitiallyEnabled = true
			}
			continue
		}
		seen[p.Name] = len(items)
		items = append(items, tui.PluginItem{Name: p.Name, InitiallyEnabled: p.Enabled})
		merged = append(merged, mergedPlugin{name: p.Name, inUser: true})
	}
	for _, p := range projectSource.plugins {
		if idx, ok := seen[p.Name]; ok {
			merged[idx].inProject = true
			if p.Enabled {
				items[idx].InitiallyEnabled = true
			}
			continue
		}
		seen[p.Name] = len(items)
		items = append(items, tui.PluginItem{Name: p.Name, InitiallyEnabled: p.Enabled})
		merged = append(merged, mergedPlugin{name: p.Name, inProject: true})
	}
	for i, p := range merged {
		switch {
		case p.inUser && p.inProject:
			items[i].SourceLabel = "User, Project"
		case p.inUser:
			items[i].SourceLabel = "User"
		case p.inProject:
			items[i].SourceLabel = "Project"
		}
	}
	return items, merged
}

func buildEditChoices(paths runtimePaths, projectConfigPath string, projectConfigExists bool) []tui.EditChoice {
	userOhMyConfigs := DiscoverOhMyConfigPaths(paths.configDir)
	choices := []tui.EditChoice{{Label: "1) .oc file", Path: paths.ocConfigPath}, {Label: "2) opencode.json file", Path: paths.configPath}}
	if len(userOhMyConfigs) == 0 {
		userOhMyConfigs = []string{ResolveOhMyOpencodePath(paths.configDir)}
	}
	for _, path := range userOhMyConfigs {
		choices = append(choices, tui.EditChoice{Label: fmt.Sprintf("%d) %s file", len(choices)+1, filepath.Base(path)), Path: path})
	}
	if projectConfigExists {
		choices = append(choices, tui.EditChoice{Label: fmt.Sprintf("%d) project opencode.json file", len(choices)+1), Path: projectConfigPath})
		for _, path := range DiscoverOhMyConfigPaths(filepath.Dir(projectConfigPath)) {
			choices = append(choices, tui.EditChoice{Label: fmt.Sprintf("%d) project %s file", len(choices)+1, filepath.Base(path)), Path: path})
		}
	}
	return choices
}

func persistSelections(deps RuntimeDeps, state iterationState, selections map[string]bool) error {
	userSelections := make(map[string]bool)
	projectSelections := make(map[string]bool)
	for _, plugin := range state.mergedPlugins {
		selected, ok := selections[plugin.name]
		if !ok {
			continue
		}
		if plugin.inUser {
			userSelections[plugin.name] = selected
		}
		if plugin.inProject {
			projectSelections[plugin.name] = selected
		}
	}
	if state.projectSource == nil && len(state.mergedPlugins) == 0 {
		maps.Copy(userSelections, selections)
	}
	if state.userSource != nil && len(userSelections) > 0 {
		modified, err := deps.ApplySelections(state.userSource.content, userSelections)
		if err != nil {
			return fmt.Errorf("failed to apply user selections: %w", err)
		}
		if err := deps.WriteConfigFile(state.userSource.path, modified); err != nil {
			return fmt.Errorf("failed to write user config: %w", err)
		}
	}
	if state.projectSource != nil && len(projectSelections) > 0 {
		modified, err := deps.ApplySelections(state.projectSource.content, projectSelections)
		if err != nil {
			return fmt.Errorf("failed to apply project selections: %w", err)
		}
		if err := deps.WriteConfigFile(state.projectSource.path, modified); err != nil {
			return fmt.Errorf("failed to write project config: %w", err)
		}
	}
	return nil
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
