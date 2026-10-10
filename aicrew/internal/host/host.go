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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/contract"
	"prompttree/aicrew/internal/gitx"
	"prompttree/aicrew/internal/review"
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
	Reviewer       review.Reviewer
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
		// Временные worktree очереди слияния после перезапуска не нужны никогда.
		for _, name := range gitx.ListTempWorktrees(repo) {
			h.Git.RemoveTempWorktree(ctx, repo, name)
		}
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
	kind := t.Kind
	if kind == "" {
		kind = api.KindExecute
	}
	log := h.Log.With("task", t.ID, "attempt", t.Attempt, "kind", kind)
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

	var err error
	switch kind {
	case api.KindMerge:
		err = r.merge()
	case api.KindRollback:
		err = r.rollback()
	default:
		err = r.pipeline()
	}
	cancel()
	<-hbDone
	r.finish(kind, err, log)
}

func (r *run) pipeline() error {
	t, h := r.t, r.h
	repo := t.Repository
	// Задача начинается от integration: следующая видит результат предыдущей (7.3).
	if err := h.Git.EnsureBranch(r.ctx, repo.LocalPath, repo.Integration(), repo.DefaultBranch); err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "integration_branch_failed", Message: err.Error()}
	}
	path, base, err := h.Git.AddWorktree(r.ctx, repo.LocalPath, t.ID, repo.Integration())
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "worktree_failed", Message: err.Error()}
	}
	if err := r.move("IN_PROGRESS", api.Facts{BaseCommit: base}); err != nil {
		return err
	}
	parsed, err := verify.ParseProfile(repo.VerificationProfile)
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "bad_profile", Message: err.Error()}
	}
	profile := parsed.ForStage(verify.StageAttempt)

	res, err := h.Agent.Run(r.ctx, agent.Request{
		TaskID: t.ID, Attempt: t.Attempt, Role: "tech_lead", Prompt: t.Prompt, Workdir: path,
		Timeout: time.Duration(t.AgentRuntimeSec) * time.Second, Previous: lessons(t.PreviousAttempts),
		Checks: profile.Commands(),
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
		return &failureWithDetails{Failure: agent.Failure{Class: class, Code: "verification_failed", Message: lastStep(rep)},
			details: map[string]any{"verification": rep}}
	}
	// Шаг 2: контракт архитектуры — код, а не LLM (ТЗ п. 9.2).
	con, err := r.checkContract(path, base, facts.ChangedFiles)
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "bad_contract", Message: err.Error()}
	}
	if len(con.Violations) > 0 {
		return &failureWithDetails{
			Failure: agent.Failure{Class: "QA_REJECT", Code: "contract_violation", Message: "architecture contract violated:\n" + con.String()},
			details: map[string]any{"contract": con, "rejection_type": "contract"}}
	}
	if err := r.move("LLM_REVIEW", api.Facts{Details: map[string]any{"verification": rep, "contract": con}}); err != nil {
		return err
	}

	// Шаги 3–4: Logic QA и, по пометке, Tech Lead. Модель предлагает вердикт, статус двигает сервер.
	diff, err := h.Git.Diff(r.ctx, path, base, facts.ResultCommit)
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "diff_failed", Message: err.Error()}
	}
	rf := review.Facts{Title: t.Title, Task: t.Prompt, Diff: diff, DiffStat: facts.DiffStat, ChangedFiles: facts.ChangedFiles,
		Tests: rep.Tests(), Checks: rep.Steps, Contract: &con, AgentSummary: res.Summary}
	roles := []string{review.RoleLogicQA}
	if t.ReviewLevel == "tech_lead" {
		roles = append(roles, review.RoleTechLead)
	}
	var verdicts []api.Verdict
	for _, role := range roles {
		v, err := r.review(role, path, rf)
		if err != nil {
			return err
		}
		verdicts = append(verdicts, v)
	}
	final := verdicts[len(verdicts)-1]
	return r.move("MERGEABLE", api.Facts{Review: &final, ApprovalRequired: con.ApprovalRequired,
		Details: map[string]any{"reviews": verdicts}})
}

// checkContract проверяет изменённые файлы по architecture.yaml из base_commit.
func (r *run) checkContract(path, base string, changed []string) (contract.Result, error) {
	raw, ok := r.h.Git.ShowFile(r.ctx, path, base, contract.FileName)
	if !ok {
		return contract.Result{}, nil // контракта нет — шаг пропускается, отметка в фактах (present: false)
	}
	c, err := contract.Parse(raw)
	if err != nil {
		return contract.Result{}, err
	}
	read := func(p string) ([]byte, bool) {
		b, err := os.ReadFile(filepath.Join(path, filepath.FromSlash(p)))
		return b, err == nil
	}
	return c.Check(changed, read, contract.DetectProject(read)), nil
}

