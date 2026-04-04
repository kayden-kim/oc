package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/tui"
)

func TestRunWithDeps_EditRequestOpensEditorAndReturnsToTUI(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "opencode.json")
	initial := "{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	ed := &fakeEditor{}

	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: scriptedTUI(t,
			tuiResponse{editTarget: configPath},
			tuiResponse{cancelled: true},
		),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      ed.Open,
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !ed.opened {
		t.Fatal("expected editor to be opened")
	}
	if ed.path != configPath {
		t.Fatalf("expected editor path %q, got %q", configPath, ed.path)
	}
	if ed.configEditor != "" {
		t.Fatalf("expected empty config editor, got %q", ed.configEditor)
	}
	if r.ran {
		t.Fatal("runner should not execute when edit is requested")
	}

	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(updated) != initial {
		t.Fatalf("file should remain unchanged on edit request\nwant:\n%s\ngot:\n%s", initial, string(updated))
	}
}

func TestRunWithDeps_PassesOcConfigEditorToEditor(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "opencode.json")
	if err := os.WriteFile(configPath, []byte("{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ocConfigPath := filepath.Join(tmp, ".oc")
	if err := os.WriteFile(ocConfigPath, []byte("editor = \"code --goto\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	ed := &fakeEditor{}

	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: scriptedTUI(t,
			tuiResponse{editTarget: configPath},
			tuiResponse{cancelled: true},
		),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      ed.Open,
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !ed.opened {
		t.Fatal("expected editor to be opened")
	}
	if ed.configEditor != "code --goto" {
		t.Fatalf("expected config editor to be passed through, got %q", ed.configEditor)
	}
	if r.ran {
		t.Fatal("runner should not execute when edit is requested")
	}
}

func TestRunWithDeps_PassesResolvedEditChoicesToTUI(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "opencode.json")
	if err := os.WriteFile(configPath, []byte("{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jsoncPath := filepath.Join(configDir, "oh-my-openagent.jsonc")
	if err := os.WriteFile(jsoncPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	var gotChoices []tui.EditChoice
	tuiCalls := 0

	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			tuiCalls++
			gotChoices = append([]tui.EditChoice(nil), editChoices...)
			if tuiCalls == 1 {
				return nil, false, filepath.Join(tmp, ".oc"), nil, nil
			}
			return nil, true, "", nil, nil
		}),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}

	if len(gotChoices) != 3 {
		t.Fatalf("expected 3 edit choices, got %d", len(gotChoices))
	}
	if gotChoices[0].Path != filepath.Join(tmp, ".oc") {
		t.Fatalf("expected first choice to target .oc, got %q", gotChoices[0].Path)
	}
	if gotChoices[1].Path != configPath {
		t.Fatalf("expected second choice to target opencode.json, got %q", gotChoices[1].Path)
	}
	wantOhMyPath := ResolveOhMyOpencodePath(configDir)
	if gotChoices[2].Path != wantOhMyPath {
		t.Fatalf("expected third choice to target resolved oh-my config %q, got %q", wantOhMyPath, gotChoices[2].Path)
	}
	if gotChoices[2].Path != jsoncPath {
		t.Fatalf("expected resolver to recognize openagent jsonc path %q, got %q", jsoncPath, gotChoices[2].Path)
	}
	if r.ran {
		t.Fatal("runner should not execute when edit is requested")
	}
}

func TestRunWithDeps_PassesProjectEditChoicePathToTUI(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	projectDir := filepath.Join(tmp, ".opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "opencode.json")
	if err := os.WriteFile(configPath, []byte("{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectConfigPath := filepath.Join(projectDir, "opencode.json")
	if err := os.WriteFile(projectConfigPath, []byte("{\n  \"plugin\": [\n    \"plugin-b\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	var gotChoices []tui.EditChoice

	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		Getwd:             func() (string, error) { return tmp, nil },
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			gotChoices = append([]tui.EditChoice(nil), editChoices...)
			return nil, true, "", nil, nil
		}),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}

	if len(gotChoices) != 4 {
		t.Fatalf("expected 4 edit choices, got %d", len(gotChoices))
	}
	if gotChoices[3].Label != "4) project opencode.json file" {
		t.Fatalf("expected fourth choice label '4) project opencode.json file', got %q", gotChoices[3].Label)
	}
	if gotChoices[3].Path != projectConfigPath {
		t.Fatalf("expected fourth choice path %q, got %q", projectConfigPath, gotChoices[3].Path)
	}
	if r.ran {
		t.Fatal("runner should not execute when TUI cancels")
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
