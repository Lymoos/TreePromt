package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"prompttree/aicrew/internal/runner"
)

// fakeOllama отвечает заранее заданными сообщениями и запоминает запросы.
type fakeOllama struct {
	mu       sync.Mutex
	replies  []string // JSON поля message
	requests []map[string]any
}

func (f *fakeOllama) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	_ = json.NewDecoder(r.Body).Decode(&req)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	msg := `{"role":"assistant","content":"done"}`
	if len(f.replies) > 0 {
		msg, f.replies = f.replies[0], f.replies[1:]
	}
	w.Write([]byte(`{"message":` + msg + `,"prompt_eval_count":100,"eval_count":10}`))
}

// lastToolOutputs — что служба вернула модели результатами инструментов в последнем запросе.
func (f *fakeOllama) lastToolOutputs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, m := range f.requests[len(f.requests)-1]["messages"].([]any) {
		if mm := m.(map[string]any); mm["role"] == "tool" {
			out = append(out, mm["content"].(string))
		}
	}
	return out
}

type fakeDocker struct {
	mu   sync.Mutex
	cmds [][]string
}

func (d *fakeDocker) Run(_ context.Context, c runner.Command) (runner.Result, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cmds = append(d.cmds, c.Args)
	if c.Args[0] == "exec" {
		return runner.Result{Stdout: "All tests passed!"}, nil
	}
	return runner.Result{}, nil
}

func call(name string, args map[string]any) string {
	b, _ := json.Marshal(map[string]any{"role": "assistant", "content": "",
		"tool_calls": []any{map[string]any{"function": map[string]any{"name": name, "arguments": args}}}})
	return string(b)
}

func text(s string) string {
	b, _ := json.Marshal(map[string]any{"role": "assistant", "content": s})
	return string(b)
}

func newLocal(t *testing.T, replies ...string) (*Local, *fakeOllama, *fakeDocker, string) {
	t.Helper()
	fo := &fakeOllama{replies: replies}
	srv := httptest.NewServer(fo)
	t.Cleanup(srv.Close)
	fd := &fakeDocker{}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.dart"), []byte("void main() {\r\n  print('old');\r\n}\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return &Local{R: fd, Image: "img", BaseURL: srv.URL, Model: "qwen", NumCtx: 32768}, fo, fd, dir
}

