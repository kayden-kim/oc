package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/editor"
	"github.com/kayden-kim/oc/internal/plugin"
	"github.com/kayden-kim/oc/internal/port"
	"github.com/kayden-kim/oc/internal/runner"
	"github.com/kayden-kim/oc/internal/tui"
	_ "modernc.org/sqlite"
)

var version = "v0.1.5" // Overridden by ldflags at build time

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		os.Exit(0)
	}
	if err := run(); err != nil {
		if exitErr, ok := runner.IsExitCode(err); ok {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

type runnerAPI interface {
	CheckAvailable() error
	Run(args []string) error
}

type onSuccessRunner interface {
	OnSuccess(func())
}

type runtimeDeps struct {
	newRunner         func() runnerAPI
	userHomeDir       func() (string, error)
	readFile          func(string) ([]byte, error)
	loadOcConfig      func(string) (*config.OcConfig, error)
	parsePlugins      func([]byte) ([]config.Plugin, string, error)
	filterByWhitelist func([]config.Plugin, []string) ([]config.Plugin, []config.Plugin)
	getwd             func() (string, error)
	listSessions      func(string) ([]tui.SessionItem, error)
	runTUI            func([]tui.PluginItem, []tui.EditChoice, []tui.SessionItem, tui.SessionItem, string, bool) (map[string]bool, bool, string, []string, tui.SessionItem, error)
	applySelections   func([]byte, map[string]bool) ([]byte, error)
	writeConfigFile   func(string, []byte) error
	openEditor        func(string, string) error
	parsePortRange    func(string) (int, int, error)
	selectPort        func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, port int, available bool)) port.SelectResult
	isPortAvailable   func(int) bool
}

func defaultDeps() runtimeDeps {
	deps := runtimeDeps{
		newRunner:         func() runnerAPI { return runner.NewRunner() },
		userHomeDir:       os.UserHomeDir,
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: plugin.FilterByWhitelist,
		getwd:             os.Getwd,
		listSessions:      listSessions,
		applySelections:   config.ApplySelections,
		writeConfigFile:   config.WriteConfigFile,
		openEditor:        editor.OpenWithConfig,
		parsePortRange:    port.ParseRange,
		selectPort:        port.Select,
		isPortAvailable:   port.IsAvailable,
	}

	deps.runTUI = func(items []tui.PluginItem, editChoices []tui.EditChoice, sessions []tui.SessionItem, session tui.SessionItem, portsRange string, allowMultiplePlugins bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
		model := tui.NewModel(items, editChoices, sessions, session, version, allowMultiplePlugins)
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

		portArgs, err := runLaunchTUI(selectedPluginNames(selections), finalModel.SelectedSession(), portsRange, deps)
		if err != nil {
			return nil, false, "", nil, tui.SessionItem{}, err
		}

		return selections, false, "", portArgs, finalModel.SelectedSession(), nil
	}

	return deps
}

const (
	toastHealthTimeout  = 5 * time.Second
	toastHealthInterval = 1 * time.Second
	toastClientTimeout  = 1 * time.Second
	toastRequestTimeout = 5 * time.Second
	toastRetryInterval  = 1 * time.Second
	toastMaxAttempts    = 5
)

func run() error {
	return runWithDeps(os.Args[1:], defaultDeps())
}

