package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"prompttree/aicrew/internal/runner"
)

// TokenSource выдаёт токен подписки Claude Code только на время запуска (решение 7.1, п. 2).
type TokenSource func() (string, error)

// ClaudeCode — исполнитель в одноразовом Docker-контейнере (окружение container, решение 7.1).
// В контейнер монтируется только worktree задачи. Токен передаётся через окружение процесса
// docker (`-e NAME` без значения), поэтому не виден в командной строке.
type ClaudeCode struct {
	R            runner.Runner
	Image        string
	Model        string
	Token        TokenSource
	AllowedTools []string
	Network      string // сеть контейнера; allowlist-прокси — этап 7.6
}

func (c *ClaudeCode) Name() string { return "claude_code/" + c.Model }

func (c *ClaudeCode) Run(ctx context.Context, req Request) (Result, error) {
	token, err := c.Token()
	if err != nil || token == "" {
		return Result{}, &Failure{Class: "AUTH_ERROR", Code: "no_claude_token",
			Message: "Claude Code token is not set on this host: run `aicrew-host set-claude-token`"}
	}
	maxTurns := req.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 60
	}
	tools := c.AllowedTools
	if len(tools) == 0 {
		tools = []string{"Read", "Edit", "Write", "Glob", "Grep", "Bash"}
	}
	args := []string{"run", "--rm", "-i", "--name", ContainerName(req.TaskID, req.Attempt),
		"--memory", "6g", "--cpus", "4", "--pids-limit", "512",
		"-v", req.Workdir + ":/work", "-w", "/work",
		"-e", "CLAUDE_CODE_OAUTH_TOKEN"}
	if c.Network != "" {
		args = append(args, "--network", c.Network)
	}
	args = append(args, c.Image,
		"claude", "-p", // текст задачи — через stdin
		"--output-format", "stream-json", "--verbose",
		"--model", c.Model,
		"--max-turns", strconv.Itoa(maxTurns),
		"--permission-mode", "acceptEdits",
		"--allowedTools", strings.Join(tools, ","))
	var lines []string
	res, err := c.R.Run(ctx, runner.Command{
		Exe: "docker", Args: args, Timeout: req.Timeout, Stdin: BuildPrompt(req),
		Env:    []string{"CLAUDE_CODE_OAUTH_TOKEN=" + token},
		Stdout: func(l string) { lines = append(lines, l) },
	})
	if err != nil {
		return Result{}, &Failure{Class: "ENVIRONMENT_FAILURE", Code: "docker_failed", Message: err.Error()}
	}
	if res.TimedOut {
		_ = c.Stop(context.WithoutCancel(ctx), req.TaskID, req.Attempt)
		return Result{}, &Failure{Class: "TIMEOUT", Code: "agent_runtime", Message: fmt.Sprintf("agent ran longer than %s", req.Timeout)}
	}
	out, fail := ParseClaudeStream(lines)
	out.ExitCode = res.ExitCode
	if fail != nil {
		if fail.Code == "no_result" && res.ExitCode == 125 {
			return out, &Failure{Class: "ENVIRONMENT_FAILURE", Code: "container_start", Message: tail(res.Stderr, 400)}
		}
		return out, fail
	}
	return out, nil
}

func (c *ClaudeCode) Stop(ctx context.Context, taskID string, attempt int) error {
	_, err := c.R.Run(ctx, runner.Command{Exe: "docker", Args: []string{"kill", ContainerName(taskID, attempt)}, Timeout: 30 * time.Second})
	return err
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
