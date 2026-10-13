package exec

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Этап 7.5: планы Manager'а (docs/stage7-5-manager-skills-mcp.md).

func (f *fx) registries(skills, mcp, executors []string) {
	f.t.Helper()
	var hostID uuid.UUID
	_ = f.pool.QueryRow(f.ctx, `SELECT id FROM exec.hosts WHERE device_id = $1`, f.host.DeviceID).Scan(&hostID)
	if err := f.svc.SetHostRegistries(f.ctx, f.host, hostID, skills, mcp, executors); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fx) plan(mode string) (uuid.UUID, *ClaimedTask) {
	f.t.Helper()
	id, err := f.svc.CreatePlan(f.ctx, f.user, NewPlan{RepositoryID: f.repo, Request: "Сделать список дел с фильтрами", ExecutionMode: mode})
	if err != nil {
		f.t.Fatal(err)
	}
	c := f.claim()
	if c == nil || c.Kind != KindPlan || c.ID != id || c.Prompt == "" || c.Attempt != 1 {
		f.t.Fatalf("plan claim: %+v", c)
	}
	return id, c
}

func goodProposal() *Proposal {
	return &Proposal{Rationale: "модель и тесты-контракт, экран, функция фильтра по контракту, сборка", Tasks: []PlanTask{
		{Title: "Модель задачи", Prompt: "Добавить класс Todo и тесты-контракт для фильтра", RequiredCapabilities: []string{"flutter"},
			RequiredSkills: []string{"dart-clean-code"}},
		{Title: "Экран списка", Prompt: "Список и фильтры", DependsOn: []int{0}, TouchesUI: true, ReviewLevel: "tech_lead", MCPProfile: "web"},
		{Title: "Функция фильтра", Prompt: "Реализовать filterTodos", DependsOn: []int{0}, Executor: "junior",
			RequiredSkills: []string{"flutter-widget-tests"},
			Contract:       &Contract{Files: []string{"lib/filter.dart"}, Tests: []string{"./test/filter_test.dart"}}},
		{Title: "Сборка", Prompt: "Подключить фильтр к экрану", DependsOn: []int{1, 2}},
	}}
}

func TestManualPlanWaitsForApprovalThenCreatesTasksWithDependencies(t *testing.T) {
	f := newFx(t)
	f.registries([]string{"dart-clean-code", "flutter-widget-tests"}, []string{"web"}, []string{"claude", "junior"})
	id, c := f.plan("manual")
	st, reason, err := f.svc.SubmitPlan(f.ctx, f.host, id, f.worker, c.Attempt, goodProposal(), "")
	if err != nil || st != PlanAwaitingApproval || reason != "" {
		t.Fatalf("%s %s %v", st, reason, err)
	}
	if tasks, _ := f.svc.ListTasks(f.ctx, f.user, nil); len(tasks) != 0 {
		t.Fatal("manual plan must not create tasks before approval")
	}
	if err := f.svc.ApprovePlan(f.ctx, f.user, id); err != nil {
		t.Fatal(err)
	}
	plans, _ := f.svc.ListPlans(f.ctx, f.user)
	if len(plans) != 1 || plans[0].Status != PlanApproved || len(plans[0].TaskIDs) != 4 {
		t.Fatalf("plan: %+v", plans)
	}
	// Первая задача — без зависимостей, вторая ждёт первую.
	first := f.claim()
	if first == nil || first.Title != "Модель задачи" || strings.Join(first.RequiredSkills, ",") != "dart-clean-code" {
		t.Fatalf("first: %+v", first)
	}
	if f.claim() != nil {
		t.Fatal("dependent tasks must wait for DONE")
	}
	f.exec(`UPDATE exec.tasks SET status = 'DONE' WHERE id = $1`, first.ID)
	second := f.claim()
	if second == nil || !second.TouchesUI || second.ReviewLevel != "tech_lead" || second.MCPProfile != "web" {
		t.Fatalf("second: %+v", second)
	}
	f.exec(`UPDATE exec.tasks SET status = 'DONE' WHERE id = $1`, second.ID)
	third := f.claim()
	if third == nil || third.Executor != "junior" || third.Contract == nil ||
		strings.Join(third.Contract.Tests, ",") != "test/filter_test.dart" || third.Contract.Files[0] != "lib/filter.dart" {
		t.Fatalf("third: %+v", third)
	}
	if f.claim() != nil {
		t.Fatal("integration waits for the junior task")
	}
}

