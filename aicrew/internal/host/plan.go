package host

import (
	"context"
	"errors"
	"log/slog"
	"os"

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
	dir, err := os.MkdirTemp("", "aicrew-skills-")
	if err != nil {
		return nil, cleanup, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
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
