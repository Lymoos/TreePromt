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
	// LocalBaseURL — Junior (7.5b): Claude Code ходит не в облако, а в Ollama на этом ПК
	// (Anthropic-совместимый API Ollama). Токен подписки тогда не нужен и не передаётся.
	LocalBaseURL string
}

func (c *ClaudeCode) Name() string { return "claude_code/" + c.Model }

func (c *ClaudeCode) Run(ctx context.Context, req Request) (Result, error) {
	var env []string
	if c.LocalBaseURL != "" {
		env = []string{"ANTHROPIC_BASE_URL=" + c.LocalBaseURL, "ANTHROPIC_AUTH_TOKEN=ollama", "ANTHROPIC_API_KEY="}
	} else {
		token, err := c.Token()
		if err != nil || token == "" {
			return Result{}, &Failure{Class: "AUTH_ERROR", Code: "no_claude_token",
				Message: "Claude Code token is not set on this host: run `aicrew-host set-claude-token`"}
		}
		env = []string{"CLAUDE_CODE_OAUTH_TOKEN=" + token}
	}
	maxTurns := req.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 60
	}
	tools := append([]string{}, c.AllowedTools...)
	if len(tools) == 0 {
		tools = []string{"Read", "Edit", "Write", "Glob", "Grep", "Bash"}
	}
	tools = append(tools, req.ExtraTools...) // инструменты MCP-профиля задачи
	name := req.Name
	if name == "" {
		name = ContainerName(req.TaskID, req.Attempt)
	}
	mount := req.Workdir + ":/work"
	if req.ReadOnly {
		mount += ":ro"
	}
	args := []string{"run", "--rm", "-i", "--name", name,
		"--memory", "6g", "--cpus", "4", "--pids-limit", "512",
		"-v", mount, "-w", "/work"}
	for _, m := range req.Mounts {
		v := m.Host + ":" + m.Container
		if m.ReadOnly {
			v += ":ro"
		}
		args = append(args, "-v", v)
	}
	for _, kv := range env {
		args = append(args, "-e", strings.SplitN(kv, "=", 2)[0]) // значение — из окружения процесса docker
	}
	if c.Network != "" {
		args = append(args, "--network", c.Network)
	}
	args = append(args, c.Image,
		"claude", "-p", // текст задачи — через stdin
		"--output-format", "stream-json", "--verbose",
		"--model", c.Model,
		"--max-turns", strconv.Itoa(maxTurns),
		"--permission-mode", permissionMode(req),
		"--allowedTools", strings.Join(tools, ","))
	if len(req.MCPConfig) > 0 {
		// Только серверы профиля задачи; настройки MCP из других мест не подхватываются.
		args = append(args, "--mcp-config", string(req.MCPConfig), "--strict-mcp-config")
	}
	prompt := BuildPrompt(req)
	if req.Raw {
		prompt = req.Prompt
	}
	var lines []string
	res, err := c.R.Run(ctx, runner.Command{
		Exe: "docker", Args: args, Timeout: req.Timeout, Stdin: prompt,
		Env:    env,
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

func permissionMode(req Request) string {
	if req.ReadOnly {
		return "default" // правки запрещены: инструменты только для чтения, worktree смонтирован :ro
	}
	return "acceptEdits"
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
