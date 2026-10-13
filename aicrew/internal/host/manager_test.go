package host

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/manager"
	"prompttree/aicrew/internal/mcp"
	"prompttree/aicrew/internal/skills"
	"prompttree/aicrew/internal/verify"
)

// Этап 7.5: Manager, скиллы, MCP, Junior (docs/stage7-5-manager-skills-mcp.md).

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

type fakePlanner struct {
	p       *api.Proposal
	err     error
	workdir string
	sawCode bool
}

func (f *fakePlanner) Plan(_ context.Context, _ *api.Task, workdir string, _ manager.Catalog) (*api.Proposal, agent.Usage, error) {
	f.workdir = workdir
	_, err := os.Stat(filepath.Join(workdir, "lib", "main.dart"))
	f.sawCode = err == nil
	return f.p, agent.Usage{Turns: 3}, f.err
}

func TestPlanJobLooksAtIntegrationAndSubmits(t *testing.T) {
	repo := gitRepo(t)
	_ = os.MkdirAll(filepath.Join(repo, "lib"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, "lib", "main.dart"), []byte("void main() {}\n"), 0o644)
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "app")

	srv := &fakeServer{}
	pl := &fakePlanner{p: &api.Proposal{Tasks: []api.PlanTask{{Title: "a", Prompt: "b"}}}}
	h := newHost(t, srv, &fakeAgent{}, fakeVerifier{})
	h.Planner = pl
	tk := job(repo, "p1", api.KindPlan)
	tk.Prompt = "Сделать фильтры"
	h.Process(context.Background(), tk)

	if !pl.sawCode {
		t.Fatal("Manager must see the integration code")
	}
	if srv.plan == nil || srv.plan.Tasks[0].Title != "a" || len(srv.got()) != 0 {
		t.Fatalf("plan must be submitted, no task transitions: %+v %v", srv.plan, srv.got())
	}
	if exists(pl.workdir) {
		t.Fatal("temporary plan worktree must be removed")
	}
}

func TestPlanFailureIsReported(t *testing.T) {
	repo := gitRepo(t)
	srv := &fakeServer{}
	h := newHost(t, srv, &fakeAgent{}, fakeVerifier{})
	h.Planner = &fakePlanner{err: errors.New("bad_plan: cycle")}
	h.Process(context.Background(), job(repo, "p1", api.KindPlan))
	if srv.plan != nil || !strings.Contains(srv.planFailure, "cycle") {
		t.Fatalf("%+v %q", srv.plan, srv.planFailure)
	}
}

func TestSkillsAreMountedReadOnlyAndRecorded(t *testing.T) {
	reg, err := skills.Load(filepath.Join(repoRoot(), "skills"))
	if err != nil {
		t.Fatal(err)
	}
	repo := gitRepo(t)
	srv := &fakeServer{}
	var sawSkill bool
	ag := &fakeAgent{run: func(ctx context.Context, req agent.Request) (agent.Result, error) {
		for _, m := range req.Mounts {
			if m.Container == agent.SkillsMount && m.ReadOnly {
				_, err := os.Stat(filepath.Join(m.Host, "flutter-widget-tests", "SKILL.md"))
				sawSkill = err == nil
			}
		}
		return writes("a.dart", "x")(ctx, req)
	}}
	h := newHost(t, srv, ag, fakeVerifier{rep: verify.Report{Passed: true}})
	h.Skills = reg
	tk := task(repo)
	tk.RequiredSkills = []string{"flutter-widget-tests"}
	h.Process(context.Background(), tk)
	if !sawSkill {
		t.Fatal("skill must be visible to the agent, read-only")
	}
	details := srv.facts[1].Details.(map[string]any)
	att := details["skills"].([]skills.Attached)
	if len(att) != 1 || att[0].Name != "flutter-widget-tests" || len(att[0].Hash) != 16 {
		t.Fatalf("skills must be recorded with their version: %+v", details["skills"])
	}
	for _, m := range ag.req.Mounts {
		if exists(m.Host) {
			t.Fatal("temporary skills folder must be removed after the run")
		}
	}
}

