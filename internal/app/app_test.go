package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/tui"
)

func TestResolveOhMyOpencodePath(t *testing.T) {
	t.Run("prefers json when present", func(t *testing.T) {
		configDir := t.TempDir()
		jsonPath := filepath.Join(configDir, "oh-my-opencode.json")
		jsoncPath := filepath.Join(configDir, "oh-my-opencode.jsonc")
		if err := os.WriteFile(jsonPath, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(jsoncPath, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if got := ResolveOhMyOpencodePath(configDir); got != jsonPath {
			t.Fatalf("expected %q, got %q", jsonPath, got)
		}
	})

	t.Run("falls back to opencode jsonc", func(t *testing.T) {
		configDir := t.TempDir()
		jsoncPath := filepath.Join(configDir, "oh-my-opencode.jsonc")
		if err := os.WriteFile(jsoncPath, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if got := ResolveOhMyOpencodePath(configDir); got != jsoncPath {
			t.Fatalf("expected %q, got %q", jsoncPath, got)
		}
	})

	t.Run("recognizes openagent variants after opencode variants", func(t *testing.T) {
		configDir := t.TempDir()
		openagentJSONPath := filepath.Join(configDir, "oh-my-openagent.json")
		if err := os.WriteFile(openagentJSONPath, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if got := ResolveOhMyOpencodePath(configDir); got != openagentJSONPath {
			t.Fatalf("expected %q, got %q", openagentJSONPath, got)
		}
	})

	t.Run("defaults to json path", func(t *testing.T) {
		configDir := t.TempDir()
		want := filepath.Join(configDir, "oh-my-opencode.json")

		if got := ResolveOhMyOpencodePath(configDir); got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})
}

func TestDiscoverOhMyConfigPaths(t *testing.T) {
	t.Run("returns all supported filenames in stable order", func(t *testing.T) {
		configDir := t.TempDir()
		want := []string{
			filepath.Join(configDir, "oh-my-opencode.json"),
			filepath.Join(configDir, "oh-my-opencode.jsonc"),
			filepath.Join(configDir, "oh-my-openagent.json"),
			filepath.Join(configDir, "oh-my-openagent.jsonc"),
		}

		for _, path := range want {
			if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		got := DiscoverOhMyConfigPaths(configDir)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
	})

	t.Run("returns empty when none exist", func(t *testing.T) {
		configDir := t.TempDir()
		got := DiscoverOhMyConfigPaths(configDir)
		if len(got) != 0 {
			t.Fatalf("expected no discovered configs, got %v", got)
		}
	})
}

func TestReadOptionalConfigContent_ExistingFile(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "config.json")
	expected := []byte(`{"test": "content"}`)
	if err := os.WriteFile(filePath, expected, 0o644); err != nil {
		t.Fatal(err)
	}

	deps := RuntimeDeps{ReadFile: os.ReadFile}

	content, err := readOptionalConfigContent(deps, filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(content, expected) {
		t.Fatalf("expected %q, got %q", expected, content)
	}
}

func TestReadOptionalConfigContent_MissingFile(t *testing.T) {
	deps := RuntimeDeps{ReadFile: os.ReadFile}

	content, err := readOptionalConfigContent(deps, "/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if content != nil {
		t.Fatalf("expected nil content for missing file, got %q", content)
	}
}

func TestReadOptionalConfigContent_OtherError(t *testing.T) {
	customErr := errors.New("permission denied")
	deps := RuntimeDeps{ReadFile: func(string) ([]byte, error) { return nil, customErr }}

	content, err := readOptionalConfigContent(deps, "/some/path")
	if err != customErr {
		t.Fatalf("expected error %v, got %v", customErr, err)
	}
	if content != nil {
		t.Fatalf("expected nil content on error, got %q", content)
	}
}

func TestLoadIterationState_UserOnlyConfig(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "workspace")
	paths := resolveRuntimePaths(home)
	userConfig := []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    // \"plugin-b\"\n  ]\n}\n")

	deps := RuntimeDeps{
		Getwd:        func() (string, error) { return cwd, nil },
		ListSessions: func(string) ([]tui.SessionItem, error) { return nil, nil },
		LoadOcConfig: func(string) (*config.OcConfig, error) { return nil, nil },
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case paths.configPath:
				return userConfig, nil
			case filepath.Join(cwd, ".opencode", "opencode.json"):
				return nil, os.ErrNotExist
			default:
				t.Fatalf("unexpected read path: %s", path)
				return nil, nil
			}
		},
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
	}

	state, err := loadIterationState(nil, deps, paths, tui.SessionItem{})
	if err != nil {
		t.Fatalf("loadIterationState returned error: %v", err)
	}
	if state.userSource == nil {
		t.Fatal("expected userSource to be populated")
	}
	if state.projectSource != nil {
		t.Fatal("expected projectSource to be nil when project config is missing")
	}
	if len(state.mergedItems) != 2 {
		t.Fatalf("expected 2 merged items from user config, got %d", len(state.mergedItems))
	}
	if state.mergedItems[0].Name != "plugin-a" || state.mergedItems[0].SourceLabel != "" {
		t.Fatalf("expected first merged item to be user-only with empty label, got %+v", state.mergedItems[0])
	}
	if state.mergedItems[1].Name != "plugin-b" || state.mergedItems[1].SourceLabel != "" {
		t.Fatalf("expected second merged item to be user-only with empty label, got %+v", state.mergedItems[1])
	}
	if len(state.mergedPlugins) != 2 || !state.mergedPlugins[0].inUser || state.mergedPlugins[0].inProject {
		t.Fatalf("unexpected merged plugin flags: %+v", state.mergedPlugins)
	}
}

func TestLoadIterationState_DualConfigMergesAndLabelsSources(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "workspace")
	paths := resolveRuntimePaths(home)
	userPath := paths.configPath
	projectPath := filepath.Join(cwd, ".opencode", "opencode.json")
	userConfig := []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    // \"plugin-b\"\n  ]\n}\n")
	projectConfig := []byte("{\n  \"plugin\": [\n    \"plugin-b\",\n    // \"plugin-c\"\n  ]\n}\n")

	deps := RuntimeDeps{
		Getwd:        func() (string, error) { return cwd, nil },
		ListSessions: func(string) ([]tui.SessionItem, error) { return nil, nil },
		LoadOcConfig: func(string) (*config.OcConfig, error) { return nil, nil },
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case userPath:
				return userConfig, nil
			case projectPath:
				return projectConfig, nil
			default:
				t.Fatalf("unexpected read path: %s", path)
				return nil, nil
			}
		},
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
	}

	state, err := loadIterationState(nil, deps, paths, tui.SessionItem{})
	if err != nil {
		t.Fatalf("loadIterationState returned error: %v", err)
	}
	if state.userSource == nil || state.projectSource == nil {
		t.Fatalf("expected both sources to be populated, got user=%v project=%v", state.userSource != nil, state.projectSource != nil)
	}

	expectedItems := []tui.PluginItem{{Name: "plugin-a", InitiallyEnabled: true, SourceLabel: "User"}, {Name: "plugin-b", InitiallyEnabled: true, SourceLabel: "User, Project"}, {Name: "plugin-c", InitiallyEnabled: false, SourceLabel: "Project"}}
	if !reflect.DeepEqual(state.mergedItems, expectedItems) {
		t.Fatalf("merged items mismatch\nwant: %#v\ngot:  %#v", expectedItems, state.mergedItems)
	}

	expectedMerged := []mergedPlugin{{name: "plugin-a", inUser: true, inProject: false}, {name: "plugin-b", inUser: true, inProject: true}, {name: "plugin-c", inUser: false, inProject: true}}
	if !reflect.DeepEqual(state.mergedPlugins, expectedMerged) {
		t.Fatalf("merged metadata mismatch\nwant: %#v\ngot:  %#v", expectedMerged, state.mergedPlugins)
	}
}

