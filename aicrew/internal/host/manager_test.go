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
	h.Process(context.Background(), tk)
	if junior.req.Role != "junior" || claude.req.TaskID != "" {
		t.Fatal("junior task must run on the junior executor")
	}
	if d := srv.facts[1].Details.(map[string]any); d["executor"] != "junior" {
		t.Fatalf("executor must be recorded: %v", d["executor"])
	}
	srv2 := &fakeServer{}
	h2 := newHost(t, srv2, claude, fakeVerifier{})
	tk2 := task(gitRepo(t))
	tk2.Executor = "junior"
	h2.Process(context.Background(), tk2)
	if _, f := last(srv2); f.FailureCode != "no_junior" {
		t.Fatalf("%+v", f)
	}
}
