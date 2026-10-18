package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"prompttree/aicrew/internal/runner"
)

// CodexAuth — где служба хранит вход Codex по подписке ChatGPT (содержимое ~/.codex/auth.json, DPAPI).
type CodexAuth interface {
	Get() (string, error)
	Set(string) error
}

// Codex — исполнитель OpenAI Codex (этап 7.8, docs/stage7-8-codex.md): родной клиент `codex exec` в том же
// одноразовом контейнере, что и Claude Code. Вход владельца кладётся во временный CODEX_HOME только на время
// попытки; обновлённый Codex'ом файл забирается обратно в хранилище, папка удаляется.
type Codex struct {
	R       runner.Runner
	Image   string
	Model   string // пусто — модель по умолчанию у подписки
	Auth    CodexAuth
	Network string
	// TempDir — где создавать временный CODEX_HOME (пусто — системная временная папка).
	TempDir string
}

func (c *Codex) Name() string { return "codex/" + nonEmpty(c.Model, "default") }

// codexHome — CODEX_HOME внутри контейнера.
const codexHome = "/home/agent/.codex"

func (c *Codex) Run(ctx context.Context, req Request) (Result, error) {
	auth, err := c.Auth.Get()
	if err != nil || strings.TrimSpace(auth) == "" {
		return Result{}, &Failure{Class: "AUTH_ERROR", Code: "no_codex_auth",
			Message: "Codex login is not saved on this host: run `codex login`, then `aicrew-host set-codex-auth`"}
	}
	home, err := os.MkdirTemp(c.TempDir, "aicrew-codex-")
	if err != nil {
		return Result{}, &Failure{Class: "ENVIRONMENT_FAILURE", Code: "codex_home", Message: err.Error()}
	}
	authFile := filepath.Join(home, "auth.json")
	defer func() {
		// Codex продлевает вход сам: свежий файл возвращается в хранилище, временная папка удаляется.
		if b, err := os.ReadFile(authFile); err == nil && string(b) != auth && json.Valid(b) {
			_ = c.Auth.Set(string(b))
		}
		_ = os.RemoveAll(home)
	}()
	if err := os.WriteFile(authFile, []byte(auth), 0o600); err != nil {
		return Result{}, &Failure{Class: "ENVIRONMENT_FAILURE", Code: "codex_home", Message: err.Error()}
	}

	name := req.Name
	if name == "" {
		name = ContainerName(req.TaskID, req.Attempt)
	}
	mount := req.Workdir + ":/work"
	if req.ReadOnly {
		mount += ":ro"
	}
	args := []string{"run", "--rm", "-i", "--name", name, "--memory", "6g", "--cpus", "4", "--pids-limit", "512",
		"-v", mount, "-w", "/work", "-v", home + ":" + codexHome, "-e", "CODEX_HOME=" + codexHome}
	for _, m := range req.Mounts {
		target := m.Container
		if target == SkillsMount {
			target = codexHome + "/skills" // скиллы задачи — там, где их ищет Codex
		}
		v := m.Host + ":" + target
		if m.ReadOnly {
			v += ":ro"
		}
		args = append(args, "-v", v)
	}
	if c.Network != "" {
		args = append(args, "--network", c.Network)
	}
	args = append(args, c.Image, "codex", "exec", "--json", "--skip-git-repo-check", "--ephemeral", "--color", "never")
	if req.ReadOnly {
		args = append(args, "--sandbox", "read-only")
	} else {
		// Песочница — сам одноразовый контейнер (только worktree, лимиты); своя песочница Codex в Docker не нужна.
		args = append(args, "--dangerously-bypass-approvals-and-sandbox")
	}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	args = append(args, "-")

	prompt := BuildPrompt(req)
	if req.Raw {
		prompt = req.Prompt
	}
	var lines []string
	res, err := c.R.Run(ctx, runner.Command{Exe: "docker", Args: args, Timeout: req.Timeout, Stdin: prompt,
		Stdout: func(l string) { lines = append(lines, l) }})
	if err != nil {
		return Result{}, &Failure{Class: "ENVIRONMENT_FAILURE", Code: "docker_failed", Message: err.Error()}
	}
	if res.TimedOut {
		_ = c.Stop(context.WithoutCancel(ctx), req.TaskID, req.Attempt)
		return Result{}, &Failure{Class: "TIMEOUT", Code: "agent_runtime", Message: fmt.Sprintf("agent ran longer than %s", req.Timeout)}
	}
	out, fail := ParseCodexStream(lines)
	out.ExitCode = res.ExitCode
	if fail != nil {
		if fail.Code == "no_result" && res.ExitCode == 125 {
			return out, &Failure{Class: "ENVIRONMENT_FAILURE", Code: "container_start", Message: tail(res.Stderr, 400)}
		}
		if fail.Code == "no_result" {
			fail.Message += ": " + tail(res.Stderr, 400)
		}
		return out, fail
	}
	return out, nil
}

func (c *Codex) Stop(ctx context.Context, taskID string, attempt int) error {
	_, err := c.R.Run(ctx, runner.Command{Exe: "docker", Args: []string{"kill", ContainerName(taskID, attempt)}, Timeout: 30 * time.Second})
	return err
}

// codexEvent — строка `codex exec --json`; нужны итоговый ответ, расход и ошибки.
type codexEvent struct {
	Type string `json:"type"`
	Item struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item"`
	Usage struct {
		InputTokens     int64 `json:"input_tokens"`
		CachedTokens    int64 `json:"cached_input_tokens"`
		OutputTokens    int64 `json:"output_tokens"`
		ReasoningTokens int64 `json:"reasoning_output_tokens"`
	} `json:"usage"`
	Message string `json:"message"`
	Error   struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ParseCodexStream разбирает вывод `codex exec --json`: итог — последнее сообщение агента после
// turn.completed; turn.failed или одни ошибки без завершённого хода — неудача с классом из ТЗ п. 8.4.
func ParseCodexStream(lines []string) (Result, *Failure) {
	var res Result
	completed := false
	var failed, lastError string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "{") {
			continue
		}
		var e codexEvent
		if json.Unmarshal([]byte(l), &e) != nil {
			continue
		}
		switch e.Type {
		case "item.completed":
			if e.Item.Type == "agent_message" && strings.TrimSpace(e.Item.Text) != "" {
				res.Summary = e.Item.Text
			}
		case "turn.completed":
			completed = true
			res.Usage.Turns++
			res.Usage.InputTokens += e.Usage.InputTokens
			res.Usage.OutputTokens += e.Usage.OutputTokens + e.Usage.ReasoningTokens
		case "turn.failed":
			failed = nonEmpty(e.Error.Message, e.Message)
		case "error":
			lastError = e.Message
		}
	}
	switch {
	case failed != "":
		return res, classifyAgentError("codex_turn_failed", failed)
	case !completed && lastError != "":
		return res, classifyAgentError("codex_error", lastError)
	case !completed:
		return res, &Failure{Class: "AGENT_ERROR", Code: "no_result", Message: "codex finished without a completed turn"}
	}
	return res, nil
}
