package app

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTerminalLineReaderHandlesLineEndingsAndFinalLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{name: "LF", input: "line1\nline2\n", expected: []string{"line1", "line2"}},
		{name: "CRLF", input: "line1\r\nline2\r\n", expected: []string{"line1", "line2"}},
		{name: "standalone CR", input: "line1\rline2\r", expected: []string{"line1", "line2"}},
		{name: "CRLF blank line", input: "line1\r\n\r\nline2\r\n", expected: []string{"line1", "", "line2"}},
		{name: "unterminated final line", input: "line1\nline2", expected: []string{"line1", "line2"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := newTerminalLineReader(strings.NewReader(test.input))
			for index, expected := range test.expected {
				if !reader.Scan() {
					t.Fatalf("Scan() failed for line %d: %v", index+1, reader.Err())
				}
				if reader.Text() != expected {
					t.Fatalf("line %d = %q, want %q", index+1, reader.Text(), expected)
				}
			}
			if reader.Scan() {
				t.Fatalf("Scan() succeeded after expected input with text %q", reader.Text())
			}
			if reader.Err() != nil {
				t.Fatalf("unexpected terminal reader error: %v", reader.Err())
			}
		})
	}
}

func TestTerminalLineReaderReturnsLineOnCarriageReturnWithoutMoreInput(t *testing.T) {
	pipeReader, pipeWriter := io.Pipe()
	defer pipeWriter.Close()
	reader := newTerminalLineReader(pipeReader)

	go pipeWriter.Write([]byte("/help\r"))

	if !reader.Scan() {
		t.Fatalf("Scan() failed: %v", reader.Err())
	}
	if reader.Text() != "/help" {
		t.Fatalf("line = %q, want %q", reader.Text(), "/help")
	}
}
func TestTerminalLineReaderHandlesBackspace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "backspace at beginning", input: "\x7fvalue\n", expected: "value"},
		{name: "delete ASCII rune", input: "ab\x7fc\n", expected: "ac"},
		{name: "delete UTF-8 rune", input: "กข\x7fค\n", expected: "กค"},
		{name: "control-H", input: "ab\x08c\n", expected: "ac"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := newTerminalLineReader(strings.NewReader(test.input))
			if !reader.Scan() {
				t.Fatalf("Scan() failed: %v", reader.Err())
			}
			if reader.Text() != test.expected {
				t.Fatalf("line = %q, want %q", reader.Text(), test.expected)
			}
		})
	}
}