// review — один ревьюер. Отказ — FAILED/QA_REJECT с замечаниями для следующей попытки.
func (r *run) review(role, path string, f review.Facts) (api.Verdict, error) {
	v, err := r.h.Reviewer.Review(r.ctx, role, r.t, path, f)
	if user, lost := r.stopped(); user || lost {
		return v, errStopped
	}
	var fa *agent.Failure
	switch {
	case errors.As(err, &fa):
		return v, err
	case errors.Is(err, api.ErrNotOwner):
		r.mu.Lock()
		r.lost = true
		r.mu.Unlock()
		return v, errStopped
	case err != nil:
		return v, &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: role + "_failed", Message: err.Error()}
	}
	if err := review.Validate(&v); err != nil {
		return v, &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: role + "_invalid", Message: err.Error()}
	}
	if v.Verdict != "approve" {
		return v, &failureWithDetails{
			Failure: agent.Failure{Class: "QA_REJECT", Code: role + "_rejected", Message: v.Summary + "\n" + review.Issues(v)},
			details: map[string]any{"review": v, "rejection_type": v.RejectionType}}
	}
	return v, nil
}

// ── очередь слияния (этап 7.3, docs/stage7-3-merge-queue.md) ──

// merge сливает ветку задачи в integration во временном worktree, проверяет результат
// слияния и только потом двигает integration. Иначе integration не меняется.
func (r *run) merge() error {
	t, h := r.t, r.h
	repo := t.Repository
	integ := repo.Integration()
	branch := gitx.Branch(t.ID)
	if err := h.Git.EnsureBranch(r.ctx, repo.LocalPath, integ, repo.DefaultBranch); err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "integration_branch_failed", Message: err.Error()}
	}
	old, err := h.Git.RevParse(r.ctx, repo.LocalPath, integ)
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "integration_branch_failed", Message: err.Error()}
	}
	if _, err := h.Git.RevParse(r.ctx, repo.LocalPath, branch); err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "task_branch_missing", Message: err.Error()}
	}
	// Прошлый запуск успел сдвинуть integration, но не доложил (ПК выключили): просто досообщаем.
	if h.Git.IsAncestor(r.ctx, repo.LocalPath, branch, integ) {
		if err := r.move("MERGED", api.Facts{MergeCommit: old, Reason: "integration already contains the task"}); err != nil {
			return err
		}
		return r.done()
	}

	name := "merge-" + t.ID
	path, err := h.Git.AddTempWorktree(r.ctx, repo.LocalPath, name, old)
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "worktree_failed", Message: err.Error()}
	}
	defer h.Git.RemoveTempWorktree(context.WithoutCancel(r.ctx), repo.LocalPath, name)

	commit, conflicts, err := h.Git.Merge(r.ctx, path, branch, fmt.Sprintf("aicrew: merge %s (task %s)", t.Title, t.ID))
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "merge_failed", Message: err.Error()}
	}
	if len(conflicts) > 0 {
		return &failureWithDetails{
			Failure: agent.Failure{Class: "MERGE_CONFLICT", Code: "merge_conflict",
				Message: "conflicts with changes merged meanwhile in: " + strings.Join(conflicts, ", ")},
			details: map[string]any{"conflicted_files": conflicts, "integration": old}}
	}
	if err := r.move("POST_MERGE_VERIFY", api.Facts{Details: map[string]any{"merge_candidate": commit, "integration": old}}); err != nil {
		return err
	}
	if err := r.verifyMerged(path, "merge"); err != nil {
		return err
	}
	// Integration сдвигается только если её никто не тронул за время проверки.
	if err := h.Git.AdvanceBranch(context.WithoutCancel(r.ctx), repo.LocalPath, integ, commit, old); err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "integration_moved", Message: err.Error()}
	}
	if err := r.move("MERGED", api.Facts{MergeCommit: commit}); err != nil {
		return err
	}
	return r.done()
}

// done завершает слитую задачу: worktree и ветка задачи больше не нужны (история — в коммите слияния).
func (r *run) done() error {
	if err := r.h.Git.RemoveWorktree(context.WithoutCancel(r.ctx), r.t.Repository.LocalPath, r.t.ID); err != nil {
		r.h.Log.Warn("remove worktree", "task", r.t.ID, "err", err)
	}
	return r.move("DONE", api.Facts{})
}

