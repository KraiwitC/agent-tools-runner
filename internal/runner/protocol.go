package runner

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const protocolVersion = "1"
const maximumActions = 100
const maximumEditReplacements = 100
const maximumReadRangeLines = 1000
const defaultMaximumTransferChars = 100000
const maximumTransferChars = 120000
const minimumTransferChars = 1000
const sha256HexLength = 64

type Request struct {
	Version          string   `json:"version"`
	MaxTransferChars int      `json:"maxTransferChars,omitempty"`
	Actions          []Action `json:"actions"`
}

type Action struct {
	ID             string        `json:"id"`
	Operation      string        `json:"operation"`
	Query          string        `json:"query,omitempty"`
	Paths          []string      `json:"paths,omitempty"`
	Path           string        `json:"path,omitempty"`
	Source         string        `json:"source,omitempty"`
	Destination    string        `json:"destination,omitempty"`
	Content        string        `json:"content,omitempty"`
	StartLine      int           `json:"startLine,omitempty"`
	EndLine        int           `json:"endLine,omitempty"`
	ExpectedSHA256 string        `json:"expectedSha256,omitempty"`
	Replacements   []Replacement `json:"replacements,omitempty"`
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
	Entries             []TreeEntry      `json:"entries,omitempty"`
	Truncated           bool             `json:"truncated,omitempty"`
	Path                string           `json:"path,omitempty"`
	Type                string           `json:"type,omitempty"`
	Content             string           `json:"content,omitempty"`
	StartLine           int              `json:"startLine,omitempty"`
	EndLine             int              `json:"endLine,omitempty"`
	TotalLines          int              `json:"totalLines,omitempty"`
	SizeBytes           int64            `json:"sizeBytes,omitempty"`
	LineCount           int              `json:"lineCount,omitempty"`
	SHA256              string           `json:"sha256,omitempty"`
	Empty               *bool            `json:"empty,omitempty"`
	ReplacementsApplied int              `json:"replacementsApplied,omitempty"`
	BytesWritten        int              `json:"bytesWritten,omitempty"`
}

type TreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type SearchMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type ReadFileResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}

type ResponseError struct {
	ActionID    string `json:"actionId,omitempty"`
	ActionIndex int    `json:"actionIndex"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Path        string `json:"path,omitempty"`
}

func normalizeJSONRequest(requestText string) (string, error) {
	normalized := strings.TrimSpace(requestText)
	normalized = removeOpeningJSONFence(normalized)
	normalized = removeTrailingFenceArtifact(normalized)
	normalized = strings.TrimSpace(normalized)

	if !strings.HasPrefix(normalized, "{") {
		return "", errors.New("JSON request must begin with an object")
	}
	if !strings.HasSuffix(normalized, "}") {
		return "", errors.New("JSON request must end with an object")
	}
	return normalized, nil
}

func removeOpeningJSONFence(input string) string {
	lineEnd := strings.IndexByte(input, '\n')
	if lineEnd < 0 {
		return input
	}
	firstLine := strings.TrimSpace(strings.TrimSuffix(input[:lineEnd], "\r"))
	lowerFirstLine := strings.ToLower(firstLine)
	if lowerFirstLine == "```" || lowerFirstLine == "```json" || lowerFirstLine == "~~~" || lowerFirstLine == "~~~json" {
		return strings.TrimSpace(input[lineEnd+1:])
	}
	return input
}

func removeTrailingFenceArtifact(input string) string {
	trimmedInput := strings.TrimSpace(input)
	if len(trimmedInput) == 0 {
		return trimmedInput
	}
	fenceCharacter := trimmedInput[len(trimmedInput)-1]
	if fenceCharacter != '`' && fenceCharacter != '~' {
		return trimmedInput
	}
	fenceLength := 0
	for index := len(trimmedInput) - 1; index >= 0 && trimmedInput[index] == fenceCharacter && fenceLength < 3; index-- {
		fenceLength++
	}
	withoutFence := strings.TrimSpace(trimmedInput[:len(trimmedInput)-fenceLength])
	if strings.HasSuffix(withoutFence, "}") {
		return withoutFence
	}
	return trimmedInput
}