func TestLocalAgentEditsFilesRunsChecksAndFinishes(t *testing.T) {
	l, fo, fd, dir := newLocal(t,
		call("read_file", map[string]any{"path": "main.dart"}),
		call("edit_file", map[string]any{"path": "main.dart", "old_string": "print('old');", "new_string": "print('new');"}),
		// Qwen3-Coder иногда присылает вызов текстом и без открывающего <tool_call>.
		text("I'll add notes.\n\n<function=write_file>\n<parameter=path>\ndocs/NOTES.md\n</parameter>\n<parameter=content>\nhello\n</parameter>\n</function>\n</tool_call>"),
		call("run", map[string]any{"command": "flutter test"}),
		call("run", map[string]any{"command": "flutter analyze"}),
		call("finish", map[string]any{"summary": "printed new"}),
	)
	res, err := l.Run(context.Background(), Request{TaskID: "t1", Attempt: 2, Workdir: dir, Prompt: "Change the message"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "printed new" || res.Usage.Turns != 6 || res.Usage.InputTokens != 600 {
		t.Fatalf("%+v", res)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "main.dart")); string(b) != "void main() {\r\n  print('new');\r\n}\r\n" {
		t.Fatalf("edit must keep CRLF: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "docs", "NOTES.md")); string(b) != "hello" {
		t.Fatalf("repaired text call must run: %q", b)
	}
	outs := fo.lastToolOutputs()
	if !strings.Contains(outs[0], "print('old');") || !strings.Contains(outs[len(outs)-1], "All tests passed!") {
		t.Fatalf("tool outputs: %q", outs)
	}
	// Один контейнер на попытку: поднят один раз, две команды, удалён в конце.
	var kinds []string
	for _, c := range fd.cmds {
		kinds = append(kinds, c[0])
	}
	if strings.Join(kinds, ",") != "run,exec,exec,rm" || fd.cmds[0][len(fd.cmds[0])-1] != "infinity" ||
		!strings.Contains(strings.Join(fd.cmds[0], " "), "aicrew-task-t1-2") ||
		strings.Join(fd.cmds[1][len(fd.cmds[1])-3:], " ") != "bash -c flutter test" {
		t.Fatalf("docker: %v", fd.cmds)
	}
	first := fo.requests[0]
	if first["options"].(map[string]any)["num_ctx"] != float64(32768) || first["stream"] != false {
		t.Fatalf("request: %v", first["options"])
	}
}

func TestLocalAgentStaysInsideTheWorktree(t *testing.T) {
	l, fo, _, dir := newLocal(t,
		call("write_file", map[string]any{"path": "../escape.txt", "content": "x"}),
		call("write_file", map[string]any{"path": "/etc/passwd", "content": "x"}),
		call("read_file", map[string]any{"path": ".git/config"}),
		call("edit_file", map[string]any{"path": "main.dart", "old_string": "nope", "new_string": "x"}),
		call("delete_everything", map[string]any{}),
		text("I could not do it."),
	)
	res, err := l.Run(context.Background(), Request{TaskID: "t", Attempt: 1, Workdir: dir, Prompt: "p"})
	if err != nil || res.Summary != "I could not do it." {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.txt")); err == nil {
		t.Fatal("wrote outside the worktree")
	}
	outs := fo.lastToolOutputs()
	for i, want := range []string{"outside the repository", "relative", ".git", "not found", "unknown tool"} {
		if !strings.HasPrefix(outs[i], "ERROR") || !strings.Contains(outs[i], want) {
			t.Errorf("output %d: %q (want %q)", i, outs[i], want)
		}
	}
}

func TestLocalAgentReadOnlyAndSkills(t *testing.T) {
	l, fo, _, dir := newLocal(t, call("write_file", map[string]any{"path": "a.txt", "content": "x"}), text("ok"))
	skills := t.TempDir()
	os.MkdirAll(filepath.Join(skills, "testing-strategy"), 0o755)
	os.WriteFile(filepath.Join(skills, "testing-strategy", "SKILL.md"), []byte("Never depend on the wall clock."), 0o644)
	_, err := l.Run(context.Background(), Request{TaskID: "t", Attempt: 1, Workdir: dir, Prompt: "review", ReadOnly: true,
		Mounts: []Mount{{Host: skills, Container: SkillsMount, ReadOnly: true}}})
	if err != nil {
		t.Fatal(err)
	}
	sys := fo.requests[0]["messages"].([]any)[0].(map[string]any)["content"].(string)
	if !strings.Contains(sys, "Skill testing-strategy") || !strings.Contains(sys, "wall clock") {
		t.Fatalf("skills must be in the system prompt: %s", sys)
	}
	for _, tl := range fo.requests[0]["tools"].([]any) {
		if n := tl.(map[string]any)["function"].(map[string]any)["name"]; n == "write_file" || n == "edit_file" {
			t.Fatal("read-only task must not offer write tools")
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); err == nil {
		t.Fatal("read-only task wrote a file")
	}
}

func TestLocalAgentTurnLimitAndModelDown(t *testing.T) {
	l, _, _, dir := newLocal(t, call("list_files", nil), call("list_files", nil), call("list_files", nil))
	_, err := l.Run(context.Background(), Request{TaskID: "t", Attempt: 1, Workdir: dir, Prompt: "p", MaxTurns: 2})
	if f, ok := err.(*Failure); !ok || f.Code != "max_turns" {
		t.Fatalf("%v", err)
	}
	l.BaseURL = "http://127.0.0.1:1"
	_, err = l.Run(context.Background(), Request{TaskID: "t", Attempt: 1, Workdir: dir, Prompt: "p"})
	if f, ok := err.(*Failure); !ok || f.Class != "ENVIRONMENT_FAILURE" || f.Code != "local_model" {
		t.Fatalf("%v", err)
	}
}

func TestRepairToolCalls(t *testing.T) {
	qwen3 := "Let me check.\n<function=read_file>\n<parameter=path>\nlib/main.dart\n</parameter>\n</function>\n</tool_call>"
	if c := RepairToolCalls(qwen3); len(c) != 1 || c[0].Function.Name != "read_file" || c[0].Function.Arguments["path"] != "lib/main.dart" {
		t.Fatalf("%+v", c)
	}
	qwen25 := `{"name": "write_file", "arguments": {"path": "a.txt", "content": "{x}"}}`
	if c := RepairToolCalls(qwen25); len(c) != 1 || c[0].Function.Arguments["content"] != "{x}" {
		t.Fatalf("%+v", c)
	}
	if c := RepairToolCalls("All done, the tests pass."); len(c) != 0 {
		t.Fatalf("%+v", c)
	}
}
