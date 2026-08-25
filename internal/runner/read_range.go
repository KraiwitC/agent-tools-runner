package runner

import (
	"path/filepath"
	"strings"
)

func executeReadRangeAction(workspace string, action Action, actionIndex int) (*ActionData, *ResponseError) {
	cleanPath := filepath.ToSlash(filepath.Clean(action.Path))
	file, err := readWorkspaceFile(workspace, action.Path)
	if err != nil {
		return &ActionData{Path: cleanPath}, workspaceResponseError(action, actionIndex, err, "READ_FAILED", "Could not read the requested file range.", cleanPath)
	}

	lines := splitFileLines(file.Content)
	totalLines := len(lines)
	if action.StartLine > totalLines {
		return &ActionData{
			Path:       file.Path,
			StartLine:  action.StartLine,
			EndLine:    action.EndLine,
			TotalLines: totalLines,
			SHA256:     file.SHA256,
		}, &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "RANGE_OUT_OF_BOUNDS",
			Message:     "startLine exceeds the file line count.",
			Path:        file.Path,
		}
	}

	endLine := action.EndLine
	if endLine > totalLines {
		endLine = totalLines
	}

	return &ActionData{
		Path:       file.Path,
		Content:    strings.Join(lines[action.StartLine-1:endLine], ""),
		StartLine:  action.StartLine,
		EndLine:    endLine,
		TotalLines: totalLines,
		SHA256:     file.SHA256,
	}, nil
}

func splitFileLines(content string) []string {
	if content == "" {
		return []string{}
	}

	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
