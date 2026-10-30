package prompt

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writePromptTestFile(t *testing.T, workspace string, relativePath string, content string) string {
	t.Helper()
	path := filepath.Join(workspace, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func TestCreateListsAvailableSkillsWithoutLoadingContents(t *testing.T) {
	workspace := t.TempDir()
	writePromptTestFile(t, workspace, ".agents/skills/find-bugs/SKILL.md", "---\nname: find-bugs\ndescription: Find bugs and security issues\n---\nSECRET BODY\n")
	writePromptTestFile(t, workspace, ".agents/skills/plain/SKILL.md", "no frontmatter\n")

	prompt, _, skillNames, warnings, err := Create(workspace, nil)
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if !reflect.DeepEqual(skillNames, []string{"find-bugs", "plain"}) {
		t.Fatalf("skill names = %v", skillNames)
	}
	for _, expected := range []string{
		"## Available Skills",
		"- find-bugs: Find bugs and security issues (.agents/skills/find-bugs/SKILL.md)",
		"- plain (.agents/skills/plain/SKILL.md)",
		"Before starting a task, read the SKILL.md of any skill relevant to it.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt does not contain %q", expected)
		}
	}
	if strings.Contains(prompt, "SECRET BODY") {
		t.Fatal("prompt must not include skill contents")
	}
}

func TestCreateOmitsSkillsSectionWithoutSkills(t *testing.T) {
	workspace := t.TempDir()
	writePromptTestFile(t, workspace, ".agents/skills/empty/notes.txt", "no SKILL.md here")
	writePromptTestFile(t, workspace, ".skills/legacy.md", "legacy skill")

	prompt, _, skillNames, _, err := Create(workspace, nil)
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if len(skillNames) != 0 {
		t.Fatalf("skill names = %v", skillNames)
	}
	if strings.Contains(prompt, "## Available Skills") || strings.Contains(prompt, "legacy skill") {
		t.Fatal("prompt must not contain a skills section or legacy .skills content")
	}
}

func TestCreateSkipsSymbolicLinkSkillsWithWarning(t *testing.T) {
	workspace := t.TempDir()
	target := t.TempDir()
	writePromptTestFile(t, target, "SKILL.md", "---\ndescription: linked\n---\n")
	writePromptTestFile(t, workspace, ".agents/skills/real/SKILL.md", "---\ndescription: real\n---\n")
	if err := os.Symlink(target, filepath.Join(workspace, ".agents", "skills", "linked-folder")); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}
	writePromptTestFile(t, workspace, "outside.md", "x")
	if err := os.MkdirAll(filepath.Join(workspace, ".agents", "skills", "linked-file"), 0o755); err != nil {
		t.Fatalf("create directory: %v", err)
	}
	if err := os.Symlink(filepath.Join(workspace, "outside.md"), filepath.Join(workspace, ".agents", "skills", "linked-file", "SKILL.md")); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}

	prompt, _, skillNames, warnings, err := Create(workspace, nil)
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if !reflect.DeepEqual(skillNames, []string{"real"}) {
		t.Fatalf("skill names = %v", skillNames)
	}
	if len(warnings) != 2 {
		t.Fatalf("expected two warnings, got %v", warnings)
	}
	if strings.Contains(prompt, "linked") {
		t.Fatal("prompt must not list symbolic link skills")
	}
}

func TestParseSkillDescription(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "plain", content: "---\nname: a\ndescription: Does things\n---\n", want: "Does things"},
		{name: "double quoted", content: "---\ndescription: \"Does: things\"\n---\n", want: "Does: things"},
		{name: "single quoted", content: "---\ndescription: 'Does things'\n---\n", want: "Does things"},
		{name: "extra whitespace", content: "---\ndescription:   Does    things  \n---\n", want: "Does things"},
		{name: "crlf", content: "---\r\ndescription: Does things\r\n---\r\n", want: "Does things"},
		{name: "byte order mark", content: "\ufeff---\ndescription: Does things\n---\n", want: "Does things"},
		{name: "no frontmatter", content: "description: Does things\n", want: ""},
		{name: "unclosed frontmatter", content: "---\ndescription: Does things\n", want: ""},
		{name: "description after closing", content: "---\nname: a\n---\ndescription: Does things\n", want: ""},
		{name: "block scalar", content: "---\ndescription: >\n  Does things\n---\n", want: ""},
		{name: "missing description", content: "---\nname: a\n---\n", want: ""},
		{name: "empty", content: "", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseSkillDescription(strings.NewReader(test.content)); got != test.want {
				t.Fatalf("parseSkillDescription = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCreateListsAllowedBashCommands(t *testing.T) {
	prompt, _, _, _, err := Create(t.TempDir(), []string{"go test", "git status"})
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	for _, expected := range []string{
		"### Execution Operations",
		"bash: Run an allowlisted command",
		"- go test\n",
		"- git status\n",
		"add it to bash.allow in .agents/atr.json",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt does not contain %q", expected)
		}
	}
	if strings.Contains(prompt, "bash is disabled") {
		t.Fatal("prompt must not say bash is disabled when commands are allowed")
	}
}

func TestCreateStatesBashIsDisabledWithoutAllowlist(t *testing.T) {
	for _, allow := range [][]string{nil, {}} {
		prompt, _, _, _, err := Create(t.TempDir(), allow)
		if err != nil {
			t.Fatalf("Create returned an error: %v", err)
		}
		if !strings.Contains(prompt, "bash is disabled") {
			t.Fatal("prompt must say bash is disabled")
		}
		if strings.Contains(prompt, "bash: Run an allowlisted command") {
			t.Fatal("prompt must not document bash usage when it is disabled")
		}
	}
}

func TestCreateIncludesPromptGuidance(t *testing.T) {
	prompt, _, _, _, err := Create(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	for _, expected := range []string{
		"<a href=...>",
		"Do not create a file and then edit it in the same request.",
		"The .agents and .git folders are read-only",
		"- BASH_NOT_ALLOWED:",
		"- BASH_FAILED:",
		"- BASH_TIMEOUT:",
		"- PROTECTED_PATH:",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt does not contain %q", expected)
		}
	}
	if strings.Contains(prompt, "Loaded Skills") {
		t.Fatal("prompt must not contain the old Loaded Skills section")
	}
}

func TestCreateIncludesProjectInstructionsAndPlan(t *testing.T) {
	workspace := t.TempDir()
	writePromptTestFile(t, workspace, "AGENTS.md", "be careful")
	writePromptTestFile(t, workspace, "PLAN.md", "- [ ] step")

	prompt, agentsFileFound, _, _, err := Create(workspace, nil)
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if !agentsFileFound {
		t.Fatal("expected AGENTS.md to be found")
	}
	for _, expected := range []string{"## Project Guidelines (AGENTS.md)", "be careful", "## Active Task Plan (PLAN.md)", "- [ ] step"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt does not contain %q", expected)
		}
	}
}
