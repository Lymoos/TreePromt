package host

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/gitx"
	"prompttree/aicrew/internal/runner"
	"prompttree/aicrew/internal/verify"
)

// Этап 7.3: очередь слияния на настоящем git (docs/stage7-3-merge-queue.md).

// funcVerifier — проверка, которая смотрит на файлы результата.
type funcVerifier func(workdir string) verify.Report

func (v funcVerifier) Run(_ context.Context, _ verify.Profile, workdir, _ string, _ time.Duration) (verify.Report, error) {
	return v(workdir), nil
}

var pass = verify.Report{Passed: true}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func fileAt(t *testing.T, repo, ref, name string) (string, bool) {
	t.Helper()
	cmd := exec.Command("git", "show", ref+":"+name)
	cmd.Dir = repo
	out, err := cmd.Output()
	return string(out), err == nil
}

// execute проводит задачу через агента до MERGEABLE.
func execute(t *testing.T, repo, id, file, content string) {
	t.Helper()
	srv := &fakeServer{}
	tk := task(repo)
	tk.ID = id
	newHost(t, srv, &fakeAgent{run: writes(file, content)}, fakeVerifier{rep: pass}).Process(context.Background(), tk)
	if got := srv.got(); got[len(got)-1] != "MERGEABLE" {
		t.Fatalf("%s: %v", id, got)
	}
}

func job(repo, id, kind string) *api.Task {
	tk := task(repo)
	tk.ID, tk.Kind, tk.Title = id, kind, "задача "+id
	return tk
}

func mergeJob(t *testing.T, repo, id string, v Verifier) *fakeServer {
	t.Helper()
	srv := &fakeServer{}
	h := newHost(t, srv, &fakeAgent{}, fakeVerifier{})
	h.Verifier = v
	h.Process(context.Background(), job(repo, id, api.KindMerge))
	return srv
}

func TestMergeMovesIntegrationOnlyAfterChecks(t *testing.T) {
	repo := gitRepo(t)
	mainBefore := git(t, repo, "rev-parse", "main")
	execute(t, repo, "a", "a.dart", "A\n")

	srv := mergeJob(t, repo, "a", funcVerifier(func(string) verify.Report { return pass }))
	if got := strings.Join(srv.got(), ","); got != "POST_MERGE_VERIFY,MERGED,DONE" {
		t.Fatalf("transitions %s", got)
	}
	integ := git(t, repo, "rev-parse", "aicrew/integration")
	if srv.facts[1].MergeCommit != integ {
		t.Fatalf("merge commit %q, integration %q", srv.facts[1].MergeCommit, integ)
	}
	if parents := strings.Fields(git(t, repo, "rev-list", "--parents", "-n", "1", integ)); len(parents) != 3 {
		t.Fatal("each task must be one merge commit (--no-ff), so it can be rolled back as a whole")
	}
	if _, ok := fileAt(t, repo, "aicrew/integration", "a.dart"); !ok {
		t.Fatal("integration must contain the task")
	}
	if git(t, repo, "rev-parse", "main") != mainBefore {
		t.Fatal("AiCrew must never move main")
	}
	if exists(gitx.WorktreePath(repo, "a")) || exists(gitx.TempWorktreePath(repo, "merge-a")) {
		t.Fatal("worktrees must be cleaned after merge")
	}
}

func TestNextTaskStartsFromIntegration(t *testing.T) {
	repo := gitRepo(t)
	execute(t, repo, "a", "a.dart", "A\n")
	mergeJob(t, repo, "a", funcVerifier(func(string) verify.Report { return pass }))

	var sawA bool
	srv := &fakeServer{}
	tk := task(repo)
	tk.ID = "b"
	ag := &fakeAgent{run: func(ctx context.Context, req agent.Request) (agent.Result, error) {
		sawA = exists(filepath.Join(req.Workdir, "a.dart"))
		return writes("b.dart", "B\n")(ctx, req)
	}}
	newHost(t, srv, ag, fakeVerifier{rep: pass}).Process(context.Background(), tk)
	if !sawA {
		t.Fatal("the next task must see what the previous one merged")
	}
}

// Сценарий 7.3: конфликт слияния — integration не двигается, задача уходит на переделку.
func TestMergeConflictLeavesIntegrationUntouched(t *testing.T) {
	repo := gitRepo(t)
	execute(t, repo, "a", "shared.dart", "версия A\n")
	execute(t, repo, "b", "shared.dart", "версия B\n") // обе от одной базы
	mergeJob(t, repo, "a", funcVerifier(func(string) verify.Report { return pass }))
	before := git(t, repo, "rev-parse", "aicrew/integration")

	srv := mergeJob(t, repo, "b", funcVerifier(func(string) verify.Report { return pass }))
	last := srv.facts[len(srv.facts)-1]
	if got := srv.got(); len(got) != 1 || got[0] != "FAILED" || last.FailureClass != "MERGE_CONFLICT" {
		t.Fatalf("got %v %+v", got, last)
	}
	files, _ := last.Details.(map[string]any)["conflicted_files"].([]string)
	if len(files) != 1 || files[0] != "shared.dart" {
		t.Fatalf("conflicted files: %v", last.Details)
	}
	if git(t, repo, "rev-parse", "aicrew/integration") != before {
		t.Fatal("integration must not move on conflict")
	}
	if exists(gitx.TempWorktreePath(repo, "merge-b")) {
		t.Fatal("temp worktree must be removed")
	}
}