func runWithDeps(args []string, deps runtimeDeps) error {
	if deps.getwd == nil {
		deps.getwd = os.Getwd
	}
	if deps.listSessions == nil {
		deps.listSessions = func(string) ([]tui.SessionItem, error) {
			return nil, nil
		}
	}

	r := deps.newRunner()
	if err := r.CheckAvailable(); err != nil {
		return fmt.Errorf("opencode not found: %w", err)
	}

	var lastExitErr *runner.ExitCodeError

	homeDir, err := deps.userHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	ocConfigPath := filepath.Join(homeDir, ".oc")
	configDir := filepath.Join(homeDir, ".config", "opencode")
	configPath := filepath.Join(configDir, "opencode.json")
	selectedSession := tui.SessionItem{}

	for {
		cwd, err := deps.getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		sessions, err := deps.listSessions(cwd)
		if err != nil {
			sessions = nil
		}
		if selectedSession.ID == "" {
			selectedSession = latestSession(sessions)
		}

		ocConfig, err := deps.loadOcConfig(ocConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load whitelist: %w", err)
		}

		var whitelist []string
		var configEditor string
		var portsRange string
		allowMultiplePlugins := false
		if ocConfig != nil {
			whitelist = ocConfig.Plugins
			configEditor = ocConfig.Editor
			portsRange = ocConfig.Ports
			allowMultiplePlugins = ocConfig.AllowMultiplePlugins
		}

		effectivePortsRange := portsRange
		if hasPortFlag(args) {
			effectivePortsRange = ""
		}

		content, err := deps.readFile(configPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("opencode.json not found at %s", configPath)
			}
			return fmt.Errorf("failed to read opencode.json: %w", err)
		}

		plugins, _, err := deps.parsePlugins(content)
		if err != nil {
			return fmt.Errorf("failed to parse plugins: %w", err)
		}

		visible, _ := deps.filterByWhitelist(plugins, whitelist)
		if len(visible) == 0 {
			portArgs, err := runLaunchTUI(nil, selectedSession, effectivePortsRange, deps)
			if err != nil {
				return fmt.Errorf("launch TUI error: %w", err)
			}
			return runOpencode(r, args, portArgs, selectedSession, nil)
		}

		items := make([]tui.PluginItem, len(visible))
		for i, p := range visible {
			items[i] = tui.PluginItem{Name: p.Name, InitiallyEnabled: p.Enabled}
		}
		editChoices := []tui.EditChoice{
			{Label: "1) .oc file", Path: ocConfigPath},
			{Label: "2) opencode.json file", Path: configPath},
			{Label: "3) oh-my-opencode.json file", Path: resolveOhMyOpencodePath(configDir)},
		}

		var selections map[string]bool
		var cancelled bool
		var editTarget string
		var portArgs []string
		selections, cancelled, editTarget, portArgs, selectedSession, err = deps.runTUI(items, editChoices, sessions, selectedSession, effectivePortsRange, allowMultiplePlugins)
		if err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		if cancelled {
			if lastExitErr != nil {
				return lastExitErr
			}
			return nil
		}
		if editTarget != "" {
			if err := deps.openEditor(editTarget, configEditor); err != nil {
				return fmt.Errorf("failed to open editor for %s: %w", editTarget, err)
			}
			continue
		}

		modified, err := deps.applySelections(content, selections)
		if err != nil {
			return fmt.Errorf("failed to apply selections: %w", err)
		}
		if err := deps.writeConfigFile(configPath, modified); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		err = runOpencode(r, args, portArgs, selectedSession, selections)
		cwd, cwdErr := deps.getwd()
		if cwdErr == nil {
			refreshedSessions, listErr := deps.listSessions(cwd)
			if listErr == nil {
				selectedSession = latestSession(refreshedSessions)
			}
		}
		if exitErr, ok := runner.IsExitCode(err); ok {
			lastExitErr = exitErr
			fmt.Fprintf(os.Stderr, "opencode exited with code %d\n\n", exitErr.Code)
			continue
		}
		if err != nil {
			return err
		}
		lastExitErr = nil
	}
}

func runOpencode(r runnerAPI, args []string, portArgs []string, session tui.SessionItem, selections map[string]bool) error {
	args = appendSessionArgs(args, session)
	args = append(args, portArgs...)
	plugins := selectedPluginNames(selections)

	if osr, ok := r.(onSuccessRunner); ok {
		osr.OnSuccess(nil)
		if port, ok := launchPort(args); ok {
			osr.OnSuccess(func() {
				if err := sendLaunchToast(port, plugins); err != nil {
					logToastFailure(port, err)
				}
			})
		}
	}

	return r.Run(args)
}

func launchPort(portArgs []string) (int, bool) {
	for i := 0; i < len(portArgs); i++ {
		arg := portArgs[i]
		switch {
		case arg == "--port" && i+1 < len(portArgs):
			port, err := strconv.Atoi(portArgs[i+1])
			if err != nil {
				return 0, false
			}
			return port, true
		case strings.HasPrefix(arg, "--port="):
			port, err := strconv.Atoi(strings.TrimPrefix(arg, "--port="))
			if err != nil {
				return 0, false
			}
			return port, true
		}
	}

	return 0, false
}

func hasPortFlag(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--port" {
			return true
		}
		if strings.HasPrefix(arg, "--port=") {
			return true
		}
	}

	return false
}

func selectedPluginNames(selections map[string]bool) []string {
	var enabled []string
	for name, selected := range selections {
		if selected {
			enabled = append(enabled, name)
		}
	}
	sort.Strings(enabled)
	return enabled
}

