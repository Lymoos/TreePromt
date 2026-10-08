package host

// Сквозной тест с настоящим сервером PromptTree (ALLOW_REGISTRATION=true):
//   AICREW_E2E_SERVER=http://127.0.0.1:58081 go test ./internal/host -run E2E -v
// Агент — имитация (меняет файл), git — настоящий; проверяются протокол, lease, статусы и факты.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/gitx"
	"prompttree/aicrew/internal/runner"
	"prompttree/aicrew/internal/verify"
)

type memTokens map[string]string

func (m memTokens) Get(k string) (string, error) { return m[k], nil }
func (m memTokens) Set(k, v string) error        { m[k] = v; return nil }

func call(t *testing.T, c *api.Client, method, path string, body, out any) int {
	t.Helper()
	tok, err := c.AccessToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, c.Base+"/api/v1"+path, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if out != nil {
		_ = json.Unmarshal(data, out)
	}
	if resp.StatusCode >= 400 {
		t.Fatalf("%s %s: %d %s", method, path, resp.StatusCode, data)
	}
	return resp.StatusCode
}

func taskStatus(t *testing.T, c *api.Client, id string) map[string]any {
	var out struct {
		Tasks []map[string]any `json:"tasks"`
	}
	call(t, c, http.MethodGet, "/exec/tasks", nil, &out)
	for _, tk := range out.Tasks {
		if tk["id"] == id {
			return tk
		}
	}
	t.Fatalf("task %s not found", id)
	return nil
}

