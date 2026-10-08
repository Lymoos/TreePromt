package host

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/gitx"
	"prompttree/aicrew/internal/runner"
	"prompttree/aicrew/internal/verify"
)

// ── имитации ──

type fakeServer struct {
	mu          sync.Mutex
	task        *api.Task
	transitions []string
	facts       []api.Facts
	cancel      bool
	lostLease   bool
	keep        map[string]bool
}

func (s *fakeServer) Claim(context.Context, string) (*api.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.task
	s.task = nil
	return t, nil
}

func (s *fakeServer) Heartbeat(context.Context, string, string, int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lostLease {
		return false, api.ErrNotOwner
	}
	return s.cancel, nil
}

func (s *fakeServer) Transition(_ context.Context, _, _ string, _ int, to string, f api.Facts) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lostLease {
		return "", api.ErrNotOwner
	}
	s.transitions = append(s.transitions, to)
	s.facts = append(s.facts, f)
	return to, nil
}

func (s *fakeServer) KeepWorktrees(context.Context) (map[string]bool, error) { return s.keep, nil }

func (s *fakeServer) got() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.transitions...)
}

type fakeAgent struct {
	run     func(ctx context.Context, req agent.Request) (agent.Result, error)
	stopped bool
	req     agent.Request
}

func (a *fakeAgent) Name() string { return "fake" }
func (a *fakeAgent) Run(ctx context.Context, req agent.Request) (agent.Result, error) {
	a.req = req
	return a.run(ctx, req)
}
func (a *fakeAgent) Stop(context.Context, string, int) error { a.stopped = true; return nil }

type fakeVerifier struct{ rep verify.Report }

func (v fakeVerifier) Run(context.Context, verify.Profile, string, string, time.Duration) (verify.Report, error) {
	return v.rep, nil
}

// writes — агент, который добавляет файл в worktree.
func writes(name, content string) func(context.Context, agent.Request) (agent.Result, error) {
	return func(_ context.Context, req agent.Request) (agent.Result, error) {
		return agent.Result{Summary: "added " + name}, os.WriteFile(filepath.Join(req.Workdir, name), []byte(content), 0o644)
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return dir
}

func newHost(t *testing.T, srv *fakeServer, ag *fakeAgent, v fakeVerifier) *Host {
	return &Host{API: srv, Git: gitx.Git{R: runner.New("git")}, Agent: ag, Verifier: v, WorkerID: "w1",
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), PollEvery: 10 * time.Millisecond, HeartbeatEvery: 20 * time.Millisecond}
}

