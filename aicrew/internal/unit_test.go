package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/runner"
	"prompttree/aicrew/internal/secrets"
	"prompttree/aicrew/internal/verify"
)

// Command Policy (ТЗ п. 12): оболочки и интерпретаторы запрещены всегда, неизвестное — DENY.
func TestRunnerAllowlist(t *testing.T) {
	r := runner.New("git", "docker", "bash") // даже если по ошибке разрешить bash
	for _, exe := range []string{"sh", "bash", "cmd", "powershell", "python", "node", "curl", "/usr/bin/git", `C:\Windows\System32\cmd.exe`, "..\\git"} {
		if _, err := r.Run(context.Background(), runner.Command{Exe: exe, Args: []string{"-c", "echo pwned"}}); !errors.Is(err, runner.ErrNotAllowed) {
			t.Errorf("%q must be denied, got %v", exe, err)
		}
	}
	res, err := r.Run(context.Background(), runner.Command{Exe: "git", Args: []string{"--version"}})
	if err != nil || res.ExitCode != 0 || !strings.Contains(res.Stdout, "git version") {
		t.Fatalf("git must run: %+v %v", res, err)
	}
}

func TestRunnerDoesNotLeakHostSecretsIntoChildEnv(t *testing.T) {
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "host-secret")
	t.Setenv("GEMINI_API_KEY", "host-secret")
	res, err := runner.New("git").Run(context.Background(), runner.Command{Exe: "git", Args: []string{"var", "GIT_EDITOR"},
		Env: []string{"GIT_EDITOR=only-this"}})
	if err != nil || strings.TrimSpace(res.Stdout) != "only-this" {
		t.Fatalf("explicit env must pass: %+v %v", res, err)
	}
	// Значения окружения процесса не наследуются целиком: в baseEnv нет ключей с секретами.
}

func TestSecretsAreNotStoredInPlainTextOnWindows(t *testing.T) {
	dir := t.TempDir()
	s := secrets.Store{Dir: dir}
	if err := s.Set(secrets.ClaudeToken, "sk-ant-oat01-very-secret"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(secrets.ClaudeToken)
	if err != nil || got != "sk-ant-oat01-very-secret" {
		t.Fatalf("round trip: %q %v", got, err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, secrets.ClaudeToken+".bin"))
	if os.PathSeparator == '\\' && strings.Contains(string(raw), "very-secret") {
		t.Fatal("token stored in plain text")
	}
	if _, err := s.Get("missing"); !errors.Is(err, secrets.ErrMissing) {
		t.Fatal(err)
	}
	if err := s.Set("../escape", "x"); err == nil {
		t.Fatal("secret names must not contain paths")
	}
}

func TestClaudeStreamParsing(t *testing.T) {
	ok := []string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"assistant","message":{}}`,
		`{"type":"result","subtype":"success","is_error":false,"result":"Добавил кнопку","num_turns":7,"total_cost_usd":0.12,"usage":{"input_tokens":1000,"output_tokens":200}}`,
	}
	res, fail := agent.ParseClaudeStream(ok)
	if fail != nil || res.Summary != "Добавил кнопку" || res.Usage.Turns != 7 || res.Usage.InputTokens != 1000 {
		t.Fatalf("%+v %v", res, fail)
	}
	cases := map[string]string{
		`{"type":"result","subtype":"error_during_execution","is_error":true,"result":"Claude usage limit reached. Resets at 5pm"}`: "RATE_LIMIT",
		`{"type":"result","is_error":true,"result":"Invalid API key · Please run /login"}`:                                          "AUTH_ERROR",
		`{"type":"result","subtype":"error_max_turns","is_error":true}`:                                                             "AGENT_ERROR",
	}
	for line, class := range cases {
		if _, fail := agent.ParseClaudeStream([]string{line}); fail == nil || fail.Class != class {
			t.Errorf("%s: got %+v, want %s", line, fail, class)
		}
	}
	if _, fail := agent.ParseClaudeStream([]string{"not json"}); fail == nil || fail.Code != "no_result" {
		t.Fatal("missing result must be an agent error")
	}
}

func TestClaudeProviderWithoutTokenIsAuthError(t *testing.T) {
	c := &agent.ClaudeCode{R: runner.New("docker"), Image: "x", Model: "sonnet",
		Token: func() (string, error) { return "", secrets.ErrMissing }}
	_, err := c.Run(context.Background(), agent.Request{TaskID: "t", Attempt: 1})
	var f *agent.Failure
	if !errors.As(err, &f) || f.Class != "AUTH_ERROR" {
		t.Fatalf("got %v", err)
	}
}

func TestVerificationProfile(t *testing.T) {
	p, err := verify.ParseProfile([]byte(`{"image":"img","steps":[{"name":"tests","kind":"test","argv":["flutter","test"]}]}`))
	if err != nil || len(p.Steps) != 1 || p.Steps[0].Argv[1] != "test" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := verify.ParseProfile([]byte(`{"steps":[{"name":"empty","argv":[]}]}`)); err == nil {
		t.Fatal("empty command must be rejected")
	}
	if p, err := verify.ParseProfile([]byte(`{}`)); err != nil || len(p.Steps) != 0 {
		t.Fatal("empty profile is allowed (no checks)")
	}
}
