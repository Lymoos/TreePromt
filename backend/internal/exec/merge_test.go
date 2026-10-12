package exec

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Этап 7.3: очередь слияния, integration-ветка, откат (docs/stage7-3-merge-queue.md).

// toMergeable проводит взятую задачу до MERGEABLE и возвращает итоговый статус.
func (f *fx) toMergeable(c *ClaimedTask) string {
	f.t.Helper()
	f.move(c, StatusInProgress, Facts{BaseCommit: "base"})
	f.move(c, StatusVerifying, Facts{ResultCommit: "result"})
	f.move(c, StatusLLMReview, Facts{})
	return f.move(c, StatusMergeable, Facts{Review: approve()})
}

func (f *fx) mergeable(opts ...func(*NewTask)) uuid.UUID {
	f.t.Helper()
	id := f.task(opts...)
	c := f.claim()
	if c == nil || c.ID != id || c.Kind != KindExecute {
		f.t.Fatalf("claim execute: %+v", c)
	}
	f.toMergeable(c)
	return id
}

// done проводит задачу через слияние до DONE.
func (f *fx) done(id uuid.UUID, merge string) {
	f.t.Helper()
	if err := f.svc.Merge(f.ctx, f.user, id); err != nil {
		f.t.Fatal(err)
	}
	j := f.claim()
	if j == nil || j.ID != id || j.Kind != KindMerge {
		f.t.Fatalf("merge job: %+v", j)
	}
	f.move(j, StatusPostMergeVerify, Facts{})
	f.move(j, StatusMerged, Facts{MergeCommit: merge})
	f.move(j, StatusDone, Facts{})
}

func TestManualTaskWaitsForMergeApproval(t *testing.T) {
	f := newFx(t)
	id := f.mergeable()
	if st, _, _ := f.status(id); st != StatusMergeable {
		t.Fatal(st)
	}
	if f.claim() != nil {
		t.Fatal("manual mode: nothing to merge before the user approves")
	}
	if err := f.svc.Merge(f.ctx, f.user, id); err != nil {
		t.Fatal(err)
	}
	j := f.claim()
	if j == nil || j.Kind != KindMerge || j.ID != id || j.Attempt != 1 || j.Repository.IntegrationBranch != "aicrew/integration" {
		t.Fatalf("merge job: %+v", j)
	}
	if st, _, _ := f.status(id); st != StatusMerging {
		t.Fatal(st)
	}
	f.move(j, StatusPostMergeVerify, Facts{})
	f.move(j, StatusMerged, Facts{MergeCommit: "m1"})
	if st := f.move(j, StatusDone, Facts{}); st != StatusDone {
		t.Fatal(st)
	}
	tasks, _ := f.svc.ListTasks(f.ctx, f.user, nil)
	if len(tasks) != 1 || tasks[0].MergeCommit == nil || *tasks[0].MergeCommit != "m1" {
		t.Fatalf("merge commit must be recorded: %+v", tasks)
	}
	// Повторно «слить» уже слитое нельзя.
	if err := f.svc.Merge(f.ctx, f.user, id); !errors.Is(err, ErrBadTransition) {
		t.Fatalf("merge twice: %v", err)
	}
}

func TestSemiAutoQueuesMergeByItself(t *testing.T) {
	f := newFx(t)
	f.task(func(n *NewTask) { n.ExecutionMode = "semi_auto" })
	c := f.claim()
	if st := f.toMergeable(c); st != StatusMergeQueued {
		t.Fatalf("semi_auto must go straight to the merge queue, got %s", st)
	}
	if j := f.claim(); j == nil || j.Kind != KindMerge {
		t.Fatalf("merge job: %+v", j)
	}
}

func TestMergeQueueGoesBeforeNewWork(t *testing.T) {
	f := newFx(t)
	merged := f.mergeable()
	f.task() // новая задача ждёт: она должна начаться от integration уже с первой
	_ = f.svc.Merge(f.ctx, f.user, merged)
	if j := f.claim(); j == nil || j.ID != merged || j.Kind != KindMerge {
		t.Fatalf("merge must be claimed first: %+v", j)
	}
}