// Junior сам по себе не работает (решение 13.10.2026).
func TestJuniorWorksOnlyUnderContractInsideAPlan(t *testing.T) {
	lists := hostLists{caps: map[string]bool{}, envs: map[string]bool{"container": true}, skills: map[string]bool{},
		mcp: map[string]bool{"default": true}, executors: map[string]bool{"claude": true, "junior": true}}
	contract := &Contract{Files: []string{"lib/a.dart"}, Tests: []string{"test/a_test.dart"}}
	plan := func(junior PlanTask) Proposal {
		junior.Title, junior.Prompt, junior.Executor = "j", "j", "junior"
		return Proposal{Tasks: []PlanTask{{Title: "contract", Prompt: "tests"}, junior,
			{Title: "integrate", Prompt: "i", DependsOn: []int{1}}}}
	}
	ok := plan(PlanTask{DependsOn: []int{0}, Contract: contract})
	if err := ValidateProposal(&ok, lists); err != nil {
		t.Fatal(err)
	}
	bad := map[string]Proposal{
		"no contract":      plan(PlanTask{DependsOn: []int{0}}),
		"no tests":         plan(PlanTask{DependsOn: []int{0}, Contract: &Contract{Files: []string{"lib/a.dart"}}}),
		"path escapes":     plan(PlanTask{DependsOn: []int{0}, Contract: &Contract{Files: []string{"../x"}, Tests: []string{"t"}}}),
		"absolute path":    plan(PlanTask{DependsOn: []int{0}, Contract: &Contract{Files: []string{"C:/x"}, Tests: []string{"t"}}}),
		"no claude before": plan(PlanTask{Contract: contract}),
		"touches ui":       plan(PlanTask{DependsOn: []int{0}, Contract: contract, TouchesUI: true}),
		"no claude after": {Tasks: []PlanTask{{Title: "c", Prompt: "c"},
			{Title: "j", Prompt: "j", Executor: "junior", DependsOn: []int{0}, Contract: contract}}},
		"contract for claude": {Tasks: []PlanTask{{Title: "c", Prompt: "c", Contract: contract}}},
	}
	for name, p := range bad {
		if err := ValidateProposal(&p, lists); err == nil {
			t.Errorf("%s: must be rejected", name)
		}
	}

	f := newFx(t)
	f.registries(nil, nil, []string{"claude", "junior"})
	if _, err := f.svc.CreateTask(f.ctx, f.user, NewTask{RepositoryID: f.repo, Title: "t", Prompt: "p", Executor: "junior"}); !errors.Is(err, ErrJuniorOnlyInPlan) {
		t.Fatalf("direct junior task: %v", err)
	}
}

func TestSemiAutoPlanCreatesTasksAtOnce(t *testing.T) {
	f := newFx(t)
	f.registries([]string{"dart-clean-code", "flutter-widget-tests"}, []string{"web"}, []string{"claude", "junior"})
	id, c := f.plan("semi_auto")
	if st, _, _ := f.svc.SubmitPlan(f.ctx, f.host, id, f.worker, c.Attempt, goodProposal(), ""); st != PlanApproved {
		t.Fatal(st)
	}
	if tasks, _ := f.svc.ListTasks(f.ctx, f.user, nil); len(tasks) != 4 || tasks[0].Mode != "semi_auto" {
		t.Fatalf("tasks: %+v", tasks)
	}
}

