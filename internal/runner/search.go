package runner

import (
	"bufio"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
	var files []string
	pathMatches := make([]SearchMatch, 0)
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
		if strings.Contains(normalizedPath, query) && len(pathMatches) < maximumCollectedSearchMatches+1 {
			pathMatches = append(pathMatches, SearchMatch{
				Path:        normalizedPath,
				MatchTarget: "path",
			})
		}

		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	if len(files) == 0 {
		truncated := len(pathMatches) > maximumCollectedSearchMatches
		if truncated {
			pathMatches = pathMatches[:maximumCollectedSearchMatches]
		}
		return pathMatches, truncated, nil
	}

	workerCount := runtime.NumCPU()
	if workerCount > len(files) {
		workerCount = len(files)
	}

	collectionLimit := maximumCollectedSearchMatches + 1
	allMatches := make([]SearchMatch, 0, len(pathMatches)+collectionLimit)
	allMatches = append(allMatches, pathMatches...)
	for batchStart := 0; batchStart < len(files) && len(allMatches) < collectionLimit; batchStart += workerCount {
		batchEnd := min(batchStart+workerCount, len(files))
		batchMatches := make([][]SearchMatch, batchEnd-batchStart)
		batchErrors := make([]error, batchEnd-batchStart)
		var wg sync.WaitGroup

		for fileIndex := batchStart; fileIndex < batchEnd; fileIndex++ {
			batchIndex := fileIndex - batchStart
			wg.Add(1)
			go func(batchIndex int, fileIndex int) {
				defer wg.Done()
				batchMatches[batchIndex], batchErrors[batchIndex] = searchFile(workspace, files[fileIndex], query, collectionLimit)
			}(batchIndex, fileIndex)
		}
		wg.Wait()

		for batchIndex, fileMatches := range batchMatches {
			if batchErrors[batchIndex] != nil {
				return nil, false, batchErrors[batchIndex]
			}
			remainingMatches := collectionLimit - len(allMatches)
			if len(fileMatches) > remainingMatches {
				fileMatches = fileMatches[:remainingMatches]
			}
			allMatches = append(allMatches, fileMatches...)
			if len(allMatches) >= collectionLimit {
				break
			}
		}
	}

	sort.Slice(allMatches, func(i, j int) bool {
		if allMatches[i].Path != allMatches[j].Path {
			return allMatches[i].Path < allMatches[j].Path
		}
		if allMatches[i].Line != allMatches[j].Line {
			return allMatches[i].Line < allMatches[j].Line
		}
		return allMatches[i].MatchTarget < allMatches[j].MatchTarget
	})

	truncated := len(allMatches) > maximumCollectedSearchMatches
	if truncated {
		allMatches = allMatches[:maximumCollectedSearchMatches]
	}

	return allMatches, truncated, nil
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
	scanner.Buffer(make([]byte, scannerInitialBufferSize), int(maximumFileSize))
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