// Сценарий 7.3: A и B по отдельности зелёные, вместе ломают. Проверка после слияния ловит.
func TestTasksBreakingOnlyTogetherAreCaught(t *testing.T) {
	repo := gitRepo(t)
	brokenTogether := funcVerifier(func(dir string) verify.Report {
		if exists(filepath.Join(dir, "a.dart")) && exists(filepath.Join(dir, "b.dart")) {
			return verify.Report{Passed: false, FailedKind: "test",
				Steps: []verify.StepResult{{Name: "flutter test", ExitCode: 1, Output: "duplicate route /todo"}}}
		}
		return pass
	})
	execute(t, repo, "a", "a.dart", "A\n")
	execute(t, repo, "b", "b.dart", "B\n")
	mergeJob(t, repo, "a", brokenTogether)
	before := git(t, repo, "rev-parse", "aicrew/integration")

	srv := mergeJob(t, repo, "b", brokenTogether)
	last := srv.facts[len(srv.facts)-1]
	if got := strings.Join(srv.got(), ","); got != "POST_MERGE_VERIFY,FAILED" || last.FailureClass != "INTEGRATION_FAILURE" ||
		!strings.Contains(last.Reason, "duplicate route") {
		t.Fatalf("got %s %+v", got, last)
	}
	if git(t, repo, "rev-parse", "aicrew/integration") != before {
		t.Fatal("integration must stay green")
	}
}

func TestRollbackRevertsTheMergeWithChecks(t *testing.T) {
	repo := gitRepo(t)
	execute(t, repo, "a", "a.dart", "A\n")
	srv := mergeJob(t, repo, "a", funcVerifier(func(string) verify.Report { return pass }))
	merge := srv.facts[1].MergeCommit

	rsrv := &fakeServer{}
	tk := job(repo, "a", api.KindRollback)
	tk.MergeCommit = merge
	newHost(t, rsrv, &fakeAgent{}, fakeVerifier{rep: pass}).Process(context.Background(), tk)
	if got := strings.Join(rsrv.got(), ","); got != "ROLLED_BACK" || rsrv.facts[0].RevertCommit == "" {
		t.Fatalf("got %s %+v", got, rsrv.facts)
	}
	if _, ok := fileAt(t, repo, "aicrew/integration", "a.dart"); ok {
		t.Fatal("rolled back task must be gone from integration")
	}
	if git(t, repo, "rev-parse", "aicrew/integration") != rsrv.facts[0].RevertCommit {
		t.Fatal("integration must point to the revert commit")
	}
}

func TestFailedRollbackChecksKeepIntegration(t *testing.T) {
	repo := gitRepo(t)
	execute(t, repo, "a", "a.dart", "A\n")
	merge := mergeJob(t, repo, "a", funcVerifier(func(string) verify.Report { return pass })).facts[1].MergeCommit
	before := git(t, repo, "rev-parse", "aicrew/integration")

	rsrv := &fakeServer{}
	tk := job(repo, "a", api.KindRollback)
	tk.MergeCommit = merge
	newHost(t, rsrv, &fakeAgent{}, fakeVerifier{rep: verify.Report{Passed: false,
		Steps: []verify.StepResult{{Name: "flutter analyze", ExitCode: 1}}}}).Process(context.Background(), tk)
	if got := rsrv.got(); got[len(got)-1] != "FAILED" {
		t.Fatalf("got %v", got)
	}
	if git(t, repo, "rev-parse", "aicrew/integration") != before {
		t.Fatal("failed rollback must not move integration")
	}
}

func TestPromoteFastForwardsMainOnlyWhenSafe(t *testing.T) {
	repo := gitRepo(t)
	execute(t, repo, "a", "a.dart", "A\n")
	mergeJob(t, repo, "a", funcVerifier(func(string) verify.Report { return pass }))
	g := gitx.Git{R: runner.New("git")}
	ctx := context.Background()

	// main выгружена и в ней незакоммиченная правка пользователя — не трогаем.
	if err := os.WriteFile(filepath.Join(repo, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "notes.txt")
	if _, _, err := g.FastForward(ctx, repo, "main", "aicrew/integration"); err == nil {
		t.Fatal("must refuse with uncommitted changes in the checked-out main")
	}
	git(t, repo, "reset", "-q", "--hard")

	from, to, err := g.FastForward(ctx, repo, "main", "aicrew/integration")
	if err != nil || from == to || git(t, repo, "rev-parse", "main") != git(t, repo, "rev-parse", "aicrew/integration") {
		t.Fatalf("fast-forward: %s → %s %v", from, to, err)
	}
	if !exists(filepath.Join(repo, "a.dart")) {
		t.Fatal("checked-out files must follow main")
	}

	// В main появился свой коммит — перемотка невозможна, сливать руками.
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "manual")
	execute(t, repo, "c", "c.dart", "C\n")
	mergeJob(t, repo, "c", funcVerifier(func(string) verify.Report { return pass }))
	if _, _, err := g.FastForward(ctx, repo, "main", "aicrew/integration"); err == nil {
		t.Fatal("diverged main must not be overwritten")
	}
}

func TestTempMergeWorktreesAreCleanedOnStart(t *testing.T) {
	repo := gitRepo(t)
	g := gitx.Git{R: runner.New("git")}
	if _, err := g.AddTempWorktree(context.Background(), repo, "merge-zombie", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := newHost(t, &fakeServer{}, &fakeAgent{}, fakeVerifier{}).CleanupOrphans(context.Background(), []string{repo}); err != nil {
		t.Fatal(err)
	}
	if exists(gitx.TempWorktreePath(repo, "merge-zombie")) {
		t.Fatal("leftover merge worktree must be removed")
	}
}
