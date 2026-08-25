package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-tools-runner/internal/runner"
)

func summarizeResponse(responseText string) (string, error) {
	var response runner.Response
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		return "", fmt.Errorf("decode response summary: %w", err)
	}

	var summary strings.Builder
	summary.WriteString("\n")
	summary.WriteString(strings.ToUpper(response.Status))
	if len(response.Results) > 0 || response.Error != nil {
		summary.WriteString("\n")
	}
	for actionIndex, result := range response.Results {
		summary.WriteString("\n")
		summary.WriteString(fmt.Sprintf("  %-9s %s", responseOperationLabel(result.Operation), summarizeActionResult(result)))
		if result.Status == "error" {
			summary.WriteString(" [ERROR]")
		}
		if response.Error != nil && response.Error.ActionIndex == actionIndex {
			summary.WriteString("\n")
			summary.WriteString(fmt.Sprintf("            %s: %s", response.Error.Code, response.Error.Message))
			if response.Error.Path != "" && (result.Data == nil || result.Data.Path == "") {
				summary.WriteString(fmt.Sprintf(" (%s)", response.Error.Path))
			}
		}
	}
	if len(response.Results) == 0 && response.Error != nil {
		summary.WriteString("\n")
		summary.WriteString(fmt.Sprintf("  %s: %s", response.Error.Code, response.Error.Message))
		if response.Error.Path != "" {
			summary.WriteString(fmt.Sprintf(" (%s)", response.Error.Path))
		}
	}
	return summary.String(), nil
}

func responseOperationLabel(operation string) string {
	if operation == "read_range" {
		return "READ"
	}
	if operation == "ranked_search" {
		return "R_SEARCH"
	}
	return strings.ToUpper(operation)
}

func summarizeActionResult(result runner.ActionResult) string {
	if result.Data == nil {
		return result.ID
	}

	data := result.Data
	switch result.Operation {
	case "search":
		detail := fmt.Sprintf("%q (%d %s)", data.Query, len(data.Matches), pluralize(len(data.Matches), "match", "matches"))
		return appendTruncated(detail, data.Truncated)
	case "ranked_search":
		detail := fmt.Sprintf("%q (%d %s)", data.Query, len(data.RankedMatches), pluralize(len(data.RankedMatches), "match", "matches"))
		return appendTruncated(detail, data.Truncated)
	case "read":
		if len(data.Files) == 1 {
			return data.Files[0].Path
		}
		return fmt.Sprintf("%d files", len(data.Files))
	case "read_range":
		return fmt.Sprintf("%s (lines %d-%d of %d)", data.Path, data.StartLine, data.EndLine, data.TotalLines)
	case "edit":
		if result.Status == "error" {
			return data.Path
		}
		return fmt.Sprintf("%s (%d %s)", data.Path, data.ReplacementsApplied, pluralize(data.ReplacementsApplied, "replacement", "replacements"))
	case "create", "copy", "move":
		return fmt.Sprintf("%s (%d bytes)", data.Path, data.BytesWritten)
	case "tree":
		detail := fmt.Sprintf("%s (%d %s)", data.Path, len(data.Entries), pluralize(len(data.Entries), "entry", "entries"))
		return appendTruncated(detail, data.Truncated)
	case "inspect":
		return summarizeInspectResult(data)
	case "mkdir":
		return data.Path
	case "delete":
		if data.Type == "" {
			return data.Path
		}
		return fmt.Sprintf("%s (%s)", data.Path, data.Type)
	default:
		if data.Path != "" {
			return data.Path
		}
		return result.ID
	}
}

func summarizeInspectResult(data *runner.ActionData) string {
	if data.Type == "file" {
		return fmt.Sprintf("%s (file, %d bytes, %d %s)", data.Path, data.SizeBytes, data.LineCount, pluralize(data.LineCount, "line", "lines"))
	}
	if data.Type == "directory" && data.Empty != nil {
		state := "not empty"
		if *data.Empty {
			state = "empty"
		}
		return fmt.Sprintf("%s (directory, %s)", data.Path, state)
	}
	if data.Type != "" {
		return fmt.Sprintf("%s (%s)", data.Path, data.Type)
	}
	return data.Path
}

func appendTruncated(detail string, truncated bool) string {
	if truncated {
		return detail + ", truncated"
	}
	return detail
}

func pluralize(count int, singular string, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