func TestLoadIterationState_ProjectParseErrorFallsBackToUserOnly(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "workspace")
	paths := resolveRuntimePaths(home)
	projectPath := filepath.Join(cwd, ".opencode", "opencode.json")
	userConfig := []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    // \"plugin-b\"\n  ]\n}\n")
	badProjectConfig := []byte("{\n  \"plugin\": [\n")

	deps := RuntimeDeps{
		Getwd:        func() (string, error) { return cwd, nil },
		ListSessions: func(string) ([]tui.SessionItem, error) { return nil, nil },
		LoadOcConfig: func(string) (*config.OcConfig, error) { return nil, nil },
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case paths.configPath:
				return userConfig, nil
			case projectPath:
				return badProjectConfig, nil
			default:
				t.Fatalf("unexpected read path: %s", path)
				return nil, nil
			}
		},
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
	}

	var state iterationState
	var loadErr error
	stderrOutput := captureOutput(t, true, func() {
		state, loadErr = loadIterationState(nil, deps, paths, tui.SessionItem{})
	})
	if loadErr != nil {
		t.Fatalf("loadIterationState returned error: %v", loadErr)
	}
	if state.projectSource != nil {
		t.Fatal("expected projectSource to be nil on project parse error")
	}
	if len(state.mergedItems) != 2 {
		t.Fatalf("expected user-only merged items on parse fallback, got %d", len(state.mergedItems))
	}
	if state.mergedItems[0].SourceLabel != "" || state.mergedItems[1].SourceLabel != "" {
		t.Fatalf("expected empty labels on parse fallback, got %+v", state.mergedItems)
	}
	if !strings.Contains(string(stderrOutput), "Warning: failed to parse project config ") {
		t.Fatalf("expected parse warning in stderr, got %q", string(stderrOutput))
	}
	if !strings.Contains(string(stderrOutput), projectPath) {
		t.Fatalf("expected warning to include project path %q, got %q", projectPath, string(stderrOutput))
	}
}

