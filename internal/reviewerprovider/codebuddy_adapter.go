package reviewerprovider

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"

	"github.com/gentleman-programming/gentle-ai/v4/internal/model"
)

// CodeBuddyAdapter invokes CodeBuddy Code with an opaque provider invocation
// and returns its stdout bytes without interpreting them.
//
// CodeBuddy is configuration-compatible with Claude Code, so this adapter
// mirrors ClaudeAdapter's fresh-reviewer boundary. The one behavioral
// difference: CodeBuddy starts a local HTTP web server on every run and
// remembers its port in ~/.codebuddy/web-ui-port.json. Two concurrent captures
// would collide (EADDRINUSE on the remembered port), so every invocation is
// pinned to a unique ephemeral port that is released immediately before the
// reviewer process starts.
type CodeBuddyAdapter struct {
	LookPath func(string) (string, error)
	// Model is optional; an absent or invalid assignment preserves CodeBuddy's default.
	Model model.ClaudeModelAlias
	// Effort is optional and must be supported by Model.
	Effort         model.ClaudeEffort
	commandContext func(context.Context, string, ...string) *exec.Cmd
	// listenPort reserves the ephemeral port handed to `--port`. Overridable in tests.
	listenPort func() (int, func(), error)
}

// NewCodeBuddyAdapter returns an adapter using the CodeBuddy binary from PATH.
func NewCodeBuddyAdapter() *CodeBuddyAdapter {
	return &CodeBuddyAdapter{LookPath: exec.LookPath, commandContext: exec.CommandContext, listenPort: reserveEphemeralPort}
}

// codebuddyReviewerArguments locks the fresh-reviewer boundary without
// `--bare`: the same flags ClaudeAdapter uses, each with a CodeBuddy
// equivalent. `--setting-sources ""` drops user/project/local settings (hooks,
// plugins, permissions), the explicit `--system-prompt` replaces default
// context assembly, `--tools ""` leaves no live tools, and the empty scratch
// working directory carries no project state.
var codebuddyReviewerArguments = []string{
	"--print", "--output-format", "text", "--tools", "",
	"--permission-mode", "dontAsk", "--no-session-persistence",
	"--setting-sources", "",
	"--system-prompt", "You are an isolated code reviewer process. Your only instructions are those delivered in the user message.",
}

// Review runs CodeBuddy in an empty temporary directory. The provider material
// is delivered through stdin so command arguments stay opaque.
func (adapter *CodeBuddyAdapter) Review(ctx context.Context, invocation Invocation) ([]byte, error) {
	lookPath := adapter.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	binary, err := lookPath("codebuddy")
	if err != nil {
		return nil, fmt.Errorf("codebuddy reviewer transport unavailable: %w", err)
	}

	scratch, err := os.MkdirTemp("", "gentle-ai-codebuddy-reviewer-*")
	if err != nil {
		return nil, fmt.Errorf("codebuddy reviewer transport unavailable: create scratch directory: %w", err)
	}
	defer os.RemoveAll(scratch)

	// Pin a unique ephemeral port so concurrent captures never collide on the
	// remembered web-ui port, then release it for CodeBuddy to bind.
	listenPort := adapter.listenPort
	if listenPort == nil {
		listenPort = reserveEphemeralPort
	}
	port, release, err := listenPort()
	if err != nil {
		return nil, fmt.Errorf("codebuddy reviewer transport unavailable: reserve port: %w", err)
	}
	if release != nil {
		release()
	}
	portArgument := strconv.Itoa(port)

	commandContext := adapter.commandContext
	if commandContext == nil {
		commandContext = exec.CommandContext
	}
	arguments := append([]string(nil), codebuddyReviewerArguments...)
	arguments = append(arguments, "--port", portArgument)
	if adapter.Model.Valid() {
		arguments = append(arguments, "--model", adapter.Model.String())
		// Match generated Claude/CodeBuddy subagents: default effort inherits,
		// and unsupported model/effort combinations never reach the CLI.
		if adapter.Effort != model.ClaudeEffortDefault && model.ClaudeEffortAllowedForModel(adapter.Model, adapter.Effort) {
			arguments = append(arguments, "--effort", string(adapter.Effort))
		}
	}
	command := commandContext(ctx, binary, arguments...)
	command.Dir = scratch
	// On Windows, `codebuddy` from an npm-style install resolves to a `.cmd`
	// shim; launching it goes through cmd.exe's own command-line reparsing,
	// which needs its own quoting when the resolved path contains a space.
	// This is a no-op for every other binary and platform.
	configureWindowsBatchLaunch(command, binary, arguments)
	command.Stdin = bytes.NewReader(invocation.Prompt())
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("codebuddy reviewer transport failed: %w: %s", err, reviewerTransportFailureDetail(stderr.String(), stdout.String()))
	}
	return stdout.Bytes(), nil
}

// reserveEphemeralPort binds an unspecified-address TCP port so the OS assigns
// a free one, returning the port and a release func that closes the listener.
// The race window between release and CodeBuddy's bind is nanoseconds and the
// port was just proven free; a collision surfaces as a transport error, never
// as silent cross-talk.
func reserveEphemeralPort() (int, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, err
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		return 0, nil, fmt.Errorf("unexpected listener address %T", listener.Addr())
	}
	port := address.Port
	return port, func() { _ = listener.Close() }, nil
}
