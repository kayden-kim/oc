package app

import (
	"fmt"
	"path/filepath"

	"github.com/kayden-kim/oc/internal/tui"
)

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
