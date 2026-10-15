package reviewerprovider

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/model"
)

// TestCodeBuddyAdapterRunsIsolatedReviewer pins the fresh-reviewer boundary:
// the adapter shells out to the codebuddy binary with the strict flags, pins an
// explicit ephemeral --port (so concurrent captures never collide on the
// remembered web-ui port), delivers the prompt through stdin, and returns
// stdout untouched.
func TestCodeBuddyAdapterRunsIsolatedReviewer(t *testing.T) {
	record := filepath.Join(t.TempDir(), "args.txt")
	stdinFile := filepath.Join(t.TempDir(), "stdin.txt")
	script := filepath.Join(t.TempDir(), "codebuddy")
	// A fake binary that records argv and stdin, then prints a sentinel.
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + record + "\ncat > " + stdinFile + "\nprintf 'REVIEWER-OUT'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	adapter := NewCodeBuddyAdapter()
	adapter.LookPath = func(string) (string, error) { return script, nil }
	adapter.listenPort = func() (int, func(), error) { return 43210, func() {}, nil }

	out, err := adapter.Review(context.Background(), NewInvocation([]byte("PROMPT-BYTES")))
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if string(out) != "REVIEWER-OUT" {
		t.Fatalf("Review() = %q, want REVIEWER-OUT", out)
	}

	args, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	got := string(args)
	for _, want := range []string{"--print", "--output-format", "--tools", "--permission-mode", "dontAsk", "--no-session-persistence", "--setting-sources", "--system-prompt", "--port", "43210"} {
		if !strings.Contains(got, want) {
			t.Errorf("argv missing %q; got:\n%s", want, got)
		}
	}
	stdin, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(stdin) != "PROMPT-BYTES" {
		t.Fatalf("stdin = %q, want PROMPT-BYTES (prompt must travel through stdin, not argv)", stdin)
	}
	if strings.Contains(got, "PROMPT-BYTES") {
		t.Fatal("prompt leaked into argv")
	}
}

// TestCodeBuddyAdapterReservesUniquePort confirms the default port reservation
// yields a bindable port and a release that frees it.
func TestCodeBuddyAdapterReservesUniquePort(t *testing.T) {
	port, release, err := reserveEphemeralPort()
	if err != nil {
		t.Fatalf("reserveEphemeralPort() error = %v", err)
	}
	if port <= 0 {
		t.Fatalf("reserveEphemeralPort() port = %d, want > 0", port)
	}
	release()
}

// TestCodeBuddyAdapterTransportUnavailable pins the fail-closed answer when the
// binary is absent.
func TestCodeBuddyAdapterTransportUnavailable(t *testing.T) {
	adapter := NewCodeBuddyAdapter()
	adapter.LookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	if _, err := adapter.Review(context.Background(), NewInvocation([]byte("x"))); err == nil {
		t.Fatal("Review() error = nil, want transport-unavailable error")
	}
}

// TestCapturesInProcessIncludesCodeBuddy pins the transport classification.
func TestCapturesInProcessIncludesCodeBuddy(t *testing.T) {
	if !CapturesInProcess(model.AgentCodeBuddy) {
		t.Fatal("CapturesInProcess(codebuddy) = false, want true")
	}
	if !RegisteredRuntime(model.AgentCodeBuddy) {
		t.Fatal("RegisteredRuntime(codebuddy) = false, want true")
	}
}

// TestClaudeEffortAliasParses guards the shared model alias type the adapter
// reuses from the Claude transport.
func TestClaudeEffortAliasParses(t *testing.T) {
	var zero model.ClaudeModelAlias
	if zero.Valid() {
		t.Fatal("zero ClaudeModelAlias should be invalid (preserve native default)")
	}
}
