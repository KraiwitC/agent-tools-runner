package runner

import (
	"bufio"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"
)

const maximumCollectedSearchMatches = 1000
const scannerInitialBufferSize = 64 * 1024

var defaultSearchExcludedDirectories = map[string]struct{}{
	".git":         {},
	".idea":        {},
	"node_modules": {},
	"target":       {},
	"build":        {},
	"dist":         {},
	"vendor":       {},
	".gradle":      {},
	".m2":          {},
	".mvn":         {},
	"out":          {},
	"bin":          {},
	".metadata":    {},
	".settings":    {},
	".venv":        {},
	"venv":         {},
	"__pycache__":  {},
	".next":        {},
	".turbo":       {},
	".cache":       {},
	"coverage":     {},
	".output":      {},
	".terraform":   {},
}

func executeSearchAction(workspace string, action Action, actionIndex int) ([]SearchMatch, bool, *ResponseError) {
	matches, truncated, err := searchWorkspace(workspace, action.Query)
	if err != nil {
		return matches, truncated, &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "SEARCH_FAILED",
			Message:     "Could not search the selected workspace.",
		}
	}

	return matches, truncated, nil
}

func searchWorkspace(workspace string, query string) ([]SearchMatch, bool, error) {
	type searchCandidate struct {
		pathMatch *SearchMatch
		filePath  string
	}

	collectionLimit := maximumCollectedSearchMatches + 1
	var candidates []searchCandidate
	err := filepath.WalkDir(workspace, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == workspace {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() && isSearchExcludedDirectory(entry.Name()) {
			return filepath.SkipDir
		}

		relativePath, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}
		normalizedPath := filepath.ToSlash(relativePath)
		candidate := searchCandidate{}
		if strings.Contains(normalizedPath, query) {
			candidate.pathMatch = &SearchMatch{
				Path:        normalizedPath,
				MatchTarget: "path",
			}
		}
		if !entry.IsDir() && entry.Type().IsRegular() {
			candidate.filePath = path
		}
		if candidate.pathMatch != nil || candidate.filePath != "" {
			candidates = append(candidates, candidate)
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	matches := make([]SearchMatch, 0, collectionLimit)
	if len(candidates) > 0 {
		workerCount := runtime.NumCPU()
		if workerCount > len(candidates) {
			workerCount = len(candidates)
		}

		for batchStart := 0; batchStart < len(candidates) && len(matches) < collectionLimit; batchStart += workerCount {
			batchEnd := min(batchStart+workerCount, len(candidates))
			batchMatches := make([][]SearchMatch, batchEnd-batchStart)
			batchErrors := make([]error, batchEnd-batchStart)
			var wg sync.WaitGroup

			for candidateIndex := batchStart; candidateIndex < batchEnd; candidateIndex++ {
				batchIndex := candidateIndex - batchStart
				if candidates[candidateIndex].filePath == "" {
					continue
				}
				wg.Add(1)
				go func(batchIndex int, candidateIndex int) {
					defer wg.Done()
					batchMatches[batchIndex], batchErrors[batchIndex] = searchFile(workspace, candidates[candidateIndex].filePath, query, collectionLimit)
				}(batchIndex, candidateIndex)
			}
			wg.Wait()

			for candidateIndex := batchStart; candidateIndex < batchEnd; candidateIndex++ {
				batchIndex := candidateIndex - batchStart
				if batchErrors[batchIndex] != nil {
					return nil, false, batchErrors[batchIndex]
				}
				if candidates[candidateIndex].pathMatch != nil {
					matches = append(matches, *candidates[candidateIndex].pathMatch)
				}
				remainingMatches := collectionLimit - len(matches)
				if remainingMatches <= 0 {
					break
				}
				fileMatches := batchMatches[batchIndex]
				if len(fileMatches) > remainingMatches {
					fileMatches = fileMatches[:remainingMatches]
				}
				matches = append(matches, fileMatches...)
				if len(matches) >= collectionLimit {
					break
				}
			}
		}
	}

	truncated := len(matches) > maximumCollectedSearchMatches
	if truncated {
		matches = matches[:maximumCollectedSearchMatches]
	}
	return matches, truncated, nil
}

func searchFile(workspace string, path string, query string, matchLimit int) ([]SearchMatch, error) {
	if matchLimit <= 0 {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maximumFileSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maximumFileSize || !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return nil, nil
	}

	relativePath, err := filepath.Rel(workspace, path)
	if err != nil {
		return nil, err
	}

	matches := make([]SearchMatch, 0)
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, scannerInitialBufferSize), int(maximumFileSize)+1)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if strings.Contains(line, query) {
			matches = append(matches, SearchMatch{
				Path:        filepath.ToSlash(relativePath),
				Line:        lineNumber,
				Text:        line,
				MatchTarget: "content",
			})
			if len(matches) >= matchLimit {
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return matches, nil
}

func isSearchExcludedDirectory(name string) bool {
	_, excluded := defaultSearchExcludedDirectories[name]
	return excluded
}
