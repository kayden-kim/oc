package app

import (
	"fmt"

	"github.com/kayden-kim/oc/internal/launch"
	"github.com/kayden-kim/oc/internal/tui"
)

func runContinuePath(args []string, deps RuntimeDeps, paths runtimePaths, r RunnerAPI) error {
	ocConfig, err := deps.LoadOcConfig(paths.ocConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load whitelist: %w", err)
	}

	_, _, effectivePortsRange, _, _ := extractRuntimeConfig(args, ocConfig)
	portArgs := launch.ResolvePortArgs(effectivePortsRange, deps.ParsePortRange, deps.SelectPort, deps.IsPortAvailable, nil)
	return runOpencode(r, args, portArgs, tui.SessionItem{}, nil, deps.SendToast)
}
