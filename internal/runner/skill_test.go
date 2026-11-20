package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func setTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func TestExecuteReadSkillActionPrefersWorkspaceOverHome(t *testing.T) {
	home := setTestHome(t)
	workspace := t.TempDir()
	writeTestFile(t, workspace, ".agents/skills/review/SKILL.md", "workspace\n")
	writeTestFile(t, home, ".agents/skills/review/SKILL.md", "home\n")

	files, responseError := executeReadSkillAction(workspace, Action{ID: "s1", Operation: "read_skill", Name: "review"}, 0)
	if responseError != nil {
		t.Fatalf("executeReadSkillAction returned an error: %#v", responseError)
	}
	if files[0].Path != ".agents/skills/review/SKILL.md" || files[0].Content != "workspace\n" {
		t.Fatalf("unexpected file: %#v", files[0])
	}
}

func TestExecuteReadSkillActionReadsHomeSkillFile(t *testing.T) {
	home := setTestHome(t)
	writeTestFile(t, home, ".agents/skills/review/references/rules.md", "rules\n")

	files, responseError := executeReadSkillAction(t.TempDir(), Action{ID: "s1", Operation: "read_skill", Name: "review", Path: "references/rules.md"}, 0)
	if responseError != nil {
		t.Fatalf("executeReadSkillAction returned an error: %#v", responseError)
	}
	if files[0].Path != "~/.agents/skills/review/references/rules.md" || files[0].Content != "rules\n" {
		t.Fatalf("unexpected file: %#v", files[0])
	}
}

func TestExecuteReadSkillActionRejectsPathOutsideSkill(t *testing.T) {
	setTestHome(t)
	workspace := t.TempDir()
	writeTestFile(t, workspace, ".agents/skills/review/SKILL.md", "skill\n")
	writeTestFile(t, workspace, ".agents/atr.json", "{}")

	_, responseError := executeReadSkillAction(workspace, Action{ID: "s1", Operation: "read_skill", Name: "review", Path: "../../atr.json"}, 0)
	if responseError == nil || responseError.Code != "PATH_OUTSIDE_WORKSPACE" {
		t.Fatalf("expected PATH_OUTSIDE_WORKSPACE, got %#v", responseError)
	}
}

func TestExecuteReadSkillActionReportsMissingSkill(t *testing.T) {
	setTestHome(t)

	_, responseError := executeReadSkillAction(t.TempDir(), Action{ID: "s1", Operation: "read_skill", Name: "missing"}, 0)
	if responseError == nil || responseError.Code != "FILE_NOT_FOUND" {
		t.Fatalf("expected FILE_NOT_FOUND, got %#v", responseError)
	}
}

func TestExecuteReadSkillActionRejectsSymlinkSkill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links require elevated privileges on Windows")
	}
	setTestHome(t)
	workspace := t.TempDir()
	writeTestFile(t, workspace, ".agents/skills/real/SKILL.md", "skill\n")
	if err := os.Symlink(filepath.Join(workspace, ".agents", "skills", "real"), filepath.Join(workspace, ".agents", "skills", "linked")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, responseError := executeReadSkillAction(workspace, Action{ID: "s1", Operation: "read_skill", Name: "linked"}, 0)
	if responseError == nil || responseError.Code != "SYMLINK_NOT_SUPPORTED" {
		t.Fatalf("expected SYMLINK_NOT_SUPPORTED, got %#v", responseError)
	}
}
