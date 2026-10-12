package audit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"prompttree/aicrew/internal/runner"
	"prompttree/aicrew/internal/secretscan"
)

type okRunner struct{}

func (okRunner) Run(context.Context, runner.Command) (runner.Result, error) {
	return runner.Result{ExitCode: 0}, nil
}

type sender struct {
	fail    bool
	entries []Entry
	prompts []Prompt
}

func (s *sender) WriteAudit(_ context.Context, e []Entry, p []Prompt) error {
	if s.fail {
		return errors.New("offline")
	}
	s.entries, s.prompts = append(s.entries, e...), append(s.prompts, p...)
	return nil
}

func TestCommandsAndModelCallsAreRecordedWithoutSecrets(t *testing.T) {
	secret := strings.Join([]string{"Mon", "day-", "Coffee-42-x"}, "")
	sink := &Sink{Dir: t.TempDir(), Scanner: secretscan.New(map[string]string{secret: "claude_token"})}
	r := Runner{Inner: okRunner{}, Sink: sink}
	ctx := WithTask(context.Background(), "t1", 2)
	_, _ = r.Run(ctx, runner.Command{Exe: "git", Args: []string{"add", "-A"}})
	_, _ = r.Run(ctx, runner.Command{Exe: "docker", Args: []string{"run", "img", "claude", "-p", "--model", "sonnet"},
		Stdin: "Сделай кнопку. токен " + secret})

	off := &sender{fail: true}
	if err := sink.Flush(context.Background(), off); err == nil {
		t.Fatal("offline must keep the queue")
	}
	srv := &sender{}
	if err := sink.Flush(context.Background(), srv); err != nil {
		t.Fatal(err)
	}
	if len(srv.entries) != 2 || srv.entries[0].Kind != "command" || srv.entries[1].Kind != "model" ||
		srv.entries[1].TaskID != "t1" || *srv.entries[1].Attempt != 2 {
		t.Fatalf("%+v", srv.entries)
	}
	if !strings.Contains(string(srv.entries[1].Data), `"model":"sonnet"`) || !strings.Contains(string(srv.entries[1].Data), "prompt_sha256") {
		t.Fatal(string(srv.entries[1].Data))
	}
	if len(srv.prompts) != 1 || strings.Contains(srv.prompts[0].Text, secret) || !strings.Contains(srv.prompts[0].Text, "[REDACTED:claude_token]") {
		t.Fatalf("%+v", srv.prompts)
	}
	again := &sender{}
	_ = sink.Flush(context.Background(), again)
	if len(again.entries) != 0 {
		t.Fatal("sent entries must not be sent twice")
	}
}