// Manager предлагает — оркестратор проверяет (ТЗ п. 10.1).
func TestOrchestratorRejectsBadPlans(t *testing.T) {
	lists := hostLists{caps: map[string]bool{"flutter": true}, envs: map[string]bool{"container": true},
		skills: map[string]bool{"dart-clean-code": true, "secret-skill": true}, mcp: map[string]bool{"default": true},
		executors: map[string]bool{"claude": true}, forbidden: []string{"secret-skill"}}
	bad := map[string]Proposal{
		"empty":           {},
		"cycle":           {Tasks: []PlanTask{{Title: "a", Prompt: "a", DependsOn: []int{1}}, {Title: "b", Prompt: "b", DependsOn: []int{0}}}},
		"self dependency": {Tasks: []PlanTask{{Title: "a", Prompt: "a", DependsOn: []int{0}}}},
		"bad index":       {Tasks: []PlanTask{{Title: "a", Prompt: "a", DependsOn: []int{5}}}},
		"unknown cap":     {Tasks: []PlanTask{{Title: "a", Prompt: "a", RequiredCapabilities: []string{"roblox_studio"}}}},
		"unknown env":     {Tasks: []PlanTask{{Title: "a", Prompt: "a", ExecutionEnv: "windows_user"}}},
		"unknown skill":   {Tasks: []PlanTask{{Title: "a", Prompt: "a", RequiredSkills: []string{"hack-the-planet"}}}},
		"forbidden skill": {Tasks: []PlanTask{{Title: "a", Prompt: "a", RequiredSkills: []string{"secret-skill"}}}},
		"unknown mcp":     {Tasks: []PlanTask{{Title: "a", Prompt: "a", MCPProfile: "filesystem-root"}}},
		"unknown exec":    {Tasks: []PlanTask{{Title: "a", Prompt: "a", Executor: "gpt"}}},
		"empty prompt":    {Tasks: []PlanTask{{Title: "a", Prompt: " "}}},
		"bad review":      {Tasks: []PlanTask{{Title: "a", Prompt: "a", ReviewLevel: "ceo"}}},
	}
	tooMany := Proposal{}
	for i := 0; i <= MaxPlanTasks; i++ {
		tooMany.Tasks = append(tooMany.Tasks, PlanTask{Title: "t", Prompt: "p"})
	}
	bad["too many"] = tooMany
	for name, p := range bad {
		if err := ValidateProposal(&p, lists); err == nil {
			t.Errorf("%s: must be rejected", name)
		}
	}
	lists.required = []string{"dart-clean-code"}
	ok := Proposal{Tasks: []PlanTask{{Title: "a", Prompt: "a"}}}
	if err := ValidateProposal(&ok, lists); err != nil || strings.Join(ok.Tasks[0].RequiredSkills, ",") != "dart-clean-code" ||
		ok.Tasks[0].Executor != "claude" || ok.Tasks[0].MCPProfile != "default" {
		t.Fatalf("required skills and defaults must be applied: %+v %v", ok.Tasks[0], err)
	}
}

func TestInvalidPlanFailsWithReason(t *testing.T) {
	f := newFx(t)
	id, c := f.plan("semi_auto")
	p := &Proposal{Tasks: []PlanTask{{Title: "a", Prompt: "a", RequiredSkills: []string{"not-installed"}}}}
	st, reason, err := f.svc.SubmitPlan(f.ctx, f.host, id, f.worker, c.Attempt, p, "")
	if err != nil || st != PlanFailed || !strings.Contains(reason, "not-installed") {
		t.Fatalf("%s %q %v", st, reason, err)
	}
	if tasks, _ := f.svc.ListTasks(f.ctx, f.user, nil); len(tasks) != 0 {
		t.Fatal("invalid plan must not create tasks")
	}
}