func TestMergePlugins(t *testing.T) {
	plugin := func(name string, enabled bool) config.Plugin { return config.Plugin{Name: name, Enabled: enabled} }
	tests := []struct {
		name           string
		userSource     *configSource
		projectSource  *configSource
		expectedItems  []tui.PluginItem
		expectedMerged []mergedPlugin
	}{
		{name: "no project config keeps backward-compatible empty labels", userSource: &configSource{plugins: []config.Plugin{plugin("plugin-a", true), plugin("plugin-b", false), plugin("plugin-c", true)}}, projectSource: nil, expectedItems: []tui.PluginItem{{Name: "plugin-a", InitiallyEnabled: true, SourceLabel: ""}, {Name: "plugin-b", InitiallyEnabled: false, SourceLabel: ""}, {Name: "plugin-c", InitiallyEnabled: true, SourceLabel: ""}}, expectedMerged: []mergedPlugin{{name: "plugin-a", inUser: true, inProject: false}, {name: "plugin-b", inUser: true, inProject: false}, {name: "plugin-c", inUser: true, inProject: false}}},
		{name: "project-only labels are project", userSource: &configSource{plugins: []config.Plugin{}}, projectSource: &configSource{plugins: []config.Plugin{plugin("plugin-a", true), plugin("plugin-b", false), plugin("plugin-c", true)}}, expectedItems: []tui.PluginItem{{Name: "plugin-a", InitiallyEnabled: true, SourceLabel: "Project"}, {Name: "plugin-b", InitiallyEnabled: false, SourceLabel: "Project"}, {Name: "plugin-c", InitiallyEnabled: true, SourceLabel: "Project"}}, expectedMerged: []mergedPlugin{{name: "plugin-a", inUser: false, inProject: true}, {name: "plugin-b", inUser: false, inProject: true}, {name: "plugin-c", inUser: false, inProject: true}}},
		{name: "mixed overlap labels user project and both", userSource: &configSource{plugins: []config.Plugin{plugin("plugin-a", true), plugin("plugin-b", false)}}, projectSource: &configSource{plugins: []config.Plugin{plugin("plugin-b", true), plugin("plugin-c", true)}}, expectedItems: []tui.PluginItem{{Name: "plugin-a", InitiallyEnabled: true, SourceLabel: "User"}, {Name: "plugin-b", InitiallyEnabled: true, SourceLabel: "User, Project"}, {Name: "plugin-c", InitiallyEnabled: true, SourceLabel: "Project"}}, expectedMerged: []mergedPlugin{{name: "plugin-a", inUser: true, inProject: false}, {name: "plugin-b", inUser: true, inProject: true}, {name: "plugin-c", inUser: false, inProject: true}}},
		{name: "both enabled remains enabled", userSource: &configSource{plugins: []config.Plugin{plugin("plugin-b", true)}}, projectSource: &configSource{plugins: []config.Plugin{plugin("plugin-b", true)}}, expectedItems: []tui.PluginItem{{Name: "plugin-b", InitiallyEnabled: true, SourceLabel: "User, Project"}}, expectedMerged: []mergedPlugin{{name: "plugin-b", inUser: true, inProject: true}}},
		{name: "split enabled user true project false still enabled", userSource: &configSource{plugins: []config.Plugin{plugin("plugin-b", true)}}, projectSource: &configSource{plugins: []config.Plugin{plugin("plugin-b", false)}}, expectedItems: []tui.PluginItem{{Name: "plugin-b", InitiallyEnabled: true, SourceLabel: "User, Project"}}, expectedMerged: []mergedPlugin{{name: "plugin-b", inUser: true, inProject: true}}},
		{name: "split enabled user false project true still enabled", userSource: &configSource{plugins: []config.Plugin{plugin("plugin-b", false)}}, projectSource: &configSource{plugins: []config.Plugin{plugin("plugin-b", true)}}, expectedItems: []tui.PluginItem{{Name: "plugin-b", InitiallyEnabled: true, SourceLabel: "User, Project"}}, expectedMerged: []mergedPlugin{{name: "plugin-b", inUser: true, inProject: true}}},
		{name: "empty project source labels user", userSource: &configSource{plugins: []config.Plugin{plugin("plugin-a", true), plugin("plugin-b", false), plugin("plugin-c", true)}}, projectSource: &configSource{plugins: []config.Plugin{}}, expectedItems: []tui.PluginItem{{Name: "plugin-a", InitiallyEnabled: true, SourceLabel: "User"}, {Name: "plugin-b", InitiallyEnabled: false, SourceLabel: "User"}, {Name: "plugin-c", InitiallyEnabled: true, SourceLabel: "User"}}, expectedMerged: []mergedPlugin{{name: "plugin-a", inUser: true, inProject: false}, {name: "plugin-b", inUser: true, inProject: false}, {name: "plugin-c", inUser: true, inProject: false}}},
		{name: "ordering keeps user first then project-only", userSource: &configSource{plugins: []config.Plugin{plugin("plugin-u1", true), plugin("plugin-u2", false)}}, projectSource: &configSource{plugins: []config.Plugin{plugin("plugin-u2", true), plugin("plugin-p1", false), plugin("plugin-p2", true)}}, expectedItems: []tui.PluginItem{{Name: "plugin-u1", InitiallyEnabled: true, SourceLabel: "User"}, {Name: "plugin-u2", InitiallyEnabled: true, SourceLabel: "User, Project"}, {Name: "plugin-p1", InitiallyEnabled: false, SourceLabel: "Project"}, {Name: "plugin-p2", InitiallyEnabled: true, SourceLabel: "Project"}}, expectedMerged: []mergedPlugin{{name: "plugin-u1", inUser: true, inProject: false}, {name: "plugin-u2", inUser: true, inProject: true}, {name: "plugin-p1", inUser: false, inProject: true}, {name: "plugin-p2", inUser: false, inProject: true}}},
		{name: "deduplicates by exact name not comparison name", userSource: &configSource{plugins: []config.Plugin{plugin("oh-my-opencode", true)}}, projectSource: &configSource{plugins: []config.Plugin{plugin("oh-my-opencode@latest", false)}}, expectedItems: []tui.PluginItem{{Name: "oh-my-opencode", InitiallyEnabled: true, SourceLabel: "User"}, {Name: "oh-my-opencode@latest", InitiallyEnabled: false, SourceLabel: "Project"}}, expectedMerged: []mergedPlugin{{name: "oh-my-opencode", inUser: true, inProject: false}, {name: "oh-my-opencode@latest", inUser: false, inProject: true}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, merged := mergePlugins(tt.userSource, tt.projectSource)
			if !reflect.DeepEqual(items, tt.expectedItems) {
				t.Fatalf("items mismatch\nwant: %#v\ngot:  %#v", tt.expectedItems, items)
			}
			if !reflect.DeepEqual(merged, tt.expectedMerged) {
				t.Fatalf("merged mismatch\nwant: %#v\ngot:  %#v", tt.expectedMerged, merged)
			}
		})
	}
}

