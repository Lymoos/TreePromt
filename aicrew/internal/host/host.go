// Package host — основной цикл aicrew-host (docs/stage7-2-state-machine.md, п. 5):
// захват задачи с lease, heartbeat, worktree, агент, автокоммит, проверка, отчёт серверу.
// Статусы меняет сервер; хост только просит переходы и сообщает факты, собранные кодом.
package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/gitx"
	"prompttree/aicrew/internal/verify"
)

// Server — то, что хосту нужно от сервера (api.Client; в тестах — имитация).
type Server interface {
	Claim(ctx context.Context, workerID string) (*api.Task, error)
	Heartbeat(ctx context.Context, taskID, workerID string, attempt int) (bool, error)
	Transition(ctx context.Context, taskID, workerID string, attempt int, to string, f api.Facts) (string, error)
	KeepWorktrees(ctx context.Context) (map[string]bool, error)
}

type Verifier interface {
	Run(ctx context.Context, p verify.Profile, workdir, name string, timeout time.Duration) (verify.Report, error)
}

type Host struct {
	API            Server
	Git            gitx.Git
	Agent          agent.Provider
	Verifier       Verifier
	WorkerID       string
	Log            *slog.Logger
	PollEvery      time.Duration
	HeartbeatEvery time.Duration
	// Wake — сигнал «появилась работа» (WebSocket); можно не задавать.
	Wake <-chan struct{}
}

// Run — цикл: брать задачи по одной (решение 7.1: одна задача за раз), пока не отменят контекст.
func (h *Host) Run(ctx context.Context) error {
	for {
		t, err := h.API.Claim(ctx, h.WorkerID)
		if errors.Is(err, api.ErrAuthRequired) {
			return err
		}
		if err != nil {
			h.Log.Warn("claim failed", "err", err)
		}
		if t != nil {
			h.Process(ctx, t)
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(h.PollEvery):
		case <-h.Wake:
		}
	}
}

// CleanupOrphans удаляет worktree задач, которые этому хосту больше не принадлежат (ТЗ п. 8.2).
func (h *Host) CleanupOrphans(ctx context.Context, repos []string) (removed []string, err error) {
	keep, err := h.API.KeepWorktrees(ctx)
	if err != nil {
		return nil, err
	}
	for _, repo := range repos {
		for _, id := range gitx.ListWorktreeTasks(repo) {
			if !keep[id] {
				if err := h.Git.RemoveWorktree(ctx, repo, id); err != nil {
					return removed, err
				}
				removed = append(removed, id)
			}
		}
	}
	return removed, nil
}

// errStopped — работа прервана: отмена пользователем или потеря lease.
var errStopped = errors.New("stopped")

type run struct {
	h        *Host
	t        *api.Task
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	userStop bool // пользователь отменил
	lost     bool // lease потерян (задачу уже вернули в очередь)
}

func (r *run) stopped() (user, lost bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.userStop, r.lost
}

func (r *run) move(to string, f api.Facts) error {
	_, err := r.h.API.Transition(context.WithoutCancel(r.ctx), r.t.ID, r.h.WorkerID, r.t.Attempt, to, f)
	if errors.Is(err, api.ErrNotOwner) {
		r.mu.Lock()
		r.lost = true
		r.mu.Unlock()
		r.cancel()
		return errStopped
	}
	return err
}

func (h *Host) Process(parent context.Context, t *api.Task) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	r := &run{h: h, t: t, ctx: ctx, cancel: cancel}
	log := h.Log.With("task", t.ID, "attempt", t.Attempt)
	log.Info("task claimed", "title", t.Title)

	// Heartbeat: продлевает lease и приносит отмену.
	hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)
		tick := time.NewTicker(h.HeartbeatEvery)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				cancelReq, err := h.API.Heartbeat(context.WithoutCancel(ctx), t.ID, h.WorkerID, t.Attempt)
				switch {
				case errors.Is(err, api.ErrNotOwner):
					r.mu.Lock()
					r.lost = true
					r.mu.Unlock()
					log.Warn("lease lost, stopping")
					cancel()
					return
				case err != nil:
					log.Warn("heartbeat failed", "err", err) // сеть мигнула — lease ещё действует
				case cancelReq:
					r.mu.Lock()
					r.userStop = true
					r.mu.Unlock()
					log.Info("cancellation requested")
					cancel()
					return
				}
			}
		}
	}()

	err := r.pipeline()
	cancel()
	<-hbDone
	r.finish(err, log)
}

