package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const protocolVersion = "1"

type Request struct {
	Version string   `json:"version"`
	Actions []Action `json:"actions"`
}

type Action struct {
	ID           string        `json:"id"`
	Operation    string        `json:"operation"`
	Query        string        `json:"query,omitempty"`
	Paths        []string      `json:"paths,omitempty"`
	Path         string        `json:"path,omitempty"`
	Replacements []Replacement `json:"replacements,omitempty"`
}

type Replacement struct {
	OldText string `json:"oldText"`
	NewText string `json:"newText"`
}

type Response struct {
	Version string         `json:"version"`
	Status  string         `json:"status"`
	Results []ActionResult `json:"results"`
	Error   *ResponseError `json:"error,omitempty"`
}

type ActionResult struct {
	ID        string      `json:"id"`
	Operation string      `json:"operation"`
	Status    string      `json:"status"`
	Data      *ActionData `json:"data,omitempty"`
}

type ActionData struct {
	Files               []ReadFileResult `json:"files,omitempty"`
	Query               string           `json:"query,omitempty"`
	Matches             []SearchMatch    `json:"matches,omitempty"`
	Truncated           bool             `json:"truncated,omitempty"`
	Path                string           `json:"path,omitempty"`
	ReplacementsApplied int              `json:"replacementsApplied,omitempty"`
}

type SearchMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type ReadFileResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type ResponseError struct {
	ActionID    string `json:"actionId,omitempty"`
	ActionIndex int    `json:"actionIndex"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Path        string `json:"path,omitempty"`
}

type lineScanner interface {
	Scan() bool
	Text() string
	Err() error
}

func collectJSONRequest(firstLine string, scanner lineScanner) (string, bool, error) {
	var input strings.Builder
	input.WriteString(firstLine)

	for {
		fmt.Print("... ")
		if !scanner.Scan() {
			if scanner.Err() != nil {
				return "", false, fmt.Errorf("read request: %w", scanner.Err())
			}
			return "", false, errors.New("JSON request ended before an empty terminating line")
		}

		line := scanner.Text()
		if strings.TrimSpace(line) == "/cancel" {
			return "", true, nil
		}
		if line == "" {
			return input.String(), false, nil
		}

		input.WriteString("\n")
		input.WriteString(line)
	}
}

func parseAndValidateRequest(requestText string) (Request, error) {
	request, err := decodeRequest(requestText)
	if err != nil {
		return Request{}, err
	}

	if err := validateRequest(request); err != nil {
		return Request{}, err
	}

	return request, nil
}

func decodeRequest(requestText string) (Request, error) {
	var request Request
	decoder := json.NewDecoder(strings.NewReader(requestText))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		return Request{}, fmt.Errorf("decode JSON request: %w", err)
	}

	if err := ensureJSONEnd(decoder); err != nil {
		return Request{}, err
	}

	return request, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("JSON request must contain exactly one object")
	}
	return fmt.Errorf("decode trailing JSON data: %w", err)
}

func validateRequest(request Request) error {
	if request.Version != protocolVersion {
		return fmt.Errorf("version must be %q", protocolVersion)
	}
	if len(request.Actions) == 0 {
		return errors.New("actions must contain at least one action")
	}

	actionIDs := make(map[string]struct{}, len(request.Actions))
	for index, action := range request.Actions {
		if err := validateAction(action, index, actionIDs); err != nil {
			return err
		}
	}

	return nil
}

func validateAction(action Action, index int, actionIDs map[string]struct{}) error {
	if strings.TrimSpace(action.ID) == "" {
		return fmt.Errorf("actions[%d].id is required", index)
	}
	if _, exists := actionIDs[action.ID]; exists {
		return fmt.Errorf("actions[%d].id %q is duplicated", index, action.ID)
	}
	actionIDs[action.ID] = struct{}{}

	switch action.Operation {
	case "search":
		if strings.TrimSpace(action.Query) == "" {
			return fmt.Errorf("actions[%d].query is required for search", index)
		}
	case "read":
		if len(action.Paths) == 0 {
			return fmt.Errorf("actions[%d].paths must contain at least one path for read", index)
		}
		for pathIndex, path := range action.Paths {
			if strings.TrimSpace(path) == "" {
				return fmt.Errorf("actions[%d].paths[%d] must not be empty", index, pathIndex)
			}
		}
	case "edit":
		if strings.TrimSpace(action.Path) == "" {
			return fmt.Errorf("actions[%d].path is required for edit", index)
		}
		if len(action.Replacements) == 0 {
			return fmt.Errorf("actions[%d].replacements must contain at least one replacement for edit", index)
		}
		for replacementIndex, replacement := range action.Replacements {
			if replacement.OldText == "" {
				return fmt.Errorf("actions[%d].replacements[%d].oldText must not be empty", index, replacementIndex)
			}
		}
	default:
		return fmt.Errorf("actions[%d].operation %q is not supported", index, action.Operation)
	}

	return nil
}

func executeRequest(workspace string, request Request) string {
	response := Response{
		Version: protocolVersion,
		Status:  "success",
		Results: make([]ActionResult, 0, len(request.Actions)),
	}

	for actionIndex, action := range request.Actions {
		result, responseError := executeAction(workspace, action, actionIndex)
		response.Results = append(response.Results, result)
		if responseError != nil {
			response.Status = "error"
			response.Error = responseError
			break
		}
	}

	return marshalResponse(response)
}

func executeAction(workspace string, action Action, actionIndex int) (ActionResult, *ResponseError) {
	result := ActionResult{
		ID:        action.ID,
		Operation: action.Operation,
		Status:    "success",
	}

	switch action.Operation {
	case "search":
		matches, truncated, responseError := executeSearchAction(workspace, action, actionIndex)
		result.Data = &ActionData{
			Query:     action.Query,
			Matches:   matches,
			Truncated: truncated,
		}
		if responseError != nil {
			result.Status = "error"
			return result, responseError
		}
	case "read":
		files, responseError := executeReadAction(workspace, action, actionIndex)
		result.Data = &ActionData{Files: files}
		if responseError != nil {
			result.Status = "error"
			return result, responseError
		}
	case "edit":
		replacementsApplied, responseError := executeEditAction(workspace, action, actionIndex)
		result.Data = &ActionData{
			Path:                filepath.ToSlash(filepath.Clean(action.Path)),
			ReplacementsApplied: replacementsApplied,
		}
		if responseError != nil {
			result.Status = "error"
			return result, responseError
		}
	}

	return result, nil
}

func createErrorResponse(code string, message string) string {
	response := Response{
		Version: protocolVersion,
		Status:  "error",
		Results: []ActionResult{},
		Error: &ResponseError{
			Code:    code,
			Message: message,
		},
	}

	return marshalResponse(response)
}

func marshalResponse(response Response) string {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Sprintf("{\"version\":%q,\"status\":\"error\",\"results\":[],\"error\":{\"code\":\"INTERNAL_ERROR\",\"message\":%q}}", protocolVersion, "Could not encode response")
	}

	return strings.TrimSuffix(output.String(), "\n")
}