// rollback откатывает коммит слияния задачи (git revert -m 1) с той же проверкой.
func (r *run) rollback() error {
	t, h := r.t, r.h
	repo := t.Repository
	integ := repo.Integration()
	if t.MergeCommit == "" {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "no_merge_commit", Message: "task has no merge commit"}
	}
	old, err := h.Git.RevParse(r.ctx, repo.LocalPath, integ)
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "integration_branch_failed", Message: err.Error()}
	}
	name := "rollback-" + t.ID
	path, err := h.Git.AddTempWorktree(r.ctx, repo.LocalPath, name, old)
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "worktree_failed", Message: err.Error()}
	}
	defer h.Git.RemoveTempWorktree(context.WithoutCancel(r.ctx), repo.LocalPath, name)

	commit, conflicts, err := h.Git.Revert(r.ctx, path, t.MergeCommit, fmt.Sprintf("aicrew: rollback %s (task %s)", t.Title, t.ID))
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "revert_failed", Message: err.Error()}
	}
	if len(conflicts) > 0 {
		return &failureWithDetails{
			Failure: agent.Failure{Class: "MERGE_CONFLICT", Code: "revert_conflict",
				Message: "later changes touch the same lines: " + strings.Join(conflicts, ", ")},
			details: map[string]any{"conflicted_files": conflicts}}
	}
	if err := r.verifyMerged(path, "rollback"); err != nil {
		return err
	}
	if err := h.Git.AdvanceBranch(context.WithoutCancel(r.ctx), repo.LocalPath, integ, commit, old); err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "integration_moved", Message: err.Error()}
	}
	return r.move("ROLLED_BACK", api.Facts{RevertCommit: commit})
}

// verifyMerged гоняет профиль проверки на результате слияния или отката.
func (r *run) verifyMerged(path, stage string) error {
	t, h := r.t, r.h
	profile, err := verify.ParseProfile(t.Repository.VerificationProfile)
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "bad_profile", Message: err.Error()}
	}
	rep, err := h.Verifier.Run(r.ctx, profile.ForStage(verify.StageMerge), path, fmt.Sprintf("aicrew-verify-%s-%s-%d", stage, t.ID, t.Attempt),
		time.Duration(t.VerifyRuntimeSec)*time.Second)
	if user, lost := r.stopped(); user || lost {
		return errStopped
	}
	if err != nil {
		return &agent.Failure{Class: "ENVIRONMENT_FAILURE", Code: "verify_failed_to_run", Message: err.Error()}
	}
	if !rep.Passed {
		class := "INTEGRATION_FAILURE"
		if rep.TimedOut {
			class = "TIMEOUT"
		}
		return &failureWithDetails{
			Failure: agent.Failure{Class: class, Code: "post_" + stage + "_verification_failed",
				Message: "together with changes merged meanwhile: " + lastStep(rep)},
			details: map[string]any{"verification": rep, "stage": "post_" + stage}}
	}
	return nil
}

type failureWithDetails struct {
	agent.Failure
	details map[string]any
}

func (r *run) finish(kind string, err error, log *slog.Logger) {
	t, h := r.t, r.h
	ctx := context.WithoutCancel(r.ctx)
	user, lost := r.stopped()
	switch {
	case err == nil && kind == api.KindExecute:
		log.Info("task ready to merge")
		return // worktree остаётся до слияния
	case err == nil:
		log.Info("merge queue job done")
		return
	case lost && kind != api.KindExecute:
		// Сервер вернёт слияние или откат в очередь; integration не сдвинута, worktree задачи нужен.
		log.Warn("merge queue job taken away from this worker")
		return
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
			f = api.Facts{FailureClass: fd.Class, FailureCode: fd.Code, Reason: fd.Message, Details: fd.details}
		case errors.As(err, &fa):
			f = api.Facts{FailureClass: fa.Class, FailureCode: fa.Code, Reason: fa.Message, RetryAfterSec: int(fa.RetryAfter.Seconds())}
		}
		status, terr := h.API.Transition(ctx, t.ID, h.WorkerID, t.Attempt, "FAILED", f)
		log.Warn("task failed", "class", f.FailureClass, "code", f.FailureCode, "next", status, "err", terr)
	}
	if kind == api.KindRollback {
		return // откат не удался — задача остаётся слитой, её ветки уже нет
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
			Verification *verify.Report   `json:"verification"`
			Conflicts    []string         `json:"conflicted_files"`
			Stage        string           `json:"stage"`
			Review       *api.Verdict     `json:"review"`
			Contract     *contract.Result `json:"contract"`
		}
		if json.Unmarshal(p.Facts, &facts) == nil {
			if len(facts.Conflicts) > 0 {
				line += ": your result conflicted with other tasks merged meanwhile in " + strings.Join(facts.Conflicts, ", ") +
					"; the worktree now starts from the updated code — redo the task on top of it and keep their changes"
			}
			if facts.Stage == "post_merge" {
				line += ": checks failed only after merging with other tasks' changes — make the task work together with them"
			}
			if facts.Contract != nil && len(facts.Contract.Violations) > 0 {
				line += ": the architecture contract (architecture.yaml) forbids:\n" + facts.Contract.String()
			}
			if facts.Review != nil && facts.Review.Verdict == "reject" {
				line += ": the reviewer rejected the change — " + facts.Review.Summary + "\n" + review.Issues(*facts.Review)
			}
			if facts.Verification != nil && facts.Review == nil {
				if s := lastStep(*facts.Verification); s != "" {
					line += ": " + s
				}
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
