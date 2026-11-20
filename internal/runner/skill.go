package runner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"agent-tools-runner/internal/prompt"
)

func isValidSkillName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\:`)
}

func executeReadSkillAction(workspace string, action Action, actionIndex int) ([]ReadFileResult, *ResponseError) {
	fileName := action.Path
	if fileName == "" {
		fileName = prompt.SkillFileName
	}
	if !filepath.IsLocal(fileName) {
		return nil, newActionResponseError(action, actionIndex, "PATH_OUTSIDE_WORKSPACE", "Path is outside the skill folder.", filepath.ToSlash(fileName))
	}

	roots := []struct{ directory, prefix string }{{workspace, ""}}
	if homeDirectory, err := os.UserHomeDir(); err == nil {
		roots = append(roots, struct{ directory, prefix string }{homeDirectory, "~/"})
	}
	skillPath := filepath.Join(AgentsDirectoryName, prompt.SkillsDirectoryName, action.Name)
	for _, root := range roots {
		if _, err := os.Lstat(filepath.Join(root.directory, skillPath)); errors.Is(err, os.ErrNotExist) {
			continue
		}
		file, err := readWorkspaceFile(root.directory, filepath.Join(skillPath, fileName))
		if err != nil {
			return nil, workspaceResponseError(action, actionIndex, err, "READ_FAILED", "Could not read the requested skill file.", filepath.ToSlash(skillPath))
		}
		file.Path = root.prefix + file.Path
		return []ReadFileResult{file}, nil
	}
	return nil, newActionResponseError(action, actionIndex, "FILE_NOT_FOUND", "Skill was not found.", action.Name)
}