func TestPersistSelections_UserOnlyPluginWritesOnlyUserFile(t *testing.T) {
	const userPath = "/tmp/user-opencode.json"
	const projectPath = "/tmp/project-opencode.json"
	state := iterationState{userSource: &configSource{path: userPath, content: []byte("{\n  \"plugin\": [\n    \"user-only\"\n  ]\n}\n")}, projectSource: &configSource{path: projectPath, content: []byte("{\n  \"plugin\": [\n    \"project-only\"\n  ]\n}\n")}, mergedPlugins: []mergedPlugin{{name: "user-only", inUser: true, inProject: false}, {name: "project-only", inUser: false, inProject: true}}}
	var writtenPaths []string
	var writtenContents [][]byte
	deps := RuntimeDeps{ApplySelections: config.ApplySelections, WriteConfigFile: func(path string, content []byte) error {
		writtenPaths = append(writtenPaths, path)
		writtenContents = append(writtenContents, append([]byte(nil), content...))
		return nil
	}}
	err := persistSelections(deps, state, map[string]bool{"user-only": false})
	if err != nil {
		t.Fatalf("persistSelections returned error: %v", err)
	}
	if len(writtenPaths) != 1 || writtenPaths[0] != userPath {
		t.Fatalf("expected only user file write, got paths=%v", writtenPaths)
	}
	if !strings.Contains(string(writtenContents[0]), "// \"user-only\"") {
		t.Fatalf("expected user-only to be disabled in user file, got:\n%s", string(writtenContents[0]))
	}
}