func TestOneMergeAtATimePerRepository(t *testing.T) {
	f := newFx(t)
	a, b := f.mergeable(), f.mergeable()
	_ = f.svc.Merge(f.ctx, f.user, a)
	_ = f.svc.Merge(f.ctx, f.user, b)
	ja := f.claim()
	if ja == nil || ja.ID != a {
		t.Fatalf("first merge: %+v", ja)
	}
	if j := f.claim(); j != nil {
		t.Fatalf("integration is busy, second merge must wait: %+v", j)
	}
	f.move(ja, StatusPostMergeVerify, Facts{})
	if j := f.claim(); j != nil {
		t.Fatal("still verifying the first merge")
	}
	f.move(ja, StatusMerged, Facts{MergeCommit: "ma"})
	f.move(ja, StatusDone, Facts{})
	if jb := f.claim(); jb == nil || jb.ID != b || jb.Kind != KindMerge {
		t.Fatalf("second merge after the first: %+v", jb)
	}
}

// ТЗ 7.1, сценарий 7.3: конфликт слияния → задача переделывается поверх свежего integration.
func TestMergeConflictSendsTaskBackForRework(t *testing.T) {
	f := newFx(t)
	id := f.mergeable()
	_ = f.svc.Merge(f.ctx, f.user, id)
	j := f.claim()
	st := f.move(j, StatusFailed, Facts{FailureClass: ClassMerge, FailureCode: "merge_conflict",
		Details: json.RawMessage(`{"conflicted_files":["lib/main.dart"]}`)})
	if st != StatusQueued {
		t.Fatalf("conflict → rework, got %s", st)
	}
	c := f.claim()
	if c == nil || c.Kind != KindExecute || c.Attempt != 2 || len(c.PreviousAttempts) != 1 ||
		*c.PreviousAttempts[0].FailureClass != ClassMerge || !strings.Contains(string(c.PreviousAttempts[0].Facts), "lib/main.dart") {
		t.Fatalf("rework claim: %+v", c)
	}
}

// ТЗ 7.1, сценарий 7.3: A и B по отдельности зелёные, вместе ломают — проверка после слияния
// ловит это, integration не двигается, B переделывается.
func TestPostMergeVerificationFailureReworks(t *testing.T) {
	f := newFx(t)
	id := f.mergeable()
	_ = f.svc.Merge(f.ctx, f.user, id)
	j := f.claim()
	f.move(j, StatusPostMergeVerify, Facts{})
	if st := f.move(j, StatusFailed, Facts{FailureClass: ClassIntegration, FailureCode: "post_merge_verification_failed"}); st != StatusQueued {
		t.Fatal(st)
	}
	tasks, _ := f.svc.ListTasks(f.ctx, f.user, nil)
	if tasks[0].MergeCommit != nil {
		t.Fatal("failed merge must not record a merge commit")
	}
}

func TestLostLeaseDuringMergeRequeuesTheMerge(t *testing.T) {
	f := newFx(t)
	id := f.mergeable()
	_ = f.svc.Merge(f.ctx, f.user, id)
	f.claim()
	f.clock = f.clock.Add(LeaseTTL + time.Second)
	if n, err := f.svc.ReapExpired(f.ctx); err != nil || n != 1 {
		t.Fatalf("reaped %d %v", n, err)
	}
	st, attempt, _ := f.status(id)
	if st != StatusMergeQueued || attempt != 1 {
		t.Fatalf("lost merge lease must requeue the merge without spending an attempt: %s %d", st, attempt)
	}
	if j := f.claim(); j == nil || j.Kind != KindMerge {
		t.Fatalf("merge again: %+v", j)
	}
}

