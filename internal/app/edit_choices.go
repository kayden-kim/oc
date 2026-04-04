package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kayden-kim/oc/internal/tui"
)

func ResolveOhMyOpencodePath(configDir string) string {
	return resolveOhMyOpencodePath(configDir, os.Stat)
}

func DiscoverOhMyConfigPaths(configDir string) []string {
	return discoverOhMyConfigPaths(configDir, os.Stat)
}

func discoverOhMyConfigPaths(configDir string, statFn func(string) (os.FileInfo, error)) []string {
	candidates := []string{
		"oh-my-opencode.json",
		"oh-my-opencode.jsonc",
		"oh-my-openagent.json",
		"oh-my-openagent.jsonc",
	}

	paths := make([]string, 0, len(candidates))
	for _, name := range candidates {
		path := filepath.Join(configDir, name)
		if _, err := statFn(path); err == nil {
			paths = append(paths, path)
		}
	}

	return paths
}

func resolveOhMyOpencodePath(configDir string, statFn func(string) (os.FileInfo, error)) string {
	paths := discoverOhMyConfigPaths(configDir, statFn)
	if len(paths) > 0 {
		return paths[0]
	}

	return filepath.Join(configDir, "oh-my-opencode.json")
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
