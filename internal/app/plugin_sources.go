package app

import (
	"fmt"
	"os"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/tui"
)

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

func loadPluginSources(deps RuntimeDeps, paths runtimePaths, whitelist []string) (*configSource, *configSource, error) {
	userContent, err := readConfigContent(deps, paths.configPath)
	if err != nil {
		return nil, nil, err
	}
	projectContent, err := readOptionalConfigContent(deps, paths.projectConfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read project opencode.json: %w", err)
	}

	userPlugins, _, err := deps.ParsePlugins(userContent)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse plugins: %w", err)
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

	return userSource, projectSource, nil
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