func TestCollectJSONRequestWithCRLF(t *testing.T) {
	input := "  \"version\": \"1\"\r\n}\r\n\r\n"
	reader := newTerminalLineReader(strings.NewReader(input))

	requestText, cancelled, err := collectJSONRequest("{", reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cancelled {
		t.Fatal("expected request not to be cancelled")
	}

	expected := "{\n  \"version\": \"1\"\n}"
	if requestText != expected {
		t.Errorf("expected %q, got %q", expected, requestText)
	}
}

func TestCollectJSONRequestCanBeCancelled(t *testing.T) {
	reader := newTerminalLineReader(strings.NewReader("  \"version\": \"1\"\n/cancel\n"))

	requestText, cancelled, err := collectJSONRequest("{", reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cancelled {
		t.Fatal("expected request to be cancelled")
	}
	if requestText != "" {
		t.Fatalf("cancelled request text = %q, want empty", requestText)
	}
}

func TestCollectJSONRequestRejectsPrematureEOFFromAsyncScanner(t *testing.T) {
	reader := newAsyncLineScanner(bufio.NewScanner(strings.NewReader("  \"version\": \"1\"\n}")))

	requestText, cancelled, err := collectJSONRequest("{", reader)
	if err == nil {
		t.Fatal("expected premature EOF error")
	}
	if cancelled {
		t.Fatal("did not expect request to be cancelled")
	}
	if requestText != "" {
		t.Fatalf("failed request text = %q, want empty", requestText)
	}
	if !strings.Contains(err.Error(), "ended before an empty terminating line") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCollectJSONRequestRejectsPrematureEOF(t *testing.T) {
	reader := newTerminalLineReader(strings.NewReader("  \"version\": \"1\"\n}"))

	requestText, cancelled, err := collectJSONRequest("{", reader)
	if err == nil {
		t.Fatal("expected premature EOF error")
	}
	if cancelled {
		t.Fatal("did not expect request to be cancelled")
	}
	if requestText != "" {
		t.Fatalf("failed request text = %q, want empty", requestText)
	}
	if !strings.Contains(err.Error(), "ended before an empty terminating line") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInitializeAgentsFolderCreatesStructure(t *testing.T) {
	workspace := t.TempDir()
	var output bytes.Buffer

	if err := initializeAgentsFolder(workspace, &output); err != nil {
		t.Fatalf("initializeAgentsFolder returned an error: %v", err)
	}

	expectedOutput := "Created .agents\nCreated .agents/skills\nCreated .agents/atr.json\n"
	if output.String() != expectedOutput {
		t.Fatalf("output = %q, want %q", output.String(), expectedOutput)
	}
	content, err := os.ReadFile(filepath.Join(workspace, ".agents", "atr.json"))
	if err != nil {
		t.Fatalf("read atr.json: %v", err)
	}
	if string(content) != initialConfigContent {
		t.Fatalf("atr.json = %q", content)
	}
	info, err := os.Stat(filepath.Join(workspace, ".agents", "skills"))
	if err != nil || !info.IsDir() {
		t.Fatalf("expected skills directory, got %v (%v)", info, err)
	}
}

func TestInitializeAgentsFolderNeverOverwrites(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".agents"), 0o755); err != nil {
		t.Fatalf("create directory: %v", err)
	}
	configPath := filepath.Join(workspace, ".agents", "atr.json")
	original := `{"bash":{"allow":["go test"]}}`
	if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var output bytes.Buffer

	if err := initializeAgentsFolder(workspace, &output); err != nil {
		t.Fatalf("initializeAgentsFolder returned an error: %v", err)
	}

	expectedOutput := ".agents already exists\nCreated .agents/skills\n.agents/atr.json already exists\n"
	if output.String() != expectedOutput {
		t.Fatalf("output = %q, want %q", output.String(), expectedOutput)
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(content) != original {
		t.Fatalf("config was overwritten: %q", content)
	}
}

func TestInitializeAgentsFolderIsIdempotent(t *testing.T) {
	workspace := t.TempDir()
	if err := initializeAgentsFolder(workspace, &bytes.Buffer{}); err != nil {
		t.Fatalf("first run returned an error: %v", err)
	}
	var output bytes.Buffer

	if err := initializeAgentsFolder(workspace, &output); err != nil {
		t.Fatalf("second run returned an error: %v", err)
	}

	expectedOutput := ".agents already exists\n.agents/skills already exists\n.agents/atr.json already exists\n"
	if output.String() != expectedOutput {
		t.Fatalf("output = %q, want %q", output.String(), expectedOutput)
	}
}

func TestInitializeAgentsFolderRejectsInvalidAgentsPath(t *testing.T) {
	t.Run("regular file", func(t *testing.T) {
		workspace := t.TempDir()
		if err := os.WriteFile(filepath.Join(workspace, ".agents"), []byte("x"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		if err := initializeAgentsFolder(workspace, &bytes.Buffer{}); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("symbolic link", func(t *testing.T) {
		workspace := t.TempDir()
		target := t.TempDir()
		if err := os.Symlink(target, filepath.Join(workspace, ".agents")); err != nil {
			t.Skipf("symbolic links are unavailable in this environment: %v", err)
		}
		if err := initializeAgentsFolder(workspace, &bytes.Buffer{}); err == nil {
			t.Fatal("expected an error")
		}
		if entries, _ := os.ReadDir(target); len(entries) != 0 {
			t.Fatalf("init wrote through the symbolic link: %v", entries)
		}
	})
}