func runLaunchTUI(plugins []string, session tui.SessionItem, portsRange string, deps runtimeDeps) ([]string, error) {
	launchModel := tui.NewLaunchModel(plugins, session, version, func(msgCh chan<- tea.Msg) {
		defer close(msgCh)
		portArgs := resolvePortArgs(portsRange, deps, func(line string) {
			msgCh <- tui.LaunchLogMsg{Line: line}
		})
		msgCh <- tui.LaunchReadyMsg{PortArgs: portArgs}
	})

	launchResult, err := tea.NewProgram(launchModel).Run()
	if err != nil {
		return nil, err
	}

	finalLaunchModel, ok := launchResult.(tui.LaunchModel)
	if !ok {
		return nil, fmt.Errorf("unexpected launch TUI model type %T", launchResult)
	}

	return finalLaunchModel.PortArgs(), nil
}

func resolvePortArgs(portsRange string, deps runtimeDeps, logFn func(string)) []string {
	if portsRange == "" {
		return nil
	}

	minPort, maxPort, err := deps.parsePortRange(portsRange)
	if err != nil {
		if logFn != nil {
			logFn(fmt.Sprintf("Warning: invalid ports config %q: %v", portsRange, err))
			logFn("Launching opencode without --port flag.")
		}
		return nil
	}

	if logFn != nil {
		logFn(fmt.Sprintf("Port selection: range %d-%d", minPort, maxPort))
	}

	result := deps.selectPort(minPort, maxPort, deps.isPortAvailable, func(attempt, p int, available bool) {
		if logFn == nil {
			return
		}

		status := "in use"
		if available {
			status = "available"
		}
		logFn(fmt.Sprintf("  [%2d/15] port %d ... %s", attempt, p, status))
	})
	if !result.Found {
		if logFn != nil {
			logFn("Warning: no available port found after 15 attempts.")
			logFn("Launching opencode without --port flag.")
		}
		return nil
	}

	if logFn != nil {
		logFn(fmt.Sprintf("Using port %d", result.Port))
	}

	return []string{"--port", strconv.Itoa(result.Port)}
}

func resolveOhMyOpencodePath(configDir string) string {
	jsonPath := filepath.Join(configDir, "oh-my-opencode.json")
	if _, err := os.Stat(jsonPath); err == nil {
		return jsonPath
	}

	jsoncPath := filepath.Join(configDir, "oh-my-opencode.jsonc")
	if _, err := os.Stat(jsoncPath); err == nil {
		return jsoncPath
	}

	return jsonPath
}

func sendLaunchToast(port int, plugins []string) error {
	client := &http.Client{Timeout: toastClientTimeout}
	healthCtx, cancel := context.WithTimeout(context.Background(), toastHealthTimeout)
	defer cancel()
	if err := waitForServerHealthy(healthCtx, client, port); err != nil {
		return err
	}

	payload := struct {
		Title   string `json:"title,omitempty"`
		Message string `json:"message"`
		Variant string `json:"variant"`
	}{
		Title:   "OC Launcher",
		Message: buildToastMessage(plugins, strconv.Itoa(port)),
		Variant: "info",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < toastMaxAttempts; attempt++ {
		err = postLaunchToast(client, port, body)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < toastMaxAttempts-1 {
			time.Sleep(toastRetryInterval)
		}
	}

	return lastErr
}

func postLaunchToast(client *http.Client, port int, body []byte) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/tui/show-toast", port)
	toastCtx, toastCancel := context.WithTimeout(context.Background(), toastRequestTimeout)
	defer toastCancel()

	req, err := http.NewRequestWithContext(toastCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	applyServerAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("toast request failed: %s", resp.Status)
	}

	var accepted bool
	if err := json.NewDecoder(resp.Body).Decode(&accepted); err != nil && err != io.EOF {
		return err
	}
	if !accepted {
		return fmt.Errorf("toast request returned false")
	}

	return nil
}

func logToastFailure(port int, err error) {
	fmt.Fprintf(os.Stderr, "oc: toast failed on port %d: %v\n", port, err)
}

func waitForServerHealthy(ctx context.Context, client *http.Client, port int) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/global/health", port)
	ticker := time.NewTicker(toastHealthInterval)
	defer ticker.Stop()

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		applyServerAuth(req)

		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func applyServerAuth(req *http.Request) {
	password := os.Getenv("OPENCODE_SERVER_PASSWORD")
	if password == "" {
		return
	}

	username := os.Getenv("OPENCODE_SERVER_USERNAME")
	if username == "" {
		username = "opencode"
	}
	req.SetBasicAuth(username, password)
}

func buildToastMessage(plugins []string, portArg string) string {
	var parts []string
	if len(plugins) > 0 {
		parts = append(parts, fmt.Sprintf("Plugins: %s", strings.Join(plugins, ", ")))
	}
	if portArg != "" {
		parts = append(parts, fmt.Sprintf("Port: %s", portArg))
	}
	if len(parts) == 0 {
		return "OpenCode launched"
	}
	return strings.Join(parts, " | ")
}