func TestE2EHostAgainstRealServer(t *testing.T) {
	server := os.Getenv("AICREW_E2E_SERVER")
	if server == "" {
		t.Skip("AICREW_E2E_SERVER is not set")
	}
	ctx := context.Background()
	email, pass := "host-"+uuid.NewString()+"@example.com", "длинный-пароль-1"
	b, _ := json.Marshal(map[string]string{"email": email, "password": pass})
	if resp, err := http.Post(server+"/api/v1/auth/register", "application/json", bytes.NewReader(b)); err != nil || resp.StatusCode != 201 {
		t.Fatalf("register: %v %v", resp, err)
	}
	c := api.New(server, memTokens{})
	if err := c.Login(ctx, email, pass, uuid.NewString(), "test-host"); err != nil {
		t.Fatal(err)
	}

	// Проект PromptTree — через обычную синхронизацию.
	project := uuid.NewString()
	call(t, c, http.MethodPost, "/sync/push", map[string]any{"operations": []map[string]any{{
		"operation_id": uuid.NewString(), "client_seq": 1, "type": "create_project", "entity_id": project,
		"payload": map[string]string{"name": "Полигон"}}}}, nil)

	h, err := c.RegisterHost(ctx, "test-host", "windows", []string{"flutter"}, []string{"container"},
		[]api.WorkerSpec{{Kind: "claude_code", Model: "sonnet"}})
	if err != nil {
		t.Fatal(err)
	}
	repoPath := gitRepo(t)
	repo, err := c.AddRepository(ctx, project, h.ID, "todo", repoPath, "main", nil)
	if err != nil {
		t.Fatal(err)
	}

	newTask := func(title string) string {
		var out struct {
			ID string `json:"id"`
		}
		call(t, c, http.MethodPost, "/exec/tasks", map[string]any{"repository_id": repo.ID, "title": title,
			"prompt": "Act as a Flutter dev.\n\n" + title}, &out)
		return out.ID
	}
	worker := h.Workers["claude_code/sonnet"]
	base := &Host{API: c, Git: gitx.Git{R: runner.New("git")}, WorkerID: worker,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), PollEvery: 50 * time.Millisecond, HeartbeatEvery: 100 * time.Millisecond,
		Verifier: fakeVerifier{rep: verify.Report{Passed: true}}}

	// 1. Успех: задача доходит до MERGEABLE, факты на сервере.
	first := newTask("Добавить список задач")
	base.Agent = &fakeAgent{run: writes("todo_list.dart", "// список\n")}
	tk, err := c.Claim(ctx, worker)
	if err != nil || tk == nil || tk.ID != first {
		t.Fatalf("claim: %+v %v", tk, err)
	}
	base.Process(ctx, tk)
	st := taskStatus(t, c, first)
	if st["status"] != "MERGEABLE" || st["result_commit"] == nil || st["base_commit"] == nil {
		t.Fatalf("first task: %v", st)
	}

	// 2. Отмена во время работы агента: пользователь жмёт «Отменить», хост узнаёт из heartbeat.
	second := newTask("Добавить удаление")
	base.Agent = &fakeAgent{run: func(ctx context.Context, _ agent.Request) (agent.Result, error) {
		call(t, c, http.MethodPost, "/exec/tasks/"+second+"/cancel", nil, nil)
		<-ctx.Done()
		return agent.Result{}, ctx.Err()
	}}
	tk, _ = c.Claim(ctx, worker)
	base.Process(ctx, tk)
	if st := taskStatus(t, c, second); st["status"] != "CANCELLED" {
		t.Fatalf("second task: %v", st)
	}

	// 3. Красные тесты: FAILED → политика повторов возвращает в очередь со следующей попыткой.
	third := newTask("Сломать тест")
	base.Agent = &fakeAgent{run: writes("broken.dart", "x")}
	base.Verifier = fakeVerifier{rep: verify.Report{Passed: false, FailedKind: "test",
		Steps: []verify.StepResult{{Name: "flutter test", ExitCode: 1, Output: "1 test failed"}}}}
	tk, _ = c.Claim(ctx, worker)
	base.Process(ctx, tk)
	st = taskStatus(t, c, third)
	if st["status"] != "QUEUED" || st["failure_class"] != "TEST_FAILURE" || st["attempt"].(float64) != 1 {
		t.Fatalf("third task: %v", st)
	}
	tk, _ = c.Claim(ctx, worker)
	if tk == nil || tk.ID != third || tk.Attempt != 2 || len(tk.PreviousAttempts) != 1 {
		t.Fatalf("retry claim: %+v", tk)
	}

	// 4. Уборка: worktree первой задачи нужен для слияния, остальные — нет.
	keep, err := c.KeepWorktrees(ctx)
	if err != nil || !keep[first] || keep[second] {
		t.Fatalf("keep: %v %v", keep, err)
	}

	// 5. Этап 7.3: «Слить» → очередь слияния → integration → DONE; main не тронута.
	mainBefore := git(t, repoPath, "rev-parse", "main")
	call(t, c, http.MethodPost, "/exec/tasks/"+first+"/merge", nil, nil)
	base.Verifier = fakeVerifier{rep: verify.Report{Passed: true}}
	tk, err = c.Claim(ctx, worker)
	if err != nil || tk == nil || tk.ID != first || tk.Kind != api.KindMerge {
		t.Fatalf("merge claim: %+v %v", tk, err)
	}
	base.Process(ctx, tk)
	st = taskStatus(t, c, first)
	integ := git(t, repoPath, "rev-parse", "aicrew/integration")
	if st["status"] != "DONE" || st["merge_commit"] != integ || git(t, repoPath, "rev-parse", "main") != mainBefore {
		t.Fatalf("merged task: %v (integration %s)", st, integ)
	}

	// 6. Откат через очередь: revert с проверкой, integration без задачи.
	call(t, c, http.MethodPost, "/exec/tasks/"+first+"/rollback", map[string]string{"reason": "e2e"}, nil)
	tk, _ = c.Claim(ctx, worker)
	if tk == nil || tk.Kind != api.KindRollback || tk.MergeCommit != integ {
		t.Fatalf("rollback claim: %+v", tk)
	}
	base.Process(ctx, tk)
	if st := taskStatus(t, c, first); st["status"] != "ROLLED_BACK" {
		t.Fatalf("rolled back: %v", st)
	}
	if _, ok := fileAt(t, repoPath, "aicrew/integration", "todo_list.dart"); ok {
		t.Fatal("rollback must remove the task from integration")
	}
}
