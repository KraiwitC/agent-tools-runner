package runner

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadConfigMissing(t *testing.T) {
	workspace := t.TempDir()

	config, found, err := LoadConfig(workspace)
	if err != nil {
		t.Fatalf("LoadConfig returned an error: %v", err)
	}
	if found {
		t.Fatal("expected found == false")
	}
	if len(config.Bash.Allow) != 0 {
		t.Fatalf("expected empty allowlist, got %v", config.Bash.Allow)
	}
}

func TestLoadConfigMissingFileInExistingAgentsDirectory(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, AgentsDirectoryName), 0o700); err != nil {
		t.Fatalf("create agents directory: %v", err)
	}

	_, found, err := LoadConfig(workspace)
	if err != nil {
		t.Fatalf("LoadConfig returned an error: %v", err)
	}
	if found {
		t.Fatal("expected found == false")
	}
}

func TestLoadConfigReadsAllowlist(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, ".agents/atr.json", `{"bash":{"allow":["go test","git status"]}}`)

	config, found, err := LoadConfig(workspace)
	if err != nil {
		t.Fatalf("LoadConfig returned an error: %v", err)
	}
	if !found {
		t.Fatal("expected found == true")
	}
	expected := []string{"go test", "git status"}
	if !reflect.DeepEqual(config.Bash.Allow, expected) {
		t.Fatalf("allowlist = %v, want %v", config.Bash.Allow, expected)
	}
}

func TestLoadConfigAcceptsEmptyAllowlist(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, ".agents/atr.json", `{"bash":{"allow":[]}}`)

	config, found, err := LoadConfig(workspace)
	if err != nil {
		t.Fatalf("LoadConfig returned an error: %v", err)
	}
	if !found || len(config.Bash.Allow) != 0 {
		t.Fatalf("found = %v, allowlist = %v", found, config.Bash.Allow)
	}
}

func TestLoadConfigRejectsInvalidContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "malformed JSON", content: `{"bash":`},
		{name: "unknown top-level field", content: `{"bash":{"allow":[]},"skills":["x"]}`},
		{name: "unknown bash field", content: `{"bash":{"allow":[],"deny":[]}}`},
		{name: "wrong allow type", content: `{"bash":{"allow":"go test"}}`},
		{name: "blank entry", content: `{"bash":{"allow":["go test","  "]}}`},
		{name: "trailing data", content: `{"bash":{"allow":[]}} {}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			writeTestFile(t, workspace, ".agents/atr.json", test.content)

			_, found, err := LoadConfig(workspace)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !found {
				t.Fatal("expected found == true for an invalid existing file")
			}
		})
	}
}

func TestLoadConfigRejectsSymbolicLinkConfigFile(t *testing.T) {
	workspace := t.TempDir()
	target := writeTestFile(t, workspace, "elsewhere.json", `{"bash":{"allow":["go"]}}`)
	if err := os.Mkdir(filepath.Join(workspace, AgentsDirectoryName), 0o700); err != nil {
		t.Fatalf("create agents directory: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(workspace, AgentsDirectoryName, ConfigFileName)); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}

	_, _, err := LoadConfig(workspace)
	if err == nil {
		t.Fatal("expected a symbolic link error")
	}
}

func TestLoadConfigRejectsSymbolicLinkAgentsDirectory(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "real/atr.json", `{"bash":{"allow":["go"]}}`)
	if err := os.Symlink(filepath.Join(workspace, "real"), filepath.Join(workspace, AgentsDirectoryName)); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}

	_, _, err := LoadConfig(workspace)
	if err == nil {
		t.Fatal("expected a symbolic link error")
	}
}

func TestExecuteActionRejectsModifyingProtectedPaths(t *testing.T) {
	const zeroHash = "0000000000000000000000000000000000000000000000000000000000000000"
	tests := []struct {
		name   string
		action Action
	}{
		{name: "create config", action: Action{Operation: "create", Path: ".agents/atr.json", Content: "{}"}},
		{name: "create skill", action: Action{Operation: "create", Path: ".agents/skills/x/SKILL.md", Content: "x"}},
		{name: "mkdir agents", action: Action{Operation: "mkdir", Path: ".agents"}},
		{name: "edit config", action: Action{Operation: "edit", Path: ".agents/atr.json", ExpectedSHA256: zeroHash, Replacements: []Replacement{{OldText: "a", NewText: "b"}}}},
		{name: "delete config", action: Action{Operation: "delete", Path: ".agents/atr.json", ExpectedSHA256: zeroHash}},
		{name: "delete agents directory", action: Action{Operation: "delete", Path: ".agents"}},
		{name: "copy destination", action: Action{Operation: "copy", Source: "a.txt", Destination: ".agents/atr.json", ExpectedSHA256: zeroHash}},
		{name: "move destination", action: Action{Operation: "move", Source: "a.txt", Destination: ".agents/atr.json", ExpectedSHA256: zeroHash}},
		{name: "copy source", action: Action{Operation: "copy", Source: ".agents/atr.json", Destination: "b.txt", ExpectedSHA256: zeroHash}},
		{name: "move source", action: Action{Operation: "move", Source: ".agents/atr.json", Destination: "b.txt", ExpectedSHA256: zeroHash}},
		{name: "upper case", action: Action{Operation: "create", Path: ".AGENTS/atr.json", Content: "{}"}},
		{name: "dot prefix", action: Action{Operation: "create", Path: "./.agents/atr.json", Content: "{}"}},
		{name: "parent traversal", action: Action{Operation: "create", Path: "docs/../.agents/atr.json", Content: "{}"}},
		{name: "trailing dot", action: Action{Operation: "create", Path: ".agents./atr.json", Content: "{}"}},
		{name: "trailing space", action: Action{Operation: "create", Path: ".agents /atr.json", Content: "{}"}},
		{name: "stream suffix", action: Action{Operation: "create", Path: ".agents:stream/atr.json", Content: "{}"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			writeTestFile(t, workspace, "a.txt", "a")
			test.action.ID = "protected"

			_, responseError := executeAction(workspace, test.action, 0, io.Discard)
			assertResponseErrorCode(t, responseError, "PROTECTED_PATH")
			assertPathDoesNotExist(t, filepath.Join(workspace, AgentsDirectoryName))
			assertPathDoesNotExist(t, filepath.Join(workspace, "b.txt"))
		})
	}
}

func TestExecuteActionRejectsProtectedPathThatLeavesAndReentersWorkspace(t *testing.T) {
	workspace := t.TempDir()
	action := Action{
		ID:        "protected",
		Operation: "create",
		Path:      filepath.Join("..", filepath.Base(workspace), ".agents", "atr.json"),
		Content:   "{}",
	}

	_, responseError := executeAction(workspace, action, 0, io.Discard)
	assertResponseErrorCode(t, responseError, "PROTECTED_PATH")
	assertPathDoesNotExist(t, filepath.Join(workspace, AgentsDirectoryName))
}

func TestExecuteActionRejectsModifyingExistingProtectedFile(t *testing.T) {
	workspace := t.TempDir()
	original := `{"bash":{"allow":[]}}`
	configPath := writeTestFile(t, workspace, ".agents/atr.json", original)
	action := Action{
		ID:             "protected",
		Operation:      "edit",
		Path:           ".agents/atr.json",
		ExpectedSHA256: calculateSHA256([]byte(original)),
		Replacements:   []Replacement{{OldText: "[]", NewText: `["rm"]`}},
	}

	_, responseError := executeAction(workspace, action, 0, io.Discard)
	assertResponseErrorCode(t, responseError, "PROTECTED_PATH")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(content) != original {
		t.Fatalf("config changed to %q", content)
	}
}

func TestExecuteActionAllowsReadingProtectedPaths(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, ".agents/skills/x/SKILL.md", "skill")

	for _, action := range []Action{
		{ID: "read", Operation: "read", Paths: []string{".agents/skills/x/SKILL.md"}},
		{ID: "tree", Operation: "tree", Path: ".agents"},
		{ID: "inspect", Operation: "inspect", Path: ".agents/skills/x/SKILL.md"},
	} {
		if _, responseError := executeAction(workspace, action, 0, io.Discard); responseError != nil {
			t.Fatalf("%s returned an error: %#v", action.Operation, responseError)
		}
	}
}

func TestExecuteActionAllowsSimilarlyNamedPaths(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "create", Operation: "create", Path: ".agentsx/file.txt", Content: "x"}
	if err := os.Mkdir(filepath.Join(workspace, ".agentsx"), 0o700); err != nil {
		t.Fatalf("create directory: %v", err)
	}

	if _, responseError := executeAction(workspace, action, 0, io.Discard); responseError != nil {
		t.Fatalf("create returned an error: %#v", responseError)
	}
}
