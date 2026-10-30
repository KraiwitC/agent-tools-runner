package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const AgentsDirectoryName = ".agents"
const ConfigFileName = "atr.json"
const ConfigDisplayPath = AgentsDirectoryName + "/" + ConfigFileName

type Config struct {
	Bash BashConfig `json:"bash"`
}

type BashConfig struct {
	Allow []string `json:"allow"`
}

func LoadConfig(workspace string) (Config, bool, error) {
	agentsPath := filepath.Join(workspace, AgentsDirectoryName)
	agentsInfo, err := os.Lstat(agentsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, false, nil
		}
		return Config{}, false, fmt.Errorf("inspect %s: %w", AgentsDirectoryName, err)
	}
	if agentsInfo.Mode()&os.ModeSymlink != 0 {
		return Config{}, true, fmt.Errorf("%s must not be a symbolic link", AgentsDirectoryName)
	}

	configPath := filepath.Join(agentsPath, ConfigFileName)
	configInfo, err := os.Lstat(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, false, nil
		}
		return Config{}, false, fmt.Errorf("inspect %s: %w", ConfigDisplayPath, err)
	}
	if configInfo.Mode()&os.ModeSymlink != 0 {
		return Config{}, true, fmt.Errorf("%s must not be a symbolic link", ConfigDisplayPath)
	}
	if !configInfo.Mode().IsRegular() {
		return Config{}, true, fmt.Errorf("%s must be a regular file", ConfigDisplayPath)
	}
	if configInfo.Size() > maximumFileSize {
		return Config{}, true, fmt.Errorf("%s exceeds the 10 MiB limit", ConfigDisplayPath)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, true, fmt.Errorf("read %s: %w", ConfigDisplayPath, err)
	}

	var config Config
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return Config{}, true, fmt.Errorf("decode %s: %w", ConfigDisplayPath, err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return Config{}, true, fmt.Errorf("decode %s: %w", ConfigDisplayPath, err)
	}

	for index, entry := range config.Bash.Allow {
		if len(strings.Fields(entry)) == 0 {
			return Config{}, true, fmt.Errorf("%s bash.allow[%d] must not be blank", ConfigDisplayPath, index)
		}
	}
	return config, true, nil
}

func protectedPathError(workspace string, action Action, actionIndex int) *ResponseError {
	if !isModifyingOperation(action.Operation) {
		return nil
	}
	for _, requestedPath := range []string{action.Path, action.Source, action.Destination} {
		if requestedPath != "" && isProtectedPath(workspace, requestedPath) {
			return newActionResponseError(action, actionIndex, "PROTECTED_PATH", "The .agents folder is read-only to ATR operations.", filepath.ToSlash(filepath.Clean(requestedPath)))
		}
	}
	return nil
}

func isProtectedPath(workspace string, requestedPath string) bool {
	cleanRequestedPath := filepath.Clean(requestedPath)
	if filepath.IsAbs(cleanRequestedPath) || filepath.VolumeName(cleanRequestedPath) != "" {
		return false
	}
	containedPath, err := filepath.Rel(workspace, filepath.Join(workspace, cleanRequestedPath))
	if err != nil {
		return false
	}
	firstSegment := strings.Split(filepath.ToSlash(containedPath), "/")[0]

	normalizedSegment, _, _ := strings.Cut(firstSegment, ":")
	normalizedSegment = strings.TrimRight(normalizedSegment, ". ")
	if strings.EqualFold(normalizedSegment, AgentsDirectoryName) {
		return true
	}

	segmentInfo, err := os.Lstat(filepath.Join(workspace, firstSegment))
	if err != nil {
		return false
	}
	agentsInfo, err := os.Lstat(filepath.Join(workspace, AgentsDirectoryName))
	return err == nil && os.SameFile(segmentInfo, agentsInfo)
}
