// Package agent — абстракция AgentProvider (ТЗ п. 15): ClaudeCode / ClaudeAPI / Ollama / Gemini.
// Замена провайдера не должна ломать оркестратор.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Request — что агент получает. Workdir — worktree задачи; ничего другого агент не видит.
type Request struct {
	TaskID   string
	Attempt  int
	Role     string // tech_lead, junior…
	Prompt   string
	Workdir  string
	Timeout  time.Duration
	MaxTurns int
	// Previous — факты прошлых попыток (класс ошибки, хвост вывода тестов), чтобы не повторять ошибку.
	Previous []string
	// Checks — команды профиля проверки: исполнитель прогоняет их сам до завершения.
	Checks []string
	// Raw — Prompt передаётся как есть, без правил исполнителя (ревьюеры).
	Raw bool
	// ReadOnly — worktree монтируется только для чтения (ревьюеры).
	ReadOnly bool
	// Name — имя контейнера; пусто — aicrew-task-<id>-<attempt>.
	Name string
}

type Usage struct {
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	Turns        int     `json:"turns"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
}

// Result — итог агента. Summary — текст модели: только дополнительный контекст, не истина (ТЗ п. 9).
type Result struct {
	Summary  string `json:"summary"`
	Usage    Usage  `json:"usage"`
	ExitCode int    `json:"exit_code"`
}

// Failure — ошибка с классом из ТЗ п. 8.4.
type Failure struct {
	Class      string
	Code       string
	Message    string
	RetryAfter time.Duration
}

func (f *Failure) Error() string { return f.Class + ": " + f.Message }

type Provider interface {
	Name() string
	Run(ctx context.Context, req Request) (Result, error)
	// Stop прерывает запущенного агента задачи (отмена пользователем).
	Stop(ctx context.Context, taskID string, attempt int) error
}

// BuildPrompt — задача для исполнителя: текст из PromptTree плюс правила среды и уроки прошлых попыток.
func BuildPrompt(req Request) string {
	var b strings.Builder
	b.WriteString(req.Prompt)
	b.WriteString("\n\n---\nRules of this environment:\n")
	b.WriteString("- You work inside a disposable sandbox. The repository is in the current directory; nothing else is available.\n")
	b.WriteString("- Do not run git commands: the orchestrator commits your changes and collects the diff itself.\n")
	b.WriteString("- Do not add dependencies or change the database schema unless the task explicitly asks for it.\n")
	if len(req.Checks) > 0 {
		b.WriteString("- Before finishing, make sure these checks pass (the orchestrator runs exactly them):\n")
		for _, c := range req.Checks {
			b.WriteString("    " + c + "\n")
		}
		b.WriteString("- Finish with a short summary of what you changed.\n")
	} else {
		b.WriteString("- Run the project's tests before finishing. Finish with a short summary of what you changed.\n")
	}
	if len(req.Previous) > 0 {
		b.WriteString("\nPrevious attempts of this task failed. Do not repeat these mistakes:\n")
		for _, p := range req.Previous {
			b.WriteString("- " + p + "\n")
		}
	}
	return b.String()
}

// streamEvent — строка stream-json Claude Code; нужны только итоговые поля.
type streamEvent struct {
	Type         string  `json:"type"`
	Subtype      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	Result       string  `json:"result"`
	NumTurns     int     `json:"num_turns"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}

// ParseClaudeStream разбирает вывод `claude -p --output-format stream-json`.
// Итог — последнее событие type=result.
func ParseClaudeStream(lines []string) (Result, *Failure) {
	var last *streamEvent
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "{") {
			continue
		}
		var e streamEvent
		if json.Unmarshal([]byte(l), &e) == nil && e.Type == "result" {
			ev := e
			last = &ev
		}
	}
	if last == nil {
		return Result{}, &Failure{Class: "AGENT_ERROR", Code: "no_result", Message: "agent finished without a result event"}
	}
	res := Result{Summary: last.Result, Usage: Usage{InputTokens: last.Usage.InputTokens, OutputTokens: last.Usage.OutputTokens,
		Turns: last.NumTurns, CostUSD: last.TotalCostUSD}}
	if last.IsError || (last.Subtype != "" && last.Subtype != "success") {
		return res, classifyAgentError(last.Subtype, last.Result)
	}
	return res, nil
}

// classifyAgentError — грубая классификация по тексту ошибки CLI; неизвестное — AGENT_ERROR.
func classifyAgentError(subtype, msg string) *Failure {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "rate limit") || strings.Contains(low, "usage limit") || strings.Contains(low, "limit reached") ||
		strings.Contains(low, "overloaded"):
		return &Failure{Class: "RATE_LIMIT", Code: "provider_limit", Message: msg}
	case strings.Contains(low, "invalid api key") || strings.Contains(low, "authentication") || strings.Contains(low, "unauthorized") ||
		strings.Contains(low, "oauth") || strings.Contains(low, "/login"):
		return &Failure{Class: "AUTH_ERROR", Code: "provider_auth", Message: msg}
	case subtype == "error_max_turns":
		return &Failure{Class: "AGENT_ERROR", Code: "max_turns", Message: "agent hit the turn limit"}
	}
	return &Failure{Class: "AGENT_ERROR", Code: nonEmpty(subtype, "agent_error"), Message: msg}
}

func nonEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func ContainerName(taskID string, attempt int) string {
	return fmt.Sprintf("aicrew-task-%s-%d", taskID, attempt)
}