func TestPersistSelections_ProjectOnlyPluginWritesOnlyProjectFile(t *testing.T) {
	const userPath = "/tmp/user-opencode.json"
	const projectPath = "/tmp/project-opencode.json"
	state := iterationState{userSource: &configSource{path: userPath, content: []byte("{\n  \"plugin\": [\n    \"user-only\"\n  ]\n}\n")}, projectSource: &configSource{path: projectPath, content: []byte("{\n  \"plugin\": [\n    // \"project-only\"\n  ]\n}\n")}, mergedPlugins: []mergedPlugin{{name: "user-only", inUser: true, inProject: false}, {name: "project-only", inUser: false, inProject: true}}}
	var writtenPaths []string
	var writtenContents [][]byte
	deps := RuntimeDeps{ApplySelections: config.ApplySelections, WriteConfigFile: func(path string, content []byte) error {
		writtenPaths = append(writtenPaths, path)
		writtenContents = append(writtenContents, append([]byte(nil), content...))
		return nil
	}}
	err := persistSelections(deps, state, map[string]bool{"project-only": true})
	if err != nil {
		t.Fatalf("persistSelections returned error: %v", err)
	}
	if len(writtenPaths) != 1 || writtenPaths[0] != projectPath {
		t.Fatalf("expected only project file write, got paths=%v", writtenPaths)
	}
	content := string(writtenContents[0])
	if strings.Contains(content, "// \"project-only\"") || !strings.Contains(content, "\"project-only\"") {
		t.Fatalf("expected project-only to be enabled in project file, got:\n%s", content)
	}
}

