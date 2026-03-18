package plugin

import (
	"github.com/kayden-kim/oc/internal/config"
)

// FilterByWhitelist separates plugins into visible and hidden groups based on a whitelist.
// - If whitelist is nil, all plugins are visible (empty hidden)
// - If whitelist is empty []string{}, all plugins are hidden (empty visible)
// - If whitelist has values, only plugins in the whitelist are visible
// The function preserves all plugin state (Enabled, LineIndex, OriginalLine) in both groups.
func FilterByWhitelist(plugins []config.Plugin, whitelist []string) (visible []config.Plugin, hidden []config.Plugin) {
	// nil whitelist means show all plugins
	if whitelist == nil {
		return plugins, []config.Plugin{}
	}

	// Build a map for O(1) lookup of whitelisted names
	whitelistMap := make(map[string]bool)
	for _, name := range whitelist {
		whitelistMap[name] = true
	}

	// Separate plugins into visible and hidden
	for _, plugin := range plugins {
		if whitelistMap[plugin.Name] {
			visible = append(visible, plugin)
		} else {
			hidden = append(hidden, plugin)
		}
	}

	// Ensure we return empty slices instead of nil for consistency
	if visible == nil {
		visible = []config.Plugin{}
	}
	if hidden == nil {
		hidden = []config.Plugin{}
	}

	return visible, hidden
}
