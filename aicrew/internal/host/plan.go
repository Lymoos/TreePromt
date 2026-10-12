package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/manager"
	"prompttree/aicrew/internal/mcp"
)

// Planner — Manager (manager.Manager; в тестах — имитация).
type Planner interface {
	Plan(ctx context.Context, t *api.Task, workdir string, c manager.Catalog) (*api.Proposal, agent.Usage, error)
}

// plan — работа вида plan: Manager смотрит на integration (только чтение) и предлагает план.
// Сервер проверяет план и в режиме manual ждёт «Утвердить план».
func (r *run) plan(log *slog.Logger) {
	t, h := r.t, r.h
	ctx := context.WithoutCancel(r.ctx)
	submit := func(p *api.Proposal, failure string) {
		status, reason, err := h.API.SubmitPlan(ctx, t.ID, h.WorkerID, t.Attempt, p, failure)
		log.Info("plan submitted", "status", status, "reason", reason, "err", err)
	}
	if h.Planner == nil {
		submit(nil, "this host has no Manager")
		return
	}
	repo := t.Repository
	if err := h.Git.EnsureBranch(r.ctx, repo.LocalPath, repo.Integration(), repo.DefaultBranch); err != nil {
		submit(nil, "integration branch: "+err.Error())
		return
	}
	head, err := h.Git.RevParse(r.ctx, repo.LocalPath, repo.Integration())
	if err != nil {
		submit(nil, "integration branch: "+err.Error())
		return
	}
	name := "plan-" + t.ID
	path, err := h.Git.AddTempWorktree(r.ctx, repo.LocalPath, name, head)
	if err != nil {
		submit(nil, "worktree: "+err.Error())
		return
	}
	defer h.Git.RemoveTempWorktree(ctx, repo.LocalPath, name)

	p, usage, err := h.Planner.Plan(r.ctx, t, path, h.Catalog)
	log.Info("manager finished", "turns", usage.Turns, "cost_usd", usage.CostUSD)
	if err != nil {
		submit(nil, err.Error())
		return
	}
	submit(p, "")
}

// attachSkills копирует выбранные скиллы во временный каталог и монтирует его исполнителю только для чтения.
func (r *run) attachSkills(req *agent.Request) (attached any, cleanup func(), err error) {
	cleanup = func() {}
	if len(r.t.RequiredSkills) == 0 {
		return []any{}, cleanup, nil
	}
	if r.h.Skills == nil {
		return nil, cleanup, errors.New("this host has no skill registry")
	}
	root := "" // временный каталог службы
	if r.t.ExecutionEnv == "windows_user" && r.h.Windows != nil {
		root = filepath.Join(r.h.Windows.WorkRoot, ".runs") // ai_worker читает только рабочую папку
	}
	base, err := os.MkdirTemp(root, "aicrew-skills-")
	if err != nil {
		return nil, cleanup, err
	}
	cleanup = func() { _ = os.RemoveAll(base) }
	dir := filepath.Join(base, "skills") // родитель — CLAUDE_CONFIG_DIR для windows_user
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return nil, cleanup, err
	}
	att, err := r.h.Skills.Materialize(r.t.RequiredSkills, dir)
	if err != nil {
		return nil, cleanup, err
	}
	req.Mounts = append(req.Mounts, agent.Mount{Host: dir, Container: agent.SkillsMount, ReadOnly: true})
	return att, cleanup, nil
}

// attachMCP подключает исполнителю только серверы MCP-профиля задачи.
func (r *run) attachMCP(req *agent.Request) (mcp.Resolved, error) {
	profile := r.t.MCPProfile
	if profile == "" {
		profile = "default"
	}
	reg := r.h.MCP
	if reg == nil {
		var err error
		if reg, err = mcp.Load(""); err != nil {
			return mcp.Resolved{}, err
		}
	}
	env := r.t.ExecutionEnv
	if env == "" {
		env = "container"
	}
	res, err := reg.Resolve(profile, env)
	if err != nil {
		return res, err
	}
	req.MCPConfig, req.ExtraTools = res.ConfigJSON, res.AllowedTools
	return res, nil
}

// WindowsEnv — окружение windows_user: исполнитель и проверки от ai_worker в рабочей папке.
type WindowsEnv struct {
	WorkRoot string
	Agent    agent.Provider
	Verifier Verifier
}

// Contains — путь внутри рабочей папки (только туда у ai_worker есть доступ).
func (w *WindowsEnv) Contains(p string) bool {
	root, err1 := filepath.Abs(w.WorkRoot)
	abs, err2 := filepath.Abs(p)
	if err1 != nil || err2 != nil {
		return false
	}
	root = strings.ToLower(filepath.Clean(root)) + string(filepath.Separator)
	return strings.HasPrefix(strings.ToLower(filepath.Clean(abs))+string(filepath.Separator), root)
}

// secretCheck — изменения исполнителя до коммита: секрет — задача сразу к человеку (7.6).
func (r *run) secretCheck(diff string) error {
	if r.h.Scanner == nil {
		return nil
	}
	found := r.h.Scanner.ScanDiff(diff)
	if len(found) == 0 {
		return nil
	}
	var where []string
	for _, f := range found {
		where = append(where, fmt.Sprintf("%s:%d (%s)", f.File, f.Line, f.Kind))
	}
	return &failureWithDetails{
		Failure: agent.Failure{Class: "SECRET_LEAK", Code: "secret_in_changes",
			Message: "the agent's changes contain secrets, nothing was committed: " + strings.Join(where, ", ")},
		details: map[string]any{"secrets": found}}
}

// redactFacts — факты и причины уходят на сервер без секретов (значения заменяются на [REDACTED:вид]).
func (h *Host) redactFacts(f api.Facts) api.Facts {
	if h.Scanner == nil {
		return f
	}
	f.Reason = h.Scanner.Redact(f.Reason)
	if f.Details != nil {
		b, err := json.Marshal(f.Details)
		if err == nil {
			red := h.Scanner.Redact(string(b))
			if red != string(b) {
				var v any
				if json.Unmarshal([]byte(red), &v) == nil {
					f.Details = v
				}
			}
		}
	}
	if f.Review != nil {
		v := *f.Review
		v.Summary = h.Scanner.Redact(v.Summary)
		f.Review = &v
	}
	return f
}