func TestPlanOwnershipAndLease(t *testing.T) {
	f := newFx(t)
	id, c := f.plan("manual")
	if _, _, err := f.svc.SubmitPlan(f.ctx, f.host, id, f.worker, c.Attempt+1, goodProposal(), ""); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("stale attempt: %v", err)
	}
	// Manager пропал — план возвращается в очередь, после второй попытки — FAILED.
	f.clock = f.clock.Add(PlanLeaseTTL + time.Second)
	if n, _ := f.svc.ReapExpired(f.ctx); n != 1 {
		t.Fatal("expired plan lease must be reaped")
	}
	c2 := f.claim()
	if c2 == nil || c2.Kind != KindPlan || c2.Attempt != 2 {
		t.Fatalf("retry: %+v", c2)
	}
	f.clock = f.clock.Add(PlanLeaseTTL + time.Second)
	_, _ = f.svc.ReapExpired(f.ctx)
	plans, _ := f.svc.ListPlans(f.ctx, f.user)
	if plans[0].Status != PlanFailed {
		t.Fatal(plans[0].Status)
	}
}

func TestRejectPlan(t *testing.T) {
	f := newFx(t)
	f.registries([]string{"dart-clean-code", "flutter-widget-tests"}, []string{"web"}, []string{"claude", "junior"})
	id, c := f.plan("manual")
	_, _, _ = f.svc.SubmitPlan(f.ctx, f.host, id, f.worker, c.Attempt, goodProposal(), "")
	if err := f.svc.RejectPlan(f.ctx, f.user, id, "слишком мелко"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ApprovePlan(f.ctx, f.user, id); !errors.Is(err, ErrBadTransition) {
		t.Fatalf("rejected plan cannot be approved: %v", err)
	}
}

func TestJuniorTasksNeedAJuniorHost(t *testing.T) {
	f := newFx(t)
	// Задачи Junior появляются только из плана; здесь важна выдача хосту, поэтому исполнитель меняется в базе.
	id := f.task()
	f.exec(`UPDATE exec.tasks SET executor = 'junior', contract = '{"files":["a"],"tests":["b"]}' WHERE id = $1`, id)
	if f.claim() != nil {
		t.Fatal("host without Ollama must not get junior tasks")
	}
	f.registries(nil, nil, []string{"claude", "junior"})
	if c := f.claim(); c == nil || c.Executor != "junior" {
		t.Fatalf("%+v", c)
	}
}

func TestSkillPolicyAppliesToPlans(t *testing.T) {
	f := newFx(t)
	f.registries([]string{"dart-clean-code", "flutter-widget-tests"}, nil, nil)
	if err := f.svc.SetSkillPolicy(f.ctx, f.user, f.repo, []string{"dart-clean-code"}, []string{"flutter-widget-tests"}); err != nil {
		t.Fatal(err)
	}
	id, c := f.plan("semi_auto")
	p := &Proposal{Tasks: []PlanTask{{Title: "a", Prompt: "a", RequiredSkills: []string{"flutter-widget-tests"}}}}
	if st, reason, _ := f.svc.SubmitPlan(f.ctx, f.host, id, f.worker, c.Attempt, p, ""); st != PlanFailed || !strings.Contains(reason, "forbidden") {
		t.Fatalf("forbidden skill: %s %s", st, reason)
	}
	id, c = f.plan("semi_auto")
	if st, _, _ := f.svc.SubmitPlan(f.ctx, f.host, id, f.worker, c.Attempt, &Proposal{Tasks: []PlanTask{{Title: "a", Prompt: "a"}}}, ""); st != PlanApproved {
		t.Fatal(st)
	}
	if task := f.claim(); task == nil || strings.Join(task.RequiredSkills, ",") != "dart-clean-code" {
		t.Fatalf("required skill must be attached: %+v", task)
	}
}

func TestRequiredSkillsAttachToDirectTasks(t *testing.T) {
	f := newFx(t)
	if err := f.svc.SetSkillPolicy(f.ctx, f.user, f.repo, []string{"testing-strategy"}, nil); err != nil {
		t.Fatal(err)
	}
	f.task(func(n *NewTask) { n.RequiredSkills = []string{"dart-clean-code"} })
	if c := f.claim(); c == nil || strings.Join(c.RequiredSkills, ",") != "dart-clean-code,testing-strategy" {
		t.Fatalf("%+v", c)
	}
}