func TestRollbackGoesThroughTheQueue(t *testing.T) {
	f := newFx(t)
	id := f.mergeable()
	f.done(id, "m1")
	if err := f.svc.Rollback(f.ctx, f.user, id, "сломало экран"); err != nil {
		t.Fatal(err)
	}
	j := f.claim()
	if j == nil || j.Kind != KindRollback || j.MergeCommit != "m1" {
		t.Fatalf("rollback job: %+v", j)
	}
	if st := f.move(j, StatusRolledBack, Facts{RevertCommit: "r1"}); st != StatusRolledBack {
		t.Fatal(st)
	}
	if st, _ := f.svc.Cancel(f.ctx, f.user, id); st != StatusRolledBack {
		t.Fatal("rolled back task is final")
	}
}

func TestFailedRollbackKeepsTaskMerged(t *testing.T) {
	f := newFx(t)
	id := f.mergeable()
	f.done(id, "m1")
	_ = f.svc.Rollback(f.ctx, f.user, id, "")
	j := f.claim()
	if st := f.move(j, StatusFailed, Facts{FailureClass: ClassMerge, FailureCode: "revert_conflict"}); st != StatusDone {
		t.Fatalf("failed rollback must leave the task merged, got %s", st)
	}
	var code string
	_ = f.pool.QueryRow(f.ctx, `SELECT failure_code FROM exec.tasks WHERE id = $1`, id).Scan(&code)
	if code != "rollback_failed" {
		t.Fatal(code)
	}
}

func TestRollbackRefusesWhenDependentsAreMerged(t *testing.T) {
	f := newFx(t)
	base := f.mergeable()
	f.done(base, "m1")
	dep := f.mergeable(func(n *NewTask) { n.Title, n.DependsOn = "Зависимая", []uuid.UUID{base} })
	f.done(dep, "m2")
	err := f.svc.Rollback(f.ctx, f.user, base, "")
	if !errors.Is(err, ErrBadTransition) || !strings.Contains(err.Error(), "Зависимая") {
		t.Fatalf("must name dependent tasks: %v", err)
	}
	if err := f.svc.Rollback(f.ctx, f.user, dep, ""); err != nil {
		t.Fatal(err)
	}
}

func TestRollbackCanBeCancelledBeforeItRuns(t *testing.T) {
	f := newFx(t)
	id := f.mergeable()
	f.done(id, "m1")
	_ = f.svc.Rollback(f.ctx, f.user, id, "")
	if st, err := f.svc.Cancel(f.ctx, f.user, id); err != nil || st != StatusDone {
		t.Fatalf("cancel queued rollback: %s %v", st, err)
	}
}

func TestIntegrationBranchMustDifferFromMain(t *testing.T) {
	f := newFx(t)
	var hostID uuid.UUID
	_ = f.pool.QueryRow(f.ctx, `SELECT id FROM exec.hosts LIMIT 1`).Scan(&hostID)
	_, err := f.svc.CreateRepository(f.ctx, f.user, Repository{ProjectID: f.project, HostID: hostID, Name: "x",
		LocalPath: `C:\repos\x`, DefaultBranch: "main", IntegrationBranch: "main"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("AiCrew must never write to main: %v", err)
	}
}

func TestKeepWorktreesCoversMergeQueue(t *testing.T) {
	f := newFx(t)
	id := f.mergeable()
	_ = f.svc.Merge(f.ctx, f.user, id)
	ids, _ := f.svc.KeepWorktrees(f.ctx, f.host)
	if len(ids) != 1 || ids[0] != id {
		t.Fatalf("queued merge needs its worktree: %v", ids)
	}
	f.done2(id)
	if ids, _ := f.svc.KeepWorktrees(f.ctx, f.host); len(ids) != 0 {
		t.Fatalf("done task needs no worktree: %v", ids)
	}
}

func (f *fx) done2(id uuid.UUID) {
	f.t.Helper()
	j := f.claim()
	f.move(j, StatusPostMergeVerify, Facts{})
	f.move(j, StatusMerged, Facts{MergeCommit: "m"})
	f.move(j, StatusDone, Facts{})
}