func (r *run) pipeline() error {
	t, h := r.t, r.h
	repo := t.Repository
	path, base, err := h.Git.AddWorktree(r.ctx, repo.LocalPath, t.ID, repo.DefaultBranch)
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "worktree_failed", Message: err.Error()}
	}
	if err := r.move("IN_PROGRESS", api.Facts{BaseCommit: base}); err != nil {
		return err
	}

	res, err := h.Agent.Run(r.ctx, agent.Request{
		TaskID: t.ID, Attempt: t.Attempt, Role: "tech_lead", Prompt: t.Prompt, Workdir: path,
		Timeout: time.Duration(t.AgentRuntimeSec) * time.Second, Previous: lessons(t.PreviousAttempts),
	})
	if err != nil {
		return err
	}
	if user, lost := r.stopped(); user || lost {
		return errStopped
	}

	facts, err := h.Git.Autocommit(context.WithoutCancel(r.ctx), path, base, fmt.Sprintf("aicrew: %s (task %s, attempt %d)", t.Title, t.ID, t.Attempt))
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "autocommit_failed", Message: err.Error()}
	}
	if facts.NoChanges {
		return &agent.Failure{Class: "AGENT_ERROR", Code: "no_changes", Message: "agent finished without changing any file"}
	}
	if err := r.move("VERIFYING", api.Facts{ResultCommit: facts.ResultCommit, Details: map[string]any{
		"changed_files": facts.ChangedFiles, "diff_stat": facts.DiffStat, "agent": res}}); err != nil {
		return err
	}

	profile, err := verify.ParseProfile(repo.VerificationProfile)
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "bad_profile", Message: err.Error()}
	}
	rep, err := h.Verifier.Run(r.ctx, profile, path, fmt.Sprintf("aicrew-verify-%s-%d", t.ID, t.Attempt),
		time.Duration(t.VerifyRuntimeSec)*time.Second)
	if user, lost := r.stopped(); user || lost {
		return errStopped
	}
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "verify_failed_to_run", Message: err.Error()}
	}
	if !rep.Passed {
		class := "BUILD_FAILURE"
		switch {
		case rep.TimedOut:
			class = "TIMEOUT"
		case rep.FailedKind == "test":
			class = "TEST_FAILURE"
		}
		return &failureWithDetails{Failure: agent.Failure{Class: class, Code: "verification_failed", Message: lastStep(rep)}, details: rep}
	}
	if err := r.move("LLM_REVIEW", api.Facts{Details: map[string]any{"verification": rep}}); err != nil {
		return err
	}
	// Logic QA / Tech Lead — этап 7.4; до тех пор ревью пропускается с пометкой.
	return r.move("MERGEABLE", api.Facts{Reason: "LLM review is not enabled yet (stage 7.4)"})
}

type failureWithDetails struct {
	agent.Failure
	details any
}

func (r *run) finish(err error, log *slog.Logger) {
	t, h := r.t, r.h
	ctx := context.WithoutCancel(r.ctx)
	user, lost := r.stopped()
	switch {
	case err == nil:
		log.Info("task ready to merge")
		return // worktree остаётся до слияния (этап 7.3)
	case lost:
		log.Warn("task taken away from this worker")
	case user:
		_ = h.Agent.Stop(ctx, t.ID, t.Attempt)
		if _, err := h.API.Transition(ctx, t.ID, h.WorkerID, t.Attempt, "CANCELLED", api.Facts{Reason: "stopped by user"}); err != nil {
			log.Warn("confirm cancellation", "err", err)
		}
		log.Info("task cancelled")
	default:
		var fd *failureWithDetails
		var fa *agent.Failure
		f := api.Facts{FailureClass: "AGENT_ERROR", FailureCode: "unexpected", Reason: err.Error()}
		switch {
		case errors.As(err, &fd):
			f = api.Facts{FailureClass: fd.Class, FailureCode: fd.Code, Reason: fd.Message, Details: map[string]any{"verification": fd.details}}
		case errors.As(err, &fa):
			f = api.Facts{FailureClass: fa.Class, FailureCode: fa.Code, Reason: fa.Message, RetryAfterSec: int(fa.RetryAfter.Seconds())}
		}
		status, terr := h.API.Transition(ctx, t.ID, h.WorkerID, t.Attempt, "FAILED", f)
		log.Warn("task failed", "class", f.FailureClass, "code", f.FailureCode, "next", status, "err", terr)
	}
	// Провал или отмена до слияния — worktree удаляется (ТЗ п. 8.5).
	if rerr := h.Git.RemoveWorktree(ctx, t.Repository.LocalPath, t.ID); rerr != nil {
		log.Warn("remove worktree", "err", rerr)
	}
}

// lessons — короткие выводы из прошлых попыток для промпта следующей.
func lessons(prev []api.PreviousAttempt) []string {
	var out []string
	for _, p := range prev {
		if p.FailureClass == nil {
			continue
		}
		line := fmt.Sprintf("attempt %d failed with %s", p.Attempt, *p.FailureClass)
		if p.FailureCode != nil {
			line += " (" + *p.FailureCode + ")"
		}
		var facts struct {
			Verification *verify.Report `json:"verification"`
		}
		if json.Unmarshal(p.Facts, &facts) == nil && facts.Verification != nil {
			if s := lastStep(*facts.Verification); s != "" {
				line += ": " + s
			}
		}
		out = append(out, line)
	}
	return out
}

func lastStep(rep verify.Report) string {
	if len(rep.Steps) == 0 {
		return ""
	}
	s := rep.Steps[len(rep.Steps)-1]
	out := strings.TrimSpace(s.Output)
	if len(out) > 1500 {
		out = "…" + out[len(out)-1500:]
	}
	return fmt.Sprintf("step %q exited with %d:\n%s", s.Name, s.ExitCode, out)
}
