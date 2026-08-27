package runner

import (
	"fmt"
	"path/filepath"
	"strings"
)

func MaximumTransferChars() int {
	return maximumTransferChars
}

func SetMaximumTransferChars(value int) {
	maximumTransferChars = value
}

func finishActionError(response Response, result ActionResult, responseError *ResponseError, action Action, actionIndex int, maxTransferChars int) string {
	response.Results = append(response.Results, result)
	response.Status = "error"
	response.Error = responseError

	responseText := marshalResponse(response)
	if len(responseText) <= maxTransferChars {
		return responseText
	}

	response.Results = response.Results[:len(response.Results)-1]
	return createTransferLimitResponse(response, action, actionIndex, maxTransferChars)
}

func fitSuccessfulResult(response Response, result ActionResult, action Action, actionIndex int, maxTransferChars int) (ActionResult, bool) {
	limitResponse := response
	limitResponse.Status = "limit"
	limitResponse.Error = newTransferLimitError(action, actionIndex)
	return fitReadOnlyResult(limitResponse, result, maxTransferChars)
}

func createTransferLimitResponse(response Response, action Action, actionIndex int, maxTransferChars int) string {
	response.Status = "limit"
	response.Error = newTransferLimitError(action, actionIndex)
	responseText := marshalResponse(response)
	if len(responseText) <= maxTransferChars {
		return responseText
	}
	return createTransferLimitErrorResponse(maxTransferChars)
}

func newTransferLimitError(action Action, actionIndex int) *ResponseError {
	return &ResponseError{
		ActionID:    action.ID,
		ActionIndex: actionIndex,
		Code:        "TRANSFER_LIMIT_EXCEEDED",
		Message:     "Response reached the configured transfer limit. The current result was truncated; any remaining actions were not executed.",
	}
}

func isModifyingOperation(operation string) bool {
	switch operation {
	case "edit", "create", "copy", "move", "mkdir", "delete":
		return true
	default:
		return false
	}
}

func modifyingActionResultFits(response Response, action Action, actionIndex int, maxTransferChars int) bool {
	candidate := response
	candidate.Status = "limit"
	candidate.Results = append(append([]ActionResult(nil), response.Results...), maximumModifyingActionResult(action))
	candidate.Error = newTransferLimitError(action, actionIndex)
	return len(marshalResponse(candidate)) <= maxTransferChars
}

func maximumModifyingActionResult(action Action) ActionResult {
	result := ActionResult{
		ID:        action.ID,
		Operation: action.Operation,
		Status:    "success",
		Data:      &ActionData{},
	}

	switch action.Operation {
	case "edit":
		result.Data.Path = filepath.ToSlash(filepath.Clean(action.Path))
		result.Data.SHA256 = strings.Repeat("0", sha256HexLength)
		result.Data.ReplacementsApplied = maximumEditReplacements
	case "create":
		result.Data.Path = filepath.ToSlash(filepath.Clean(action.Path))
		result.Data.SHA256 = strings.Repeat("0", sha256HexLength)
		result.Data.BytesWritten = int(maximumFileSize)
	case "copy", "move":
		result.Data.Path = filepath.ToSlash(filepath.Clean(action.Destination))
		result.Data.SHA256 = strings.Repeat("0", sha256HexLength)
		result.Data.BytesWritten = int(maximumFileSize)
	case "mkdir":
		result.Data.Path = filepath.ToSlash(filepath.Clean(action.Path))
		result.Data.Type = "directory"
	case "delete":
		result.Data.Path = filepath.ToSlash(filepath.Clean(action.Path))
		result.Data.Type = "directory"
	}
	return result
}

func fitReadOnlyResult(response Response, result ActionResult, maxTransferChars int) (ActionResult, bool) {
	if result.Data == nil || resultFitsTransferLimit(response, result, maxTransferChars) {
		return result, false
	}

	data := *result.Data
	result.Data = &data
	data.Truncated = true

	switch result.Operation {
	case "search":
		fitSearchResult(response, result, maxTransferChars)
	case "ranked_search":
		fitRankedSearchResult(response, result, maxTransferChars)
	case "tree":
		fitTreeResult(response, result, maxTransferChars)
	case "read":
		fitReadResult(response, result, maxTransferChars)
	case "read_range":
		fitReadRangeResult(response, result, maxTransferChars)
	default:
		return result, false
	}
	return result, true
}

