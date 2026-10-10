package host

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/gitx"
	"prompttree/aicrew/internal/verify"
)

// Этап 7.4: контур проверок (docs/stage7-4-checks.md).

const sandboxContract = `version: 1
modules:
  app: { paths: ["lib/**"] }
forbidden_imports: ["dart:mirrors", "package:http"]
protected_paths: [pubspec.yaml, architecture.yaml]
readonly_paths: [".github/**"]
`

// repoWithContract — репозиторий, где architecture.yaml и pubspec.yaml уже в main.
func repoWithContract(t *testing.T) string {
	repo := gitRepo(t)
	for name, body := range map[string]string{"architecture.yaml": sandboxContract, "pubspec.yaml": "name: todo\n"} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "contract")
	return repo
}

func writesAll(files map[string]string) func(context.Context, agent.Request) (agent.Result, error) {
	return func(_ context.Context, req agent.Request) (agent.Result, error) {
		for name, body := range files {
			p := filepath.Join(req.Workdir, filepath.FromSlash(name))
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				return agent.Result{}, err
			}
		}
		return agent.Result{Summary: "done"}, nil
	}
}

func runTask(t *testing.T, repo string, files map[string]string, rv *fakeReviewer, mutate func(*api.Task)) (*fakeServer, *fakeAgent) {
	t.Helper()
	srv := &fakeServer{}
	ag := &fakeAgent{run: writesAll(files)}
	h := newHost(t, srv, ag, fakeVerifier{rep: verify.Report{Passed: true}})
	if rv != nil {
		h.Reviewer = rv
	}
	tk := task(repo)
	if mutate != nil {
		mutate(tk)
	}
	h.Process(context.Background(), tk)
	return srv, ag
}

func last(srv *fakeServer) (string, api.Facts) {
	got := srv.got()
	return got[len(got)-1], srv.facts[len(srv.facts)-1]
}

func TestApprovedReviewMakesTaskMergeable(t *testing.T) {
	repo := repoWithContract(t)
	rv := &fakeReviewer{}
	srv, _ := runTask(t, repo, map[string]string{"lib/a.dart": "import 'package:todo/b.dart';\n"}, rv, nil)
	st, f := last(srv)
	if st != "MERGEABLE" || f.Review == nil || f.Review.Verdict != "approve" || len(f.ApprovalRequired) != 0 {
		t.Fatalf("%s %+v", st, f)
	}
	if strings.Join(rv.roles, ",") != "logic_qa" {
		t.Fatalf("only Logic QA by default: %v", rv.roles)
	}
	if !strings.Contains(rv.facts.Diff, "+import 'package:todo/b.dart';") || len(rv.facts.ChangedFiles) != 1 {
		t.Fatalf("reviewer must get the real diff: %+v", rv.facts)
	}
}

func TestReviewRejectIsQARejectWithIssues(t *testing.T) {
	repo := gitRepo(t)
	rv := &fakeReviewer{verdict: map[string]api.Verdict{"logic_qa": {Verdict: "reject", RejectionType: "code_bug",
		Issues: []api.Issue{{File: "lib/a.dart", Line: 3, Severity: "blocker", Message: "empty title is saved"}}, Summary: "bug"}}}
	srv, _ := runTask(t, repo, map[string]string{"lib/a.dart": "x"}, rv, nil)
	st, f := last(srv)
	if st != "FAILED" || f.FailureClass != "QA_REJECT" || f.FailureCode != "logic_qa_rejected" || !strings.Contains(f.Reason, "empty title is saved") {
		t.Fatalf("%s %+v", st, f)
	}
	if exists(gitx.WorktreePath(repo, "t1")) {
		t.Fatal("rejected attempt's worktree must be removed")
	}
	// Замечания доходят до следующей попытки.
	facts, _ := json.Marshal(f.Details)
	class, code := f.FailureClass, f.FailureCode
	ls := lessons([]api.PreviousAttempt{{Attempt: 1, FailureClass: &class, FailureCode: &code, Facts: facts}})
	if len(ls) != 1 || !strings.Contains(ls[0], "lib/a.dart:3 empty title is saved") {
		t.Fatalf("lessons: %v", ls)
	}
}

