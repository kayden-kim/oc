package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tidwall/jsonc"
)

func TestApplySelections_ActiveToCommented(t *testing.T) {
	input := []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    \"plugin-b\"\n  ]\n}\n")

	result, err := ApplySelections(input, map[string]bool{"plugin-a": false})
	if err != nil {
		t.Fatalf("ApplySelections returned error: %v", err)
	}

	output := string(result)
	if !strings.Contains(output, "    // \"plugin-a\",") {
		t.Fatalf("expected plugin-a to be commented, got:\n%s", output)
	}
	if !strings.Contains(output, "    \"plugin-b\"") {
		t.Fatalf("expected plugin-b unchanged, got:\n%s", output)
	}
}

func TestApplySelections_CommentedToActive(t *testing.T) {
	input := []byte("{\n  \"plugin\": [\n    // \"plugin-a\",\n    \"plugin-b\"\n  ]\n}\n")

	result, err := ApplySelections(input, map[string]bool{"plugin-a": true})
	if err != nil {
		t.Fatalf("ApplySelections returned error: %v", err)
	}

	output := string(result)
	if strings.Contains(output, "// \"plugin-a\"") {
		t.Fatalf("expected plugin-a to be uncommented, got:\n%s", output)
	}
	if !strings.Contains(output, "    \"plugin-a\",") {
		t.Fatalf("expected uncommented plugin-a line, got:\n%s", output)
	}
}

func TestApplySelections_MixedTogglesAndHiddenPreserved(t *testing.T) {
	input := []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    // \"plugin-b\",\n    \"hidden-plugin\"\n  ]\n}\n")

	result, err := ApplySelections(input, map[string]bool{
		"plugin-a": false,
		"plugin-b": true,
	})
	if err != nil {
		t.Fatalf("ApplySelections returned error: %v", err)
	}

	output := string(result)
	if !strings.Contains(output, "    // \"plugin-a\",") {
		t.Fatalf("expected plugin-a to be commented, got:\n%s", output)
	}
	if !strings.Contains(output, "    \"plugin-b\",") {
		t.Fatalf("expected plugin-b to be uncommented, got:\n%s", output)
	}
	if !strings.Contains(output, "    \"hidden-plugin\"") {
		t.Fatalf("expected hidden plugin preserved, got:\n%s", output)
	}
}

func TestApplySelections_IndentationPreservedOnComment(t *testing.T) {
	input := []byte("{\n  \"plugin\": [\n      \"plugin-a\"\n  ]\n}\n")

	result, err := ApplySelections(input, map[string]bool{"plugin-a": false})
	if err != nil {
		t.Fatalf("ApplySelections returned error: %v", err)
	}

	if !strings.Contains(string(result), "      // \"plugin-a\"") {
		t.Fatalf("expected indentation to be preserved, got:\n%s", string(result))
	}
}

func TestApplySelections_LineEndingPreservedCRLF(t *testing.T) {
	input := []byte("{\r\n  \"plugin\": [\r\n    \"plugin-a\",\r\n    // \"plugin-b\"\r\n  ]\r\n}\r\n")

	result, err := ApplySelections(input, map[string]bool{"plugin-a": false, "plugin-b": true})
	if err != nil {
		t.Fatalf("ApplySelections returned error: %v", err)
	}

	if !bytes.Contains(result, []byte("\r\n")) {
		t.Fatalf("expected CRLF line endings to be preserved")
	}
	if bytes.Contains(result, []byte("\n")) && !bytes.Contains(result, []byte("\r\n")) {
		t.Fatalf("expected no LF-only endings in output")
	}
}

func TestApplySelections_NonPluginContentPreserved(t *testing.T) {
	input := []byte("{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"mcp\": {\n    \"server\": {\n      \"command\": \"node\"\n    }\n  },\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n")

	result, err := ApplySelections(input, map[string]bool{"plugin-a": false})
	if err != nil {
		t.Fatalf("ApplySelections returned error: %v", err)
	}

	output := string(result)
	if !strings.Contains(output, "\"$schema\": \"https://opencode.ai/config.json\"") {
		t.Fatalf("expected schema line preserved, got:\n%s", output)
	}
	if !strings.Contains(output, "\"mcp\": {") || !strings.Contains(output, "\"command\": \"node\"") {
		t.Fatalf("expected mcp section preserved, got:\n%s", output)
	}
}

func TestApplySelections_OutputIsValidJSONC(t *testing.T) {
	input := []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    // \"plugin-b\"\n  ]\n}\n")

	result, err := ApplySelections(input, map[string]bool{"plugin-a": false, "plugin-b": true})
	if err != nil {
		t.Fatalf("ApplySelections returned error: %v", err)
	}

	jsonBytes := jsonc.ToJSON(result)
	var decoded any
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("expected valid JSONC output, unmarshal failed: %v", err)
	}
}

func TestApplySelections_TrailingCommaPreserved(t *testing.T) {
	input := []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    \"plugin-b\"\n  ]\n}\n")

	result, err := ApplySelections(input, map[string]bool{"plugin-a": false, "plugin-b": false})
	if err != nil {
		t.Fatalf("ApplySelections returned error: %v", err)
	}

	output := string(result)
	if !strings.Contains(output, "    // \"plugin-a\",") {
		t.Fatalf("expected trailing comma preserved for plugin-a, got:\n%s", output)
	}
	if !strings.Contains(output, "    // \"plugin-b\"") {
		t.Fatalf("expected no comma introduced for last element plugin-b, got:\n%s", output)
	}
}

func TestApplySelections_CommentSpacingNormalizedOnComment(t *testing.T) {
	input := []byte("{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n")

	result, err := ApplySelections(input, map[string]bool{"plugin-a": false})
	if err != nil {
		t.Fatalf("ApplySelections returned error: %v", err)
	}

	if !strings.Contains(string(result), "// \"plugin-a\"") {
		t.Fatalf("expected normalized comment spacing, got:\n%s", string(result))
	}
}

func TestApplySelections_UncommentHandlesExtraCommentSpacing(t *testing.T) {
	input := []byte("{\n  \"plugin\": [\n    //  \"plugin-a\"\n  ]\n}\n")

	result, err := ApplySelections(input, map[string]bool{"plugin-a": true})
	if err != nil {
		t.Fatalf("ApplySelections returned error: %v", err)
	}

	output := string(result)
	if strings.Contains(output, "//") {
		t.Fatalf("expected comment marker removed when enabling plugin, got:\n%s", output)
	}
	if !strings.Contains(output, "    \"plugin-a\"") {
		t.Fatalf("expected plugin line to stay properly indented, got:\n%s", output)
	}
}

func TestWriteConfigFile_AtomicCreateAndOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")

	first := []byte("{\n  \"plugin\": [\n    \"a\"\n  ]\n}\n")
	if err := WriteConfigFile(path, first); err != nil {
		t.Fatalf("WriteConfigFile create failed: %v", err)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after create failed: %v", err)
	}
	if !bytes.Equal(written, first) {
		t.Fatalf("create content mismatch: got %q want %q", written, first)
	}

	second := []byte("{\n  \"plugin\": [\n    \"b\"\n  ]\n}\n")
	if err := WriteConfigFile(path, second); err != nil {
		t.Fatalf("WriteConfigFile overwrite failed: %v", err)
	}

	written, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after overwrite failed: %v", err)
	}
	if !bytes.Equal(written, second) {
		t.Fatalf("overwrite content mismatch: got %q want %q", written, second)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "opencode.json.tmp.") {
			t.Fatalf("unexpected temp file left behind: %s", entry.Name())
		}
	}
}