func TestPersistSelections_SharedPluginWritesBothFiles(t *testing.T) {
	const userPath = "/tmp/user-opencode.json"
	const projectPath = "/tmp/project-opencode.json"
	state := iterationState{userSource: &configSource{path: userPath, content: []byte("{\n  \"plugin\": [\n    \"shared\"\n  ]\n}\n")}, projectSource: &configSource{path: projectPath, content: []byte("{\n  \"plugin\": [\n    \"shared\"\n  ]\n}\n")}, mergedPlugins: []mergedPlugin{{name: "shared", inUser: true, inProject: true}}}
	var writtenPaths []string
	var writtenContents [][]byte
	deps := RuntimeDeps{ApplySelections: config.ApplySelections, WriteConfigFile: func(path string, content []byte) error {
		writtenPaths = append(writtenPaths, path)
		writtenContents = append(writtenContents, append([]byte(nil), content...))
		return nil
	}}
	err := persistSelections(deps, state, map[string]bool{"shared": false})
	if err != nil {
		t.Fatalf("persistSelections returned error: %v", err)
	}
	if len(writtenPaths) != 2 {
		t.Fatalf("expected dual writes, got paths=%v", writtenPaths)
	}
	if writtenPaths[0] != userPath || writtenPaths[1] != projectPath {
		t.Fatalf("expected user then project writes, got paths=%v", writtenPaths)
	}
	if !strings.Contains(string(writtenContents[0]), "// \"shared\"") {
		t.Fatalf("expected shared plugin to be disabled in user file, got:\n%s", string(writtenContents[0]))
	}
	if !strings.Contains(string(writtenContents[1]), "// \"shared\"") {
		t.Fatalf("expected shared plugin to be disabled in project file, got:\n%s", string(writtenContents[1]))
	}
}

func TestPersistSelections_NoProjectSourceRemainsBackwardCompatible(t *testing.T) {
	const userPath = "/tmp/user-opencode.json"
	state := iterationState{userSource: &configSource{path: userPath, content: []byte("{\n  \"plugin\": [\n    // \"user-only\"\n  ]\n}\n")}, projectSource: nil, mergedPlugins: []mergedPlugin{{name: "user-only", inUser: true, inProject: false}}}
	var writtenPaths []string
	var writtenContents [][]byte
	deps := RuntimeDeps{ApplySelections: config.ApplySelections, WriteConfigFile: func(path string, content []byte) error {
		writtenPaths = append(writtenPaths, path)
		writtenContents = append(writtenContents, append([]byte(nil), content...))
		return nil
	}}
	err := persistSelections(deps, state, map[string]bool{"user-only": true})
	if err != nil {
		t.Fatalf("persistSelections returned error: %v", err)
	}
	if len(writtenPaths) != 1 || writtenPaths[0] != userPath {
		t.Fatalf("expected single user write with no project source, got paths=%v", writtenPaths)
	}
	content := string(writtenContents[0])
	if strings.Contains(content, "// \"user-only\"") || !strings.Contains(content, "\"user-only\"") {
		t.Fatalf("expected user-only to be enabled in user file, got:\n%s", content)
	}
}