func ParseAndValidateRequest(requestText string) (Request, error) {
	normalizedRequest, err := normalizeJSONRequest(requestText)
	if err != nil {
		return Request{}, err
	}
	request, err := decodeRequest(normalizedRequest)
	if err != nil {
		return Request{}, err
	}
	if err := validateRequest(request); err != nil {
		return Request{}, err
	}
	request.MaxTransferChars = effectiveMaximumTransferChars(request.MaxTransferChars)
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
	if request.MaxTransferChars != 0 && (request.MaxTransferChars < minimumTransferChars || request.MaxTransferChars > maximumTransferChars) {
		return fmt.Errorf("maxTransferChars must be between %d and %d", minimumTransferChars, maximumTransferChars)
	}
	if len(request.Actions) == 0 {
		return errors.New("actions must contain at least one action")
	}
	if len(request.Actions) > maximumActions {
		return fmt.Errorf("actions must not contain more than %d actions", maximumActions)
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

	if action.Operation != "read_range" && (action.StartLine != 0 || action.EndLine != 0) {
		return fmt.Errorf("actions[%d] contains range fields that are only supported for read_range", index)
	}
	if action.Operation != "copy" && (action.Source != "" || action.Destination != "") {
		return fmt.Errorf("actions[%d] contains source or destination fields that are only supported for copy", index)
	}

	switch action.Operation {
	case "search":
		if strings.TrimSpace(action.Query) == "" {
			return fmt.Errorf("actions[%d].query is required for search", index)
		}
		if len(action.Paths) != 0 || action.Path != "" || action.Content != "" || action.ExpectedSHA256 != "" || len(action.Replacements) != 0 {
			return fmt.Errorf("actions[%d] contains fields that are not supported for search", index)
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
		if action.Query != "" || action.Path != "" || action.Content != "" || action.ExpectedSHA256 != "" || len(action.Replacements) != 0 {
			return fmt.Errorf("actions[%d] contains fields that are not supported for read", index)
		}
	case "read_range":
		if strings.TrimSpace(action.Path) == "" {
			return fmt.Errorf("actions[%d].path is required for read_range", index)
		}
		if action.StartLine < 1 {
			return fmt.Errorf("actions[%d].startLine must be at least 1 for read_range", index)
		}
		if action.EndLine < action.StartLine {
			return fmt.Errorf("actions[%d].endLine must be greater than or equal to startLine for read_range", index)
		}
		if action.EndLine-action.StartLine+1 > maximumReadRangeLines {
			return fmt.Errorf("actions[%d] read_range must not request more than %d lines", index, maximumReadRangeLines)
		}
		if action.Query != "" || len(action.Paths) != 0 || action.Content != "" || action.ExpectedSHA256 != "" || len(action.Replacements) != 0 {
			return fmt.Errorf("actions[%d] contains fields that are not supported for read_range", index)
		}
	case "edit":
		if strings.TrimSpace(action.Path) == "" {
			return fmt.Errorf("actions[%d].path is required for edit", index)
		}
		if !isValidSHA256(action.ExpectedSHA256) {
			return fmt.Errorf("actions[%d].expectedSha256 must be a lowercase SHA-256 hash for edit", index)
		}
		if len(action.Replacements) == 0 {
			return fmt.Errorf("actions[%d].replacements must contain at least one replacement for edit", index)
		}
		if len(action.Replacements) > maximumEditReplacements {
			return fmt.Errorf("actions[%d].replacements must not contain more than %d replacements", index, maximumEditReplacements)
		}
		for replacementIndex, replacement := range action.Replacements {
			if replacement.OldText == "" {
				return fmt.Errorf("actions[%d].replacements[%d].oldText must not be empty", index, replacementIndex)
			}
		}
		if action.Query != "" || len(action.Paths) != 0 || action.Content != "" {
			return fmt.Errorf("actions[%d] contains fields that are not supported for edit", index)
		}
	case "create":
		if strings.TrimSpace(action.Path) == "" {
			return fmt.Errorf("actions[%d].path is required for create", index)
		}
		if action.Content == "" {
			return fmt.Errorf("actions[%d].content must not be empty for create", index)
		}
		if action.Query != "" || len(action.Paths) != 0 || action.Source != "" || action.Destination != "" || action.ExpectedSHA256 != "" || len(action.Replacements) != 0 {
			return fmt.Errorf("actions[%d] contains fields that are not supported for create", index)
		}
	case "copy":
		if strings.TrimSpace(action.Source) == "" {
			return fmt.Errorf("actions[%d].source is required for copy", index)
		}
		if strings.TrimSpace(action.Destination) == "" {
			return fmt.Errorf("actions[%d].destination is required for copy", index)
		}
		if !isValidSHA256(action.ExpectedSHA256) {
			return fmt.Errorf("actions[%d].expectedSha256 must be a lowercase SHA-256 hash for copy", index)
		}
		if action.Query != "" || len(action.Paths) != 0 || action.Path != "" || action.Content != "" || len(action.Replacements) != 0 {
			return fmt.Errorf("actions[%d] contains fields that are not supported for copy", index)
		}
	case "tree":
		if strings.TrimSpace(action.Path) == "" {
			return fmt.Errorf("actions[%d].path is required for tree", index)
		}
		if action.Query != "" || len(action.Paths) != 0 || action.Content != "" || action.ExpectedSHA256 != "" || len(action.Replacements) != 0 {
			return fmt.Errorf("actions[%d] contains fields that are not supported for tree", index)
		}
	case "inspect":
		if strings.TrimSpace(action.Path) == "" {
			return fmt.Errorf("actions[%d].path is required for inspect", index)
		}
		if action.Query != "" || len(action.Paths) != 0 || action.Content != "" || action.ExpectedSHA256 != "" || len(action.Replacements) != 0 {
			return fmt.Errorf("actions[%d] contains fields that are not supported for inspect", index)
		}
	case "mkdir":
		if strings.TrimSpace(action.Path) == "" {
			return fmt.Errorf("actions[%d].path is required for mkdir", index)
		}
		if action.Query != "" || len(action.Paths) != 0 || action.Content != "" || action.ExpectedSHA256 != "" || len(action.Replacements) != 0 {
			return fmt.Errorf("actions[%d] contains fields that are not supported for mkdir", index)
		}
	case "delete":
		if strings.TrimSpace(action.Path) == "" {
			return fmt.Errorf("actions[%d].path is required for delete", index)
		}
		if action.ExpectedSHA256 != "" && !isValidSHA256(action.ExpectedSHA256) {
			return fmt.Errorf("actions[%d].expectedSha256 must be empty or a lowercase SHA-256 hash for delete", index)
		}
		if action.Query != "" || len(action.Paths) != 0 || action.Content != "" || len(action.Replacements) != 0 {
			return fmt.Errorf("actions[%d] contains fields that are not supported for delete", index)
		}
	default:
		return fmt.Errorf("actions[%d].operation %q is not supported", index, action.Operation)
	}
	return nil
}

func effectiveMaximumTransferChars(value int) int {
	if value == 0 {
		return defaultMaximumTransferChars
	}
	return value
}

func isValidSHA256(value string) bool {
	if len(value) != sha256HexLength || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func ExecuteRequest(workspace string, request Request) string {
	response := Response{
		Version: protocolVersion,
		Status:  "success",
		Results: make([]ActionResult, 0, len(request.Actions)),
	}
	maxTransferChars := effectiveMaximumTransferChars(request.MaxTransferChars)
	for actionIndex, action := range request.Actions {
		result, responseError := executeAction(workspace, action, actionIndex)
		response.Results = append(response.Results, result)
		if responseError != nil {
			response.Status = "error"
			response.Error = responseError
			break
		}
		if len(marshalResponse(response)) > maxTransferChars {
			response.Results[len(response.Results)-1] = ActionResult{
				ID:        action.ID,
				Operation: action.Operation,
				Status:    "error",
			}
			response.Status = "error"
			response.Error = &ResponseError{
				ActionID:    action.ID,
				ActionIndex: actionIndex,
				Code:        "TRANSFER_LIMIT_EXCEEDED",
				Message:     "The action result exceeds maxTransferChars. Request less content or use a smaller range.",
			}
			break
		}
	}
	responseText := marshalResponse(response)
	if len(responseText) <= maxTransferChars {
		return responseText
	}
	return createTransferLimitErrorResponse(maxTransferChars)
}

func createTransferLimitErrorResponse(maxTransferChars int) string {
	response := Response{
		Version: protocolVersion,
		Status:  "error",
		Results: []ActionResult{},
		Error: &ResponseError{
			Code:    "TRANSFER_LIMIT_EXCEEDED",
			Message: fmt.Sprintf("The response exceeds the %d character transfer limit.", maxTransferChars),
		},
	}
	return marshalResponse(response)
}

func executeAction(workspace string, action Action, actionIndex int) (ActionResult, *ResponseError) {
	result := ActionResult{
		ID:        action.ID,
		Operation: action.Operation,
		Status:    "success",
	}
	var responseError *ResponseError

	switch action.Operation {
	case "search":
		var matches []SearchMatch
		var truncated bool
		matches, truncated, responseError = executeSearchAction(workspace, action, actionIndex)
		result.Data = &ActionData{
			Query:     action.Query,
			Matches:   matches,
			Truncated: truncated,
		}
	case "read":
		var files []ReadFileResult
		files, responseError = executeReadAction(workspace, action, actionIndex)
		result.Data = &ActionData{Files: files}
	case "read_range":
		result.Data, responseError = executeReadRangeAction(workspace, action, actionIndex)
	case "edit":
		var replacementsApplied int
		var updatedSHA256 string
		replacementsApplied, updatedSHA256, responseError = executeEditAction(workspace, action, actionIndex)
		result.Data = &ActionData{
			Path:                filepath.ToSlash(filepath.Clean(action.Path)),
			SHA256:              updatedSHA256,
			ReplacementsApplied: replacementsApplied,
		}
	case "create":
		var bytesWritten int
		var relativePath string
		bytesWritten, relativePath, responseError = executeCreateAction(workspace, action, actionIndex)
		result.Data = &ActionData{
			Path:         relativePath,
			BytesWritten: bytesWritten,
		}
		if responseError == nil {
			result.Data.SHA256 = calculateSHA256([]byte(action.Content))
		}
	case "copy":
		var bytesWritten int
		var relativePath string
		var copiedSHA256 string
		bytesWritten, relativePath, copiedSHA256, responseError = executeCopyAction(workspace, action, actionIndex)
		result.Data = &ActionData{
			Path:         relativePath,
			SHA256:       copiedSHA256,
			BytesWritten: bytesWritten,
		}
	case "tree":
		var entries []TreeEntry
		var relativePath string
		var truncated bool
		entries, relativePath, truncated, responseError = executeTreeAction(workspace, action, actionIndex)
		result.Data = &ActionData{
			Path:      relativePath,
			Entries:   entries,
			Truncated: truncated,
		}
	case "inspect":
		result.Data, responseError = executeInspectAction(workspace, action, actionIndex)
	case "mkdir":
		var relativePath string
		relativePath, responseError = executeMkdirAction(workspace, action, actionIndex)
		result.Data = &ActionData{
			Path: relativePath,
			Type: "directory",
		}
	case "delete":
		var relativePath string
		var pathType string
		relativePath, pathType, responseError = executeDeleteAction(workspace, action, actionIndex)
		result.Data = &ActionData{
			Path: relativePath,
			Type: pathType,
		}
	}
	if responseError != nil {
		result.Status = "error"
	}
	return result, responseError
}

func CreateErrorResponse(code string, message string) string {
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
