package app

import (
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
