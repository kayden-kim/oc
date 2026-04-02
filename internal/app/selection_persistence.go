package app

import (
	"fmt"
	"maps"
)

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
