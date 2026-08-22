package runner

import (
	"bufio"
	"bytes"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maximumRankedSearchMatches = 20
const minimumRankedSearchCharacters = 2
const minimumFuzzySearchCharacters = 3
const minimumFuzzyIdentifierScore = 0.75
const minimumFuzzyLineScore = 0.85

func executeRankedSearchAction(workspace string, action Action, actionIndex int) ([]RankedSearchMatch, bool, *ResponseError) {
	matches, truncated, err := rankedSearchWorkspace(workspace, action.Query)
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

func rankedSearchWorkspace(workspace string, query string) ([]RankedSearchMatch, bool, error) {
	matches := make([]RankedSearchMatch, 0)
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
		if entry.IsDir() {
			if isSearchExcludedDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}

		fileMatches, err := rankedSearchFile(workspace, path, query)
		if err != nil {
			return nil
		}
		matches = append(matches, fileMatches...)
		return nil
	})
	if err != nil {
		return matches, false, err
	}

	sort.Slice(matches, func(first int, second int) bool {
		if matches[first].Score != matches[second].Score {
			return matches[first].Score > matches[second].Score
		}
		firstPriority := rankedMatchTypePriority(matches[first].MatchType)
		secondPriority := rankedMatchTypePriority(matches[second].MatchType)
		if firstPriority != secondPriority {
			return firstPriority < secondPriority
		}
		if matches[first].Path != matches[second].Path {
			return matches[first].Path < matches[second].Path
		}
		return matches[first].Line < matches[second].Line
	})

	truncated := len(matches) > maximumRankedSearchMatches
	if truncated {
		matches = matches[:maximumRankedSearchMatches]
	}
	return matches, truncated, nil
}

func rankedSearchFile(workspace string, path string, query string) ([]RankedSearchMatch, error) {
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

	matches := make([]RankedSearchMatch, 0)
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, scannerInitialBufferSize), int(maximumFileSize))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		matchType, score, matched := rankSearchLine(query, line)
		if matched {
			matches = append(matches, RankedSearchMatch{
				Path:      filepath.ToSlash(relativePath),
				Line:      lineNumber,
				Text:      line,
				MatchType: matchType,
				Score:     score,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return matches, nil
}

func rankSearchLine(query string, line string) (string, float64, bool) {
	trimmedQuery := strings.TrimSpace(query)
	trimmedLine := strings.TrimSpace(line)
	if strings.Contains(line, trimmedQuery) {
		return "exact", 1, true
	}
	if strings.Contains(strings.ToLower(line), strings.ToLower(trimmedQuery)) {
		return "case_insensitive", 0.99, true
	}

	queryIdentifier := strings.Join(tokenizeIdentifier(trimmedQuery), "")
	bestIdentifierScore := 0.0
	for _, candidate := range identifierCandidates(trimmedLine) {
		candidateIdentifier := strings.Join(tokenizeIdentifier(candidate), "")
		if candidateIdentifier == "" {
			continue
		}
		if candidateIdentifier == queryIdentifier {
			return "identifier", 0.98, true
		}
		if countLettersAndDigits(trimmedQuery) >= minimumFuzzySearchCharacters {
			score := similarityScore(queryIdentifier, candidateIdentifier)
			if score > bestIdentifierScore {
				bestIdentifierScore = score
			}
		}
	}
	if bestIdentifierScore >= minimumFuzzyIdentifierScore {
		return "fuzzy_identifier", roundSearchScore(bestIdentifierScore * 0.97), true
	}

	if countLettersAndDigits(trimmedQuery) >= minimumFuzzySearchCharacters {
		lineScore := similarityScore(strings.ToLower(trimmedQuery), strings.ToLower(trimmedLine))
		if lineScore >= minimumFuzzyLineScore {
			return "fuzzy_line", roundSearchScore(lineScore * 0.9), true
		}
	}
	return "", 0, false
}

func tokenizeIdentifier(value string) []string {
	runes := []rune(value)
	tokens := make([]string, 0)
	start := -1
	for index, current := range runes {
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			if start >= 0 {
				tokens = append(tokens, strings.ToLower(string(runes[start:index])))
				start = -1
			}
			continue
		}
		if start < 0 {
			start = index
			continue
		}
		previous := runes[index-1]
		if unicode.IsUpper(current) && unicode.IsLower(previous) {
			tokens = append(tokens, strings.ToLower(string(runes[start:index])))
			start = index
		}
	}
	if start >= 0 {
		tokens = append(tokens, strings.ToLower(string(runes[start:])))
	}
	return tokens
}

func identifierCandidates(line string) []string {
	return strings.FieldsFunc(line, func(value rune) bool {
		return !unicode.IsLetter(value) && !unicode.IsDigit(value) && value != '_' && value != '-'
	})
}

func countLettersAndDigits(value string) int {
	count := 0
	for _, current := range value {
		if unicode.IsLetter(current) || unicode.IsDigit(current) {
			count++
		}
	}
	return count
}

func similarityScore(first string, second string) float64 {
	firstRunes := []rune(first)
	secondRunes := []rune(second)
	maximumLength := len(firstRunes)
	if len(secondRunes) > maximumLength {
		maximumLength = len(secondRunes)
	}
	if maximumLength == 0 {
		return 1
	}
	lengthDiff := len(firstRunes) - len(secondRunes)
	if lengthDiff < 0 {
		lengthDiff = -lengthDiff
	}
	if float64(lengthDiff)/float64(maximumLength) > 0.25 {
		return 0
	}
	distance := levenshteinDistance(firstRunes, secondRunes)
	return 1 - float64(distance)/float64(maximumLength)
}

func levenshteinDistance(first []rune, second []rune) int {
	previous := make([]int, len(second)+1)
	current := make([]int, len(second)+1)
	for index := range previous {
		previous[index] = index
	}
	for firstIndex, firstRune := range first {
		current[0] = firstIndex + 1
		for secondIndex, secondRune := range second {
			cost := 0
			if firstRune != secondRune {
				cost = 1
			}
			deletion := previous[secondIndex+1] + 1
			insertion := current[secondIndex] + 1
			substitution := previous[secondIndex] + cost
			current[secondIndex+1] = min(deletion, insertion, substitution)
		}
		previous, current = current, previous
	}
	return previous[len(second)]
}

func roundSearchScore(score float64) float64 {
	return math.Round(score*1000) / 1000
}

func rankedMatchTypePriority(matchType string) int {
	switch matchType {
	case "exact":
		return 0
	case "case_insensitive":
		return 1
	case "identifier":
		return 2
	case "fuzzy_identifier":
		return 3
	case "fuzzy_line":
		return 4
	default:
		return 5
	}
}