func TestUnknownSkillStopsTheRun(t *testing.T) {
	reg, _ := skills.Load(filepath.Join(repoRoot(), "skills"))
	repo := gitRepo(t)
	srv := &fakeServer{}
	h := newHost(t, srv, &fakeAgent{run: writes("a", "x")}, fakeVerifier{rep: verify.Report{Passed: true}})
	h.Skills = reg
	tk := task(repo)
	tk.RequiredSkills = []string{"steal-tokens"}
	h.Process(context.Background(), tk)
	if _, f := last(srv); f.FailureCode != "skills_failed" {
		t.Fatalf("%+v", f)
	}
}

func TestMCPProfileGivesOnlyItsTools(t *testing.T) {
	reg, err := mcp.Load(filepath.Join(repoRoot(), "mcp", "registry.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	repo := gitRepo(t)
	srv := &fakeServer{}
	ag := &fakeAgent{run: writes("a", "x")}
	h := newHost(t, srv, ag, fakeVerifier{rep: verify.Report{Passed: true}})
	h.MCP = reg
	tk := task(repo)
	tk.MCPProfile = "web"
	h.Process(context.Background(), tk)
	if strings.Join(ag.req.ExtraTools, ",") != "WebFetch" || ag.req.MCPConfig != nil {
		t.Fatalf("web profile: %v %s", ag.req.ExtraTools, ag.req.MCPConfig)
	}
	// Профиль Roblox в контейнере не выдаётся (MCP-scopes).
	srv2 := &fakeServer{}
	h2 := newHost(t, srv2, &fakeAgent{run: writes("a", "x")}, fakeVerifier{rep: verify.Report{Passed: true}})
	h2.MCP = reg
	tk2 := task(gitRepo(t))
	tk2.MCPProfile = "roblox"
	h2.Process(context.Background(), tk2)
	if _, f := last(srv2); f.FailureCode != "mcp_profile_failed" {
		t.Fatalf("%+v", f)
	}
}

func TestJuniorTasksGoToTheLocalModel(t *testing.T) {
	repo := gitRepo(t)
	srv := &fakeServer{}
	claude := &fakeAgent{run: writes("a", "x")}
	junior := &fakeAgent{run: writes("b", "y")}
	h := newHost(t, srv, claude, fakeVerifier{rep: verify.Report{Passed: true}})
	h.Junior = junior
	tk := task(repo)
	tk.Executor = "junior"
	tk.Contract = &api.Contract{Files: []string{"b"}, Tests: []string{"b"}}
	h.Process(context.Background(), tk)
	if junior.req.Role != "junior" || claude.req.TaskID != "" {
		t.Fatal("junior task must run on the junior executor")
	}
	if !strings.Contains(junior.req.Prompt, "Contract of this task") || !strings.Contains(junior.req.Prompt, api.ContractMarker) {
		t.Fatalf("junior must get its contract: %s", junior.req.Prompt)
	}
	if d := srv.facts[1].Details.(map[string]any); d["executor"] != "junior" {
		t.Fatalf("executor must be recorded: %v", d["executor"])
	}
	srv2 := &fakeServer{}
	h2 := newHost(t, srv2, claude, fakeVerifier{})
	tk2 := task(gitRepo(t))
	tk2.Executor = "junior"
	tk2.Contract = tk.Contract
	h2.Process(context.Background(), tk2)
	if _, f := last(srv2); f.FailureCode != "no_junior" {
		t.Fatalf("%+v", f)
	}
}

func TestTouchesUIAddsVisionQA(t *testing.T) {
	repo := gitRepo(t)
	rv := &fakeReviewer{}
	srv, _ := runTask(t, repo, map[string]string{"lib/a.dart": "x"}, rv, func(tk *api.Task) { tk.TouchesUI = true })
	if st, _ := last(srv); st != "MERGEABLE" || strings.Join(rv.roles, ",") != "logic_qa,vision_qa" {
		t.Fatalf("%s %v", st, rv.roles)
	}
}

func TestWindowsUserTasksRunAsAIWorkerInsideWorkRoot(t *testing.T) {
	reg, _ := skills.Load(filepath.Join(repoRoot(), "skills"))
	repo := gitRepo(t) // репозиторий внутри «рабочей папки»
	work := filepath.Dir(repo)
	_ = os.MkdirAll(filepath.Join(work, ".runs"), 0o755)
	srv := &fakeServer{}
	docker := &fakeAgent{run: writes("a", "x")}
	var skillsIn string
	worker := &fakeAgent{run: func(ctx context.Context, req agent.Request) (agent.Result, error) {
		for _, m := range req.Mounts {
			skillsIn = m.Host
		}
		return writes("game.luau", "print('hi')\n")(ctx, req)
	}}
	h := newHost(t, srv, docker, fakeVerifier{})
	h.Skills = reg
	h.Windows = &WindowsEnv{WorkRoot: work, Agent: worker, Verifier: fakeVerifier{rep: verify.Report{Passed: true}}}
	tk := task(repo)
	tk.ExecutionEnv, tk.RequiredSkills = "windows_user", []string{"dart-clean-code"}
	h.Process(context.Background(), tk)
	if st, _ := last(srv); st != "MERGEABLE" || worker.req.Role != "tech_lead_windows" || docker.req.TaskID != "" {
		t.Fatalf("%s %+v", st, worker.req)
	}
	if !strings.HasPrefix(strings.ToLower(skillsIn), strings.ToLower(filepath.Join(work, ".runs"))) {
		t.Fatalf("skills for ai_worker must live inside the work root: %s", skillsIn)
	}

	srv2 := &fakeServer{}
	h2 := newHost(t, srv2, docker, fakeVerifier{})
	h2.Windows = &WindowsEnv{WorkRoot: t.TempDir(), Agent: worker, Verifier: fakeVerifier{}}
	tk2 := task(gitRepo(t))
	tk2.ExecutionEnv = "windows_user"
	h2.Process(context.Background(), tk2)
	if _, f := last(srv2); f.FailureCode != "repo_outside_work_root" {
		t.Fatalf("%+v", f)
	}
}

// Junior — только по контракту: ничего сверх разрешённых файлов, тесты-контракт без пропуска.
func TestJuniorContractIsEnforced(t *testing.T) {
	contract := &api.Contract{Files: []string{"lib/filter.dart"}, Tests: []string{"test/filter_test.dart"}}
	cases := map[string]struct {
		files    map[string]string
		contract *api.Contract
		code     string
	}{
		"no contract":      {map[string]string{"lib/filter.dart": "x"}, nil, "no_contract"},
		"outside contract": {map[string]string{"lib/filter.dart": "x", "test/filter_test.dart": "ok", "lib/main.dart": "hack"}, contract, "contract_broken"},
		"still skipped":    {map[string]string{"lib/filter.dart": "x", "test/filter_test.dart": "skip: '" + api.ContractMarker + "'"}, contract, "contract_unmet"},
		"done":             {map[string]string{"lib/filter.dart": "x", "test/filter_test.dart": "test passes"}, contract, ""},
	}
	for name, c := range cases {
		srv := &fakeServer{}
		h := newHost(t, srv, &fakeAgent{}, fakeVerifier{rep: verify.Report{Passed: true}})
		h.Junior = &fakeAgent{run: writesAll(c.files)}
		tk := task(gitRepo(t))
		tk.Executor, tk.Contract = "junior", c.contract
		h.Process(context.Background(), tk)
		_, f := last(srv)
		if f.FailureCode != c.code {
			t.Errorf("%s: failure %q, want %q", name, f.FailureCode, c.code)
		}
	}
}
