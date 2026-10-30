package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const bashHelperEnvironmentVariable = "ATR_BASH_HELPER"

func TestMain(m *testing.M) {
	runBashHelper(os.Getenv(bashHelperEnvironmentVariable))
	os.Exit(m.Run())
}

func runBashHelper(mode string) {
	if mode == "" {
		return
	}
	switch mode {
	case "output":
		fmt.Fprint(os.Stdout, "out-text\n")
		fmt.Fprint(os.Stderr, "err-text\n")
	case "exit":
		os.Exit(3)
	case "sleep":
		time.Sleep(time.Minute)
	case "large":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 300000))
	case "cwd":
		workingDirectory, _ := os.Getwd()
		fmt.Fprint(os.Stdout, workingDirectory)
	}
	os.Exit(0)
}
func newBashHelperWorkspace(t *testing.T, mode string) (string, string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}
	if strings.ContainsAny(executable, " \t") {
		t.Skip("test binary path contains whitespace")
	}
	workspace := t.TempDir()
	writeBashConfig(t, workspace, []string{executable})
	t.Setenv(bashHelperEnvironmentVariable, mode)
	return workspace, executable
}

func writeBashConfig(t *testing.T, workspace string, allow []string) {
	t.Helper()
	content, err := json.Marshal(Config{Bash: BashConfig{Allow: allow}})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	writeTestFile(t, workspace, ".agents/atr.json", string(content))
}

func TestExecuteBashActionCapturesOutputAndStreamsProgress(t *testing.T) {
	workspace, command := newBashHelperWorkspace(t, "output")
	var progress bytes.Buffer

	data, responseError := executeBashAction(workspace, Action{ID: "b1", Operation: "bash", Command: command}, 0, &progress)
	if responseError != nil {
		t.Fatalf("executeBashAction returned an error: %#v", responseError)
	}
	if data.ExitCode == nil || *data.ExitCode != 0 {
		t.Fatalf("exit code = %s, want 0", describeExitCode(data.ExitCode))
	}
	if strings.TrimSpace(data.Stdout) != "out-text" || strings.TrimSpace(data.Stderr) != "err-text" {
		t.Fatalf("stdout = %q, stderr = %q", data.Stdout, data.Stderr)
	}
	progressText := progress.String()
	for _, expected := range []string{"Running bash:", "out-text", "err-text"} {
		if !strings.Contains(progressText, expected) {
			t.Fatalf("progress %q does not contain %q", progressText, expected)
		}
	}
}

func TestExecuteBashActionRunsInWorkspace(t *testing.T) {
	workspace, command := newBashHelperWorkspace(t, "cwd")

	data, responseError := executeBashAction(workspace, Action{ID: "b1", Operation: "bash", Command: command}, 0, io.Discard)
	if responseError != nil {
		t.Fatalf("executeBashAction returned an error: %#v", responseError)
	}
	reportedInfo, err := os.Stat(strings.TrimSpace(data.Stdout))
	if err != nil {
		t.Fatalf("stat reported working directory %q: %v", data.Stdout, err)
	}
	workspaceInfo, err := os.Stat(workspace)
	if err != nil {
		t.Fatalf("stat workspace: %v", err)
	}
	if !os.SameFile(reportedInfo, workspaceInfo) {
		t.Fatalf("working directory = %q, want %q", data.Stdout, workspace)
	}
}

func TestExecuteRequestReturnsNonZeroExitCodeAsData(t *testing.T) {
	workspace, command := newBashHelperWorkspace(t, "exit")
	request := Request{Version: protocolVersion, Actions: []Action{{ID: "b1", Operation: "bash", Command: command}}}

	var response Response
	if err := json.Unmarshal([]byte(ExecuteRequest(workspace, request, io.Discard)), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "success" || response.Error != nil {
		t.Fatalf("status = %q, error = %#v", response.Status, response.Error)
	}
	exitCode := response.Results[0].Data.ExitCode
	if exitCode == nil || *exitCode != 3 {
		t.Fatalf("exit code = %s, want 3", describeExitCode(exitCode))
	}
}

func describeExitCode(exitCode *int) string {
	if exitCode == nil {
		return "<nil>"
	}
	return fmt.Sprint(*exitCode)
}

func TestExecuteBashActionTimesOut(t *testing.T) {
	workspace, command := newBashHelperWorkspace(t, "sleep")
	action := Action{ID: "b1", Operation: "bash", Command: command, TimeoutSeconds: 1}

	startedAt := time.Now()
	data, responseError := executeBashAction(workspace, action, 0, io.Discard)
	assertResponseErrorCode(t, responseError, "BASH_TIMEOUT")
	if data == nil || !data.TimedOut {
		t.Fatalf("expected timedOut data, got %#v", data)
	}
	if elapsed := time.Since(startedAt); elapsed > 15*time.Second {
		t.Fatalf("timeout took %v", elapsed)
	}
}

func TestExecuteBashActionReportsStartFailure(t *testing.T) {
	workspace := t.TempDir()
	command := "atr-command-that-does-not-exist-" + filepath.Base(workspace)
	writeBashConfig(t, workspace, []string{command})

	data, responseError := executeBashAction(workspace, Action{ID: "b1", Operation: "bash", Command: command}, 0, io.Discard)
	if responseError == nil {
		if data.ExitCode == nil || *data.ExitCode == 0 {
			t.Fatalf("expected a failure, got %#v", data)
		}
		return
	}
	assertResponseErrorCode(t, responseError, "BASH_FAILED")
}

func TestExecuteBashActionRequiresValidConfig(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, workspace string)
	}{
		{name: "missing config", setup: func(t *testing.T, workspace string) {}},
		{name: "invalid config", setup: func(t *testing.T, workspace string) {
			writeTestFile(t, workspace, ".agents/atr.json", `{"bash":`)
		}},
		{name: "empty allowlist", setup: func(t *testing.T, workspace string) {
			writeBashConfig(t, workspace, []string{})
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			test.setup(t, workspace)

			_, responseError := executeBashAction(workspace, Action{ID: "b1", Operation: "bash", Command: "go version"}, 0, io.Discard)
			assertResponseErrorCode(t, responseError, "BASH_NOT_ALLOWED")
		})
	}
}

