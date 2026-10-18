package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"prompttree/aicrew/internal/runner"
)

type memAuth struct{ v string }

func (m *memAuth) Get() (string, error) { return m.v, nil }
func (m *memAuth) Set(v string) error   { m.v = v; return nil }

// fakeCodexDocker отвечает заданными строками и, как настоящий Codex, продлевает вход в CODEX_HOME.
type fakeCodexDocker struct {
	args   []string
	stdin  string
	lines  []string
	home   string
	sawOld string
}

func (f *fakeCodexDocker) Run(_ context.Context, c runner.Command) (runner.Result, error) {
	f.args, f.stdin = c.Args, c.Stdin
	for i, a := range c.Args {
		if a == "-v" && strings.HasSuffix(c.Args[i+1], ":"+codexHome) {
			f.home = strings.TrimSuffix(c.Args[i+1], ":"+codexHome)
		}
	}
	if f.home != "" {
		b, _ := os.ReadFile(filepath.Join(f.home, "auth.json"))
		f.sawOld = string(b)
		_ = os.WriteFile(filepath.Join(f.home, "auth.json"), []byte(`{"tokens":{"access_token":"fresh"}}`), 0o600)
	}
	for _, l := range f.lines {
		c.Stdout(l)
	}
	return runner.Result{}, nil
}

func TestCodexRunsInContainerAndRefreshesLogin(t *testing.T) {
	auth := &memAuth{v: `{"tokens":{"access_token":"old"}}`}
	fd := &fakeCodexDocker{lines: []string{
		`{"type":"thread.started","thread_id":"t1"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"i1","type":"command_execution","command":"flutter test"}}`,
		`{"type":"item.completed","item":{"id":"i2","type":"agent_message","text":"Готово: фильтр и тесты."}}`,
		`{"type":"turn.completed","usage":{"input_tokens":1200,"cached_input_tokens":800,"output_tokens":300,"reasoning_output_tokens":100}}`,
	}}
	c := &Codex{R: fd, Image: "img", Model: "gpt-x", Auth: auth}
	skills := t.TempDir()
	res, err := c.Run(context.Background(), Request{TaskID: "t", Attempt: 1, Workdir: "/repo", Prompt: "Сделай фильтр",
		Mounts: []Mount{{Host: skills, Container: SkillsMount, ReadOnly: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "Готово: фильтр и тесты." || res.Usage.InputTokens != 1200 || res.Usage.OutputTokens != 400 || res.Usage.Turns != 1 {
		t.Fatalf("%+v", res)
	}
	a := strings.Join(fd.args, " ")
	for _, want := range []string{"exec --json --skip-git-repo-check --ephemeral", "--dangerously-bypass-approvals-and-sandbox",
		"--model gpt-x", "/repo:/work", skills + ":" + codexHome + "/skills:ro", "CODEX_HOME=" + codexHome} {
		if !strings.Contains(a, want) {
			t.Errorf("args lack %q: %s", want, a)
		}
	}
	if strings.Contains(a, "access_token") || !strings.Contains(fd.stdin, "Сделай фильтр") {
		t.Fatal("login must never be in the command line; the task goes through stdin")
	}
	if fd.sawOld != `{"tokens":{"access_token":"old"}}` {
		t.Fatalf("codex must see the saved login, got %q", fd.sawOld)
	}
	if auth.v != `{"tokens":{"access_token":"fresh"}}` {
		t.Fatalf("refreshed login must be saved back: %q", auth.v)
	}
	if _, err := os.Stat(fd.home); !os.IsNotExist(err) {
		t.Fatal("temporary CODEX_HOME must be removed")
	}
}

func TestCodexReadOnlyReviewAndMissingLogin(t *testing.T) {
	fd := &fakeCodexDocker{lines: []string{`{"type":"turn.completed","usage":{}}`}}
	c := &Codex{R: fd, Image: "img", Auth: &memAuth{v: "{}"}}
	if _, err := c.Run(context.Background(), Request{TaskID: "t", Attempt: 1, Workdir: "/repo", Prompt: "p", ReadOnly: true, Raw: true}); err != nil {
		t.Fatal(err)
	}
	a := strings.Join(fd.args, " ")
	if !strings.Contains(a, "/repo:/work:ro") || !strings.Contains(a, "--sandbox read-only") || strings.Contains(a, "bypass") {
		t.Fatalf("review must be read-only: %s", a)
	}
	_, err := (&Codex{R: fd, Auth: &memAuth{}}).Run(context.Background(), Request{TaskID: "t", Attempt: 1})
	if f, ok := err.(*Failure); !ok || f.Code != "no_codex_auth" {
		t.Fatalf("%v", err)
	}
}

func TestParseCodexStreamFailures(t *testing.T) {
	// Настоящий вывод codex-cli 0.160.0 без входа (18.10.2026).
	unauthorized := []string{
		`{"type":"thread.started","thread_id":"01a14fcd"}`,
		`{"type":"turn.started"}`,
		`2026-10-18T16:16:56Z ERROR codex_api::endpoint::responses_websocket: failed to connect`,
		`{"type":"error","message":"Reconnecting... 5/5 (unexpected status 401 Unauthorized: Missing bearer or basic authentication in header)"}`,
	}
	if _, f := ParseCodexStream(unauthorized); f == nil || f.Class != "AUTH_ERROR" {
		t.Fatalf("401 must be AUTH_ERROR: %+v", f)
	}
	limit := []string{`{"type":"turn.failed","error":{"message":"You've hit your usage limit. Try again later."}}`}
	if _, f := ParseCodexStream(limit); f == nil || f.Class != "RATE_LIMIT" {
		t.Fatalf("usage limit must be RATE_LIMIT: %+v", f)
	}
	if _, f := ParseCodexStream(nil); f == nil || f.Code != "no_result" {
		t.Fatalf("%+v", f)
	}
}