func fitSearchResult(response Response, result ActionResult, maxTransferChars int) {
	data := result.Data
	matches := data.Matches
	count := maximumFittingCount(len(matches), func(count int) bool {
		data.Matches = matches[:count]
		return resultFitsTransferLimit(response, result, maxTransferChars)
	})
	data.Matches = append([]SearchMatch(nil), matches[:count]...)
	if count == len(matches) {
		return
	}

	match := matches[count]
	data.Matches = append(data.Matches, match)
	matchIndex := len(data.Matches) - 1
	data.Matches[matchIndex].Text = trimTextToFit(match.Text, func(content string) bool {
		data.Matches[matchIndex].Text = content
		return resultFitsTransferLimit(response, result, maxTransferChars)
	})
	if !resultFitsTransferLimit(response, result, maxTransferChars) {
		data.Matches = data.Matches[:matchIndex]
	}
}

func fitRankedSearchResult(response Response, result ActionResult, maxTransferChars int) {
	data := result.Data
	matches := data.RankedMatches
	count := maximumFittingCount(len(matches), func(count int) bool {
		data.RankedMatches = matches[:count]
		return resultFitsTransferLimit(response, result, maxTransferChars)
	})
	data.RankedMatches = append([]RankedSearchMatch(nil), matches[:count]...)
	if count == len(matches) {
		return
	}

	match := matches[count]
	data.RankedMatches = append(data.RankedMatches, match)
	matchIndex := len(data.RankedMatches) - 1
	data.RankedMatches[matchIndex].Text = trimTextToFit(match.Text, func(content string) bool {
		data.RankedMatches[matchIndex].Text = content
		return resultFitsTransferLimit(response, result, maxTransferChars)
	})
	if !resultFitsTransferLimit(response, result, maxTransferChars) {
		data.RankedMatches = data.RankedMatches[:matchIndex]
	}
}

func fitTreeResult(response Response, result ActionResult, maxTransferChars int) {
	data := result.Data
	entries := data.Entries
	count := maximumFittingCount(len(entries), func(count int) bool {
		data.Entries = entries[:count]
		return resultFitsTransferLimit(response, result, maxTransferChars)
	})
	data.Entries = entries[:count]
}

func fitReadResult(response Response, result ActionResult, maxTransferChars int) {
	data := result.Data
	files := data.Files
	count := maximumFittingCount(len(files), func(count int) bool {
		data.Files = files[:count]
		return resultFitsTransferLimit(response, result, maxTransferChars)
	})
	data.Files = append([]ReadFileResult(nil), files[:count]...)
	if count == len(files) {
		return
	}

	file := files[count]
	data.Files = append(data.Files, file)
	fileIndex := len(data.Files) - 1
	data.Files[fileIndex].Content = trimTextToFit(file.Content, func(content string) bool {
		data.Files[fileIndex].Content = content
		return resultFitsTransferLimit(response, result, maxTransferChars)
	})
	if !resultFitsTransferLimit(response, result, maxTransferChars) {
		data.Files = data.Files[:fileIndex]
	}
}

func fitReadRangeResult(response Response, result ActionResult, maxTransferChars int) {
	data := result.Data
	lines := splitFileLines(data.Content)
	lineCount := maximumFittingCount(len(lines), func(count int) bool {
		data.Content = strings.Join(lines[:count], "")
		return resultFitsTransferLimit(response, result, maxTransferChars)
	})
	data.Content = strings.Join(lines[:lineCount], "")
	if lineCount == 0 {
		data.EndLine = 0
		return
	}
	data.EndLine = data.StartLine + lineCount - 1
}

func maximumFittingCount(length int, fits func(int) bool) int {
	low := 0
	high := length
	for low < high {
		middle := low + (high-low+1)/2
		if fits(middle) {
			low = middle
		} else {
			high = middle - 1
		}
	}
	return low
}

func trimTextToFit(content string, fits func(string) bool) string {
	runes := []rune(content)
	low := 0
	high := len(runes)
	for low < high {
		middle := low + (high-low+1)/2
		if fits(string(runes[:middle])) {
			low = middle
		} else {
			high = middle - 1
		}
	}
	return string(runes[:low])
}

func resultFitsTransferLimit(response Response, result ActionResult, maxTransferChars int) bool {
	candidate := response
	candidate.Results = append(append([]ActionResult(nil), response.Results...), result)
	return len(marshalResponse(candidate)) <= maxTransferChars
}

func createTransferLimitErrorResponse(maxTransferChars int) string {
	response := Response{
		Version: protocolVersion,
		Status:  "limit",
		Results: []ActionResult{},
		Error: &ResponseError{
			Code:    "TRANSFER_LIMIT_EXCEEDED",
			Message: fmt.Sprintf("Response exceeded the configured transfer limit (%d characters)", maxTransferChars),
		},
	}
	return marshalResponse(response)
}