type sessionRow struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Updated   int64  `json:"updated"`
	Directory string `json:"directory"`
}

func listSessions(dir string) ([]tui.SessionItem, error) {
	items, err := listSessionsDB(dir)
	if err == nil {
		return items, nil
	}

	return listSessionsCLI(dir)
}

func listSessionsDB(dir string) ([]tui.SessionItem, error) {
	dbPath, err := opencodeDBPath()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := "SELECT id, title, time_updated, directory FROM session WHERE parent_id IS NULL AND directory = ? ORDER BY time_updated DESC LIMIT 100"
	if runtime.GOOS == "windows" {
		query = "SELECT id, title, time_updated, directory FROM session WHERE parent_id IS NULL AND replace(lower(directory), '\\', '/') = replace(lower(?), '\\', '/') ORDER BY time_updated DESC LIMIT 100"
	}

	rows, err := db.Query(query, filepath.Clean(dir))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []tui.SessionItem
	for rows.Next() {
		var row sessionRow
		if err := rows.Scan(&row.ID, &row.Title, &row.Updated, &row.Directory); err != nil {
			return nil, err
		}
		if !sameDir(row.Directory, dir) {
			continue
		}
		result = append(result, tui.SessionItem{ID: row.ID, Title: row.Title, UpdatedAt: unixTimestampToTime(row.Updated)})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func listSessionsCLI(dir string) ([]tui.SessionItem, error) {
	cmd := exec.Command("opencode", "session", "list", "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var rows []sessionRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, err
	}

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Updated > rows[j].Updated
	})

	var result []tui.SessionItem
	for _, row := range rows {
		if !sameDir(row.Directory, dir) {
			continue
		}
		result = append(result, tui.SessionItem{ID: row.ID, Title: row.Title, UpdatedAt: unixTimestampToTime(row.Updated)})
	}

	return result, nil
}

func sqliteDSN(path string) string {
	path = filepath.ToSlash(path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "file:" + path + "?mode=ro&_pragma=busy_timeout(5000)"
}

func opencodeDBPath() (string, error) {
	if name := os.Getenv("OPENCODE_DB"); name != "" {
		if filepath.IsAbs(name) {
			if _, err := os.Stat(name); err == nil {
				return name, nil
			} else {
				return "", err
			}
		}

		root, err := opencodeDataDir()
		if err != nil {
			return "", err
		}
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else {
			return "", err
		}
	}

	root, err := opencodeDataDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(root, "opencode.db")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	paths, err := filepath.Glob(filepath.Join(root, "opencode-*.db"))
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", os.ErrNotExist
	}

	sort.SliceStable(paths, func(i, j int) bool {
		left, leftErr := os.Stat(paths[i])
		right, rightErr := os.Stat(paths[j])
		if leftErr != nil || rightErr != nil {
			return paths[i] < paths[j]
		}
		return left.ModTime().After(right.ModTime())
	})

	return paths[0], nil
}

func opencodeDataDir() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "opencode"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "opencode"), nil
	}

	return filepath.Join(home, ".local", "share", "opencode"), nil
}

func unixTimestampToTime(value int64) time.Time {
	switch {
	case value >= 1_000_000_000_000_000_000 || value <= -1_000_000_000_000_000_000:
		return time.Unix(0, value).Local()
	case value >= 1_000_000_000_000_000 || value <= -1_000_000_000_000_000:
		return time.UnixMicro(value).Local()
	case value >= 1_000_000_000_000 || value <= -1_000_000_000_000:
		return time.UnixMilli(value).Local()
	default:
		return time.Unix(value, 0).Local()
	}
}

func latestSession(items []tui.SessionItem) tui.SessionItem {
	if len(items) == 0 {
		return tui.SessionItem{}
	}
	return items[0]
}

func sameDir(left string, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func appendSessionArgs(args []string, session tui.SessionItem) []string {
	if session.ID == "" || hasSessionArgs(args) {
		return append([]string(nil), args...)
	}

	result := append([]string(nil), args...)
	result = append(result, "-s", session.ID)
	return result
}

func hasSessionArgs(args []string) bool {
	for i, arg := range args {
		if arg == "-s" || arg == "--session" || arg == "-c" || arg == "--continue" {
			return true
		}
		if strings.HasPrefix(arg, "--session=") {
			return true
		}
		if arg == "-s=" || arg == "--continue=" {
			return true
		}
		if arg == "-s" && i < len(args)-1 {
			return true
		}
	}
	return false
}
