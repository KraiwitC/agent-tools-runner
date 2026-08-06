package runner

import (
	"errors"
	"strings"
)

func executeReadRangeAction(workspace string, action Action, actionIndex int) (*ActionData, *ResponseError) {
	file, err := readWorkspaceFile(workspace, action.Path)
	if err != nil {
		return &ActionData{Path: action.Path}, workspaceReadRangeResponseError(action, actionIndex, err)
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

func workspaceReadRangeResponseError(action Action, actionIndex int, err error) *ResponseError {
	var pathError *workspaceError
	if errors.As(err, &pathError) {
		return &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        pathError.Code,
			Message:     pathError.Message,
			Path:        pathError.Path,
		}
	}

	return &ResponseError{
		ActionID:    action.ID,
		ActionIndex: actionIndex,
		Code:        "READ_FAILED",
		Message:     "Could not read the requested file range.",
		Path:        action.Path,
	}
}