func TestRunLaunchTUI_ReturnsPortArgsWithoutRenderer(t *testing.T) {
	tmp := t.TempDir()
	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)

	portArgs, err := runLaunchTUI([]string{"oh-my-opencode"}, tui.SessionItem{}, "50000-55000", deps, "test")
	if err != nil {
		t.Fatalf("runLaunchTUI returned error: %v", err)
	}
	if len(portArgs) != 2 || portArgs[0] != "--port" || portArgs[1] != "51234" {
		t.Fatalf("expected --port 51234, got %v", portArgs)
	}
}

func TestBuildEditChoices_ProjectConfigExists(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	projectDir := filepath.Join(tmp, ".opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userOhMyPath := filepath.Join(configDir, "oh-my-opencode.jsonc")
	projectOhMyPath := filepath.Join(projectDir, "oh-my-openagent.json")
	if err := os.WriteFile(userOhMyPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectOhMyPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ocConfigPath := filepath.Join(tmp, ".oc")
	projectConfigPath := filepath.Join(projectDir, "opencode.json")

	paths := runtimePaths{
		ocConfigPath: ocConfigPath,
		configPath:   filepath.Join(configDir, "opencode.json"),
		configDir:    configDir,
	}

	choices := buildEditChoices(paths, projectConfigPath, true)

	if len(choices) != 5 {
		t.Fatalf("expected 5 edit choices when project config and oh-my configs exist, got %d", len(choices))
	}
	if choices[0].Label != "1) .oc file" {
		t.Fatalf("expected first choice label '1) .oc file', got %q", choices[0].Label)
	}
	if choices[0].Path != ocConfigPath {
		t.Fatalf("expected first choice path %q, got %q", ocConfigPath, choices[0].Path)
	}
	if choices[1].Label != "2) opencode.json file" {
		t.Fatalf("expected second choice label '2) opencode.json file', got %q", choices[1].Label)
	}
	if choices[2].Path != userOhMyPath {
		t.Fatalf("expected third choice path %q, got %q", userOhMyPath, choices[2].Path)
	}
	if choices[3].Label != "4) project opencode.json file" {
		t.Fatalf("expected fourth choice label '4) project opencode.json file', got %q", choices[3].Label)
	}
	if choices[3].Path != projectConfigPath {
		t.Fatalf("expected fourth choice path %q, got %q", projectConfigPath, choices[3].Path)
	}
	if choices[4].Path != projectOhMyPath {
		t.Fatalf("expected fifth choice path %q, got %q", projectOhMyPath, choices[4].Path)
	}
	if choices[4].Label != "5) project oh-my-openagent.json file" {
		t.Fatalf("expected fifth choice label '5) project oh-my-openagent.json file', got %q", choices[4].Label)
	}
}

func TestBuildEditChoices_ProjectConfigAbsent(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userOhMyPath := filepath.Join(configDir, "oh-my-openagent.json")
	if err := os.WriteFile(userOhMyPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ocConfigPath := filepath.Join(tmp, ".oc")

	paths := runtimePaths{
		ocConfigPath: ocConfigPath,
		configPath:   filepath.Join(configDir, "opencode.json"),
		configDir:    configDir,
	}
	projectPath := "/tmp/project-opencode.json"

	choices := buildEditChoices(paths, projectPath, false)

	if len(choices) != 3 {
		t.Fatalf("expected 3 edit choices when project config is absent and one user oh-my config exists, got %d", len(choices))
	}
	if choices[0].Label != "1) .oc file" {
		t.Fatalf("expected first choice label '1) .oc file', got %q", choices[0].Label)
	}
	if choices[0].Path != ocConfigPath {
		t.Fatalf("expected first choice path %q, got %q", ocConfigPath, choices[0].Path)
	}
	if choices[2].Path != userOhMyPath {
		t.Fatalf("expected third choice path %q, got %q", userOhMyPath, choices[2].Path)
	}
	if choices[2].Label != "3) oh-my-openagent.json file" {
		t.Fatalf("expected third choice label '3) oh-my-openagent.json file', got %q", choices[2].Label)
	}
}