func TestExecuteBashActionRejectsShellControlCharacters(t *testing.T) {
	commands := []string{
		"go version; go env",
		"go version && go env",
		"go version | go env",
		"go version > output.txt",
		"go version < input.txt",
		"go version `go env`",
		"go version $(go env)",
		"go version ${GOOS}",
		"go version (go env)",
		"go version {go env}",
		"go version\ngo env",
		"go version\rgo env",
		`go version ";"`,
	}

	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			workspace := t.TempDir()
			writeBashConfig(t, workspace, []string{"go"})

			_, responseError := executeBashAction(workspace, Action{ID: "b1", Operation: "bash", Command: command}, 0, io.Discard)
			assertResponseErrorCode(t, responseError, "BASH_NOT_ALLOWED")
			assertPathDoesNotExist(t, filepath.Join(workspace, "output.txt"))
		})
	}
}

func TestIsBashCommandAllowed(t *testing.T) {
	tests := []struct {
		name    string
		command string
		allow   []string
		want    bool
	}{
		{name: "exact entry", command: "go test", allow: []string{"go test"}, want: true},
		{name: "entry with arguments", command: "go test -run X ./...", allow: []string{"go test"}, want: true},
		{name: "single word entry allows subcommands", command: "go vet ./...", allow: []string{"go"}, want: true},
		{name: "extra whitespace", command: "  go   test  ", allow: []string{"go test"}, want: true},
		{name: "second entry", command: "git status", allow: []string{"go test", "git status"}, want: true},
		{name: "different subcommand", command: "go vet", allow: []string{"go test"}, want: false},
		{name: "partial word", command: "gofmt -l .", allow: []string{"go"}, want: false},
		{name: "case sensitive", command: "Go test", allow: []string{"go test"}, want: false},
		{name: "command shorter than entry", command: "go", allow: []string{"go test"}, want: false},
		{name: "empty allowlist", command: "go test", allow: nil, want: false},
		{name: "blank entry matches nothing", command: "go test", allow: []string{"  "}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isBashCommandAllowed(test.command, test.allow); got != test.want {
				t.Fatalf("isBashCommandAllowed(%q, %v) = %v, want %v", test.command, test.allow, got, test.want)
			}
		})
	}
}

func TestExecuteRequestTrimsLargeBashOutput(t *testing.T) {
	workspace, command := newBashHelperWorkspace(t, "large")
	request := Request{Version: protocolVersion, Actions: []Action{{ID: "b1", Operation: "bash", Command: command}}}

	responseText := ExecuteRequest(workspace, request, io.Discard)
	if len(responseText) > MaximumTransferChars() {
		t.Fatalf("response has %d characters, limit is %d", len(responseText), MaximumTransferChars())
	}
	var response Response
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := response.Results[len(response.Results)-1].Data
	if data == nil || !data.Truncated || data.Stdout == "" {
		t.Fatalf("expected truncated non-empty stdout, got %#v", data)
	}
}

func TestValidateBashAction(t *testing.T) {
	tests := []struct {
		name    string
		request string
		wantErr bool
	}{
		{name: "valid", request: `{"version":"1","actions":[{"id":"b1","operation":"bash","command":"go test"}]}`},
		{name: "valid with timeout", request: `{"version":"1","actions":[{"id":"b1","operation":"bash","command":"go test","timeoutSeconds":30}]}`},
		{name: "missing command", request: `{"version":"1","actions":[{"id":"b1","operation":"bash"}]}`, wantErr: true},
		{name: "blank command", request: `{"version":"1","actions":[{"id":"b1","operation":"bash","command":"   "}]}`, wantErr: true},
		{name: "timeout too large", request: `{"version":"1","actions":[{"id":"b1","operation":"bash","command":"go test","timeoutSeconds":31}]}`, wantErr: true},
		{name: "negative timeout", request: `{"version":"1","actions":[{"id":"b1","operation":"bash","command":"go test","timeoutSeconds":-1}]}`, wantErr: true},
		{name: "unsupported field", request: `{"version":"1","actions":[{"id":"b1","operation":"bash","command":"go test","path":"x"}]}`, wantErr: true},
		{name: "command on another operation", request: `{"version":"1","actions":[{"id":"r1","operation":"read","paths":["a.go"],"command":"go test"}]}`, wantErr: true},
		{name: "timeout on another operation", request: `{"version":"1","actions":[{"id":"t1","operation":"tree","path":".","timeoutSeconds":5}]}`, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseAndValidateRequest(test.request)
			if (err != nil) != test.wantErr {
				t.Fatalf("ParseAndValidateRequest error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