func task(repo string) *api.Task {
	return &api.Task{ID: "t1", Attempt: 1, Title: "Кнопка", Prompt: "Добавить кнопку", AgentRuntimeSec: 60, VerifyRuntimeSec: 60,
		Repository: api.Repository{ID: "r1", LocalPath: repo, DefaultBranch: "main", VerificationProfile: json.RawMessage(`{}`)}}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// ── сценарии ──

func TestHappyPathEndsMergeableWithFacts(t *testing.T) {
	repo := gitRepo(t)
	srv := &fakeServer{}
	ag := &fakeAgent{run: writes("button.dart", "// кнопка\n")}
	h := newHost(t, srv, ag, fakeVerifier{rep: verify.Report{Passed: true}})
	h.Process(context.Background(), task(repo))

	want := []string{"IN_PROGRESS", "VERIFYING", "LLM_REVIEW", "MERGEABLE"}
	if got := srv.got(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("transitions %v", got)
	}
	if srv.facts[0].BaseCommit == "" || srv.facts[1].ResultCommit == "" || srv.facts[1].ResultCommit == srv.facts[0].BaseCommit {
		t.Fatalf("base/result commits: %+v", srv.facts[:2])
	}
	details := srv.facts[1].Details.(map[string]any)
	if files := details["changed_files"].([]string); len(files) != 1 || files[0] != "button.dart" {
		t.Fatalf("changed files: %v", files)
	}
	if !exists(gitx.WorktreePath(repo, "t1")) {
		t.Fatal("worktree must stay until merge (7.3)")
	}
	if !strings.Contains(ag.req.Prompt, "Добавить кнопку") {
		t.Fatal("agent must get the task text")
	}
}

func TestAgentFailureReportsClassAndRemovesWorktree(t *testing.T) {
	repo := gitRepo(t)
	srv := &fakeServer{}
	ag := &fakeAgent{run: func(context.Context, agent.Request) (agent.Result, error) {
		return agent.Result{}, &agent.Failure{Class: "RATE_LIMIT", Code: "provider_limit", Message: "usage limit", RetryAfter: 10 * time.Minute}
	}}
	newHost(t, srv, ag, fakeVerifier{}).Process(context.Background(), task(repo))
	got := srv.got()
	if got[len(got)-1] != "FAILED" || srv.facts[len(got)-1].FailureClass != "RATE_LIMIT" || srv.facts[len(got)-1].RetryAfterSec != 600 {
		t.Fatalf("got %v %+v", got, srv.facts)
	}
	if exists(gitx.WorktreePath(repo, "t1")) {
		t.Fatal("failed task worktree must be removed (ТЗ 8.5)")
	}
}

func TestRedTestsAreTestFailureWithOutput(t *testing.T) {
	repo := gitRepo(t)
	srv := &fakeServer{}
	rep := verify.Report{Passed: false, FailedKind: "test", Steps: []verify.StepResult{{Name: "flutter test", ExitCode: 1, Output: "Expected: 2 Actual: 3"}}}
	newHost(t, srv, &fakeAgent{run: writes("a.dart", "x")}, fakeVerifier{rep: rep}).Process(context.Background(), task(repo))
	last := srv.facts[len(srv.facts)-1]
	if last.FailureClass != "TEST_FAILURE" || !strings.Contains(last.Reason, "Expected: 2") {
		t.Fatalf("got %+v", last)
	}
}

func TestNoChangesIsAnAgentError(t *testing.T) {
	repo := gitRepo(t)
	srv := &fakeServer{}
	ag := &fakeAgent{run: func(context.Context, agent.Request) (agent.Result, error) { return agent.Result{}, nil }}
	newHost(t, srv, ag, fakeVerifier{}).Process(context.Background(), task(repo))
	if last := srv.facts[len(srv.facts)-1]; last.FailureClass != "AGENT_ERROR" || last.FailureCode != "no_changes" {
		t.Fatalf("got %+v", last)
	}
}

func TestUserCancelStopsAgentAndConfirms(t *testing.T) {
	repo := gitRepo(t)
	srv := &fakeServer{}
	ag := &fakeAgent{run: func(ctx context.Context, _ agent.Request) (agent.Result, error) {
		srv.mu.Lock()
		srv.cancel = true // пользователь нажал «Отменить», хост узнает из heartbeat
		srv.mu.Unlock()
		<-ctx.Done()
		return agent.Result{}, ctx.Err()
	}}
	newHost(t, srv, ag, fakeVerifier{}).Process(context.Background(), task(repo))
	got := srv.got()
	if got[len(got)-1] != "CANCELLED" || !ag.stopped {
		t.Fatalf("got %v stopped=%v", got, ag.stopped)
	}
	if exists(gitx.WorktreePath(repo, "t1")) {
		t.Fatal("cancelled task worktree must be removed")
	}
}

func TestLostLeaseStopsWithoutFurtherReports(t *testing.T) {
	repo := gitRepo(t)
	srv := &fakeServer{}
	ag := &fakeAgent{run: func(ctx context.Context, _ agent.Request) (agent.Result, error) {
		srv.mu.Lock()
		srv.lostLease = true // сервер уже отдал задачу другому (ПК «засыпал»)
		srv.mu.Unlock()
		<-ctx.Done()
		return agent.Result{}, ctx.Err()
	}}
	newHost(t, srv, ag, fakeVerifier{}).Process(context.Background(), task(repo))
	if got := srv.got(); len(got) != 1 || got[0] != "IN_PROGRESS" {
		t.Fatalf("worker without lease must not report: %v", got)
	}
}

func TestPreviousFailuresReachTheNextAttempt(t *testing.T) {
	repo := gitRepo(t)
	srv := &fakeServer{}
	ag := &fakeAgent{run: writes("a.dart", "x")}
	tk := task(repo)
	class, code := "TEST_FAILURE", "verification_failed"
	facts, _ := json.Marshal(map[string]any{"verification": verify.Report{Steps: []verify.StepResult{{Name: "flutter test", ExitCode: 1, Output: "Null check operator"}}}})
	tk.Attempt, tk.PreviousAttempts = 2, []api.PreviousAttempt{{Attempt: 1, FailureClass: &class, FailureCode: &code, Facts: facts}}
	newHost(t, srv, ag, fakeVerifier{rep: verify.Report{Passed: true}}).Process(context.Background(), tk)
	if len(ag.req.Previous) != 1 || !strings.Contains(ag.req.Previous[0], "Null check operator") {
		t.Fatalf("lessons: %v", ag.req.Previous)
	}
	if p := agent.BuildPrompt(ag.req); !strings.Contains(p, "Do not repeat these mistakes") {
		t.Fatal("prompt must carry lessons")
	}
}

func TestOrphanWorktreesAreCleanedOnStart(t *testing.T) {
	repo := gitRepo(t)
	g := gitx.Git{R: runner.New("git")}
	for _, id := range []string{"keep", "orphan"} {
		if _, _, err := g.AddWorktree(context.Background(), repo, id, "main"); err != nil {
			t.Fatal(err)
		}
	}
	srv := &fakeServer{keep: map[string]bool{"keep": true}}
	removed, err := newHost(t, srv, &fakeAgent{}, fakeVerifier{}).CleanupOrphans(context.Background(), []string{repo})
	if err != nil || len(removed) != 1 || removed[0] != "orphan" {
		t.Fatalf("removed %v %v", removed, err)
	}
	if !exists(gitx.WorktreePath(repo, "keep")) || exists(gitx.WorktreePath(repo, "orphan")) {
		t.Fatal("wrong worktree removed")
	}
}

func TestRunStopsOnContextAndPolls(t *testing.T) {
	repo := gitRepo(t)
	srv := &fakeServer{task: task(repo)}
	h := newHost(t, srv, &fakeAgent{run: writes("a.dart", "x")}, fakeVerifier{rep: verify.Report{Passed: true}})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() {
		for len(srv.got()) < 4 {
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
	}()
	if err := h.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if got := srv.got(); len(got) != 4 {
		t.Fatalf("got %v", got)
	}
}
