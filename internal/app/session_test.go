package app

import (
	"strings"
	"testing"
)

func TestTerminalLineReaderHandlesLF(t *testing.T) {
	input := "line1\nline2\n"
	reader := newTerminalLineReader(strings.NewReader(input))

	if !reader.Scan() {
		t.Fatalf("expected Scan() to succeed for line 1, got err: %v", reader.Err())
	}
	if reader.Text() != "line1" {
		t.Errorf("expected line1, got %q", reader.Text())
	}

	if !reader.Scan() {
		t.Fatalf("expected Scan() to succeed for line 2, got err: %v", reader.Err())
	}
	if reader.Text() != "line2" {
		t.Errorf("expected line2, got %q", reader.Text())
	}

	if reader.Scan() {
		t.Errorf("expected Scan() to return false at EOF, got true with text %q", reader.Text())
	}
}

func TestTerminalLineReaderHandlesCRLF(t *testing.T) {
	input := "line1\r\nline2\r\n"
	reader := newTerminalLineReader(strings.NewReader(input))

	if !reader.Scan() {
		t.Fatalf("expected Scan() to succeed for line 1, got err: %v", reader.Err())
	}
	if reader.Text() != "line1" {
		t.Errorf("expected line1, got %q", reader.Text())
	}

	if !reader.Scan() {
		t.Fatalf("expected Scan() to succeed for line 2, got err: %v", reader.Err())
	}
	if reader.Text() != "line2" {
		t.Errorf("expected line2, got %q", reader.Text())
	}

	if reader.Scan() {
		t.Errorf("expected Scan() to return false at EOF, got true with text %q", reader.Text())
	}
}

func TestCollectJSONRequestWithCRLF(t *testing.T) {
	input := "  \"version\": \"1\"\r\n}\r\n\r\n"
	reader := newTerminalLineReader(strings.NewReader(input))

	firstLine := "{"
	requestText, cancelled, err := collectJSONRequest(firstLine, reader)
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