func TestContractViolationStopsBeforeReview(t *testing.T) {
	repo := repoWithContract(t)
	rv := &fakeReviewer{}
	srv, _ := runTask(t, repo, map[string]string{"lib/a.dart": "import 'dart:mirrors';\n"}, rv, nil)
	st, f := last(srv)
	if st != "FAILED" || f.FailureClass != "QA_REJECT" || f.FailureCode != "contract_violation" || !strings.Contains(f.Reason, "dart:mirrors") {
		t.Fatalf("%s %+v", st, f)
	}
	if len(rv.roles) != 0 {
		t.Fatal("LLM must not review code that broke the contract (ТЗ п. 9: deterministic checks first)")
	}
}

// Исполнитель не может ослабить контракт своей правкой: проверка идёт по контракту из base_commit.
func TestContractIsReadFromBaseCommit(t *testing.T) {
	repo := repoWithContract(t)
	srv, _ := runTask(t, repo, map[string]string{
		"architecture.yaml": "version: 1\n",
		"lib/a.dart":        "import 'package:http/http.dart';\n",
	}, nil, nil)
	if st, f := last(srv); st != "FAILED" || f.FailureCode != "contract_violation" {
		t.Fatalf("weakened contract must not help: %s %+v", st, f)
	}
}

func TestProtectedChangesAreMarkedForApproval(t *testing.T) {
	repo := repoWithContract(t)
	srv, _ := runTask(t, repo, map[string]string{"pubspec.yaml": "name: todo\ndependencies:\n  uuid: ^4.0.0\n"}, nil, nil)
	st, f := last(srv)
	if st != "MERGEABLE" || len(f.ApprovalRequired) != 1 || f.ApprovalRequired[0] != "pubspec.yaml" {
		t.Fatalf("%s %+v", st, f)
	}
}

func TestReadonlyPathIsAViolation(t *testing.T) {
	repo := repoWithContract(t)
	srv, _ := runTask(t, repo, map[string]string{".github/workflows/ci.yml": "on: push\n"}, nil, nil)
	if st, f := last(srv); st != "FAILED" || f.FailureCode != "contract_violation" {
		t.Fatalf("%s %+v", st, f)
	}
}

func TestTechLeadReviewsFlaggedTasks(t *testing.T) {
	repo := gitRepo(t)
	rv := &fakeReviewer{}
	srv, _ := runTask(t, repo, map[string]string{"lib/a.dart": "x"}, rv, func(tk *api.Task) { tk.ReviewLevel = "tech_lead" })
	st, f := last(srv)
	if st != "MERGEABLE" || strings.Join(rv.roles, ",") != "logic_qa,tech_lead" || f.Review.Reviewer != "tech_lead:fake" {
		t.Fatalf("%s %v %+v", st, rv.roles, f.Review)
	}
	rv = &fakeReviewer{verdict: map[string]api.Verdict{"tech_lead": {Verdict: "reject", RejectionType: "code_bug",
		Issues: []api.Issue{{Severity: "major", Message: "duplicated state"}}}}}
	srv, _ = runTask(t, gitRepo(t), map[string]string{"lib/a.dart": "x"}, rv, func(tk *api.Task) { tk.ReviewLevel = "tech_lead" })
	if _, f := last(srv); f.FailureCode != "tech_lead_rejected" {
		t.Fatalf("%+v", f)
	}
}

func TestAgentIsToldWhichChecksWillRun(t *testing.T) {
	repo := gitRepo(t)
	_, ag := runTask(t, repo, map[string]string{"a": "x"}, nil, func(tk *api.Task) {
		tk.Repository.VerificationProfile = json.RawMessage(`{"template":"flutter"}`)
	})
	checks := strings.Join(ag.req.Checks, "\n")
	if !strings.Contains(checks, "flutter test --machine") || strings.Contains(checks, "build web") {
		t.Fatalf("checks: %v", ag.req.Checks)
	}
	if p := agent.BuildPrompt(ag.req); !strings.Contains(p, "dart format --output=none") {
		t.Fatal("prompt must list the checks")
	}
}
