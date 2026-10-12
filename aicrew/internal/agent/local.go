package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"prompttree/aicrew/internal/runner"
)

// Local — собственный исполнитель AiCrew для локальных моделей (Ollama). Разговор с моделью ведёт
// служба через родной API Ollama (/api/chat с инструментами), а не чужой клиент: так AiCrew остаётся
// переходником между моделями и сам чинит то, что модель прислала не по формату.
//
// Файлы правятся прямо в worktree задачи (только внутри него); команды выполняются в одноразовом
// контейнере, куда смонтирован тот же worktree, — как у Claude Code.
type Local struct {
	R       runner.Runner // docker
	Image   string        // образ для команд
	BaseURL string        // Ollama, как её видит служба: http://127.0.0.1:11434
	Model   string
	NumCtx  int // окно контекста, токенов (options.num_ctx)
	Network string
	HTTP    *http.Client

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func (l *Local) Name() string { return "local/" + l.Model }

const (
	localMaxRead    = 24 << 10 // байт из одного файла за раз
	localMaxOutput  = 12 << 10 // хвост вывода команды
	localMaxMatches = 120
	localMaxFiles   = 400
)

var skipDirs = map[string]bool{".git": true, ".dart_tool": true, "build": true, "node_modules": true, ".idea": true}

func (l *Local) Run(ctx context.Context, req Request) (Result, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}
	name := req.Name
	if name == "" {
		name = ContainerName(req.TaskID, req.Attempt)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	l.track(name, cancel)
	defer l.track(name, nil)

	s := &localSession{l: l, req: req, container: name}
	defer s.stopContainer()

	maxTurns := req.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 60
	}
	msgs := []chatMessage{{Role: "system", Content: s.systemPrompt()}, {Role: "user", Content: s.taskPrompt()}}
	var usage Usage
	for turn := 1; turn <= maxTurns; turn++ {
		usage.Turns = turn
		reply, err := l.chat(ctx, trimHistory(msgs, l.NumCtx), s.tools())
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return Result{Usage: usage}, &Failure{Class: "TIMEOUT", Code: "agent_runtime", Message: fmt.Sprintf("agent ran longer than %s", req.Timeout)}
			}
			if ctx.Err() != nil {
				return Result{Usage: usage}, &Failure{Class: "AGENT_ERROR", Code: "stopped", Message: "stopped"}
			}
			return Result{Usage: usage}, &Failure{Class: "ENVIRONMENT_FAILURE", Code: "local_model", Message: err.Error()}
		}
		usage.InputTokens += reply.PromptEvalCount
		usage.OutputTokens += reply.EvalCount
		msg := reply.Message
		calls := msg.ToolCalls
		if len(calls) == 0 {
			calls = RepairToolCalls(msg.Content) // вызов текстом: Qwen иногда теряет обёртку
		}
		msgs = append(msgs, chatMessage{Role: "assistant", Content: msg.Content, ToolCalls: calls})
		if len(calls) == 0 {
			return Result{Summary: strings.TrimSpace(msg.Content), Usage: usage}, nil
		}
		for _, c := range calls {
			if c.Function.Name == "finish" {
				return Result{Summary: argString(c.Function.Arguments, "summary"), Usage: usage}, nil
			}
			out := s.call(ctx, c.Function.Name, c.Function.Arguments)
			msgs = append(msgs, chatMessage{Role: "tool", ToolName: c.Function.Name, Content: out})
		}
	}
	return Result{Usage: usage}, &Failure{Class: "AGENT_ERROR", Code: "max_turns", Message: "agent hit the turn limit"}
}

func (l *Local) Stop(ctx context.Context, taskID string, attempt int) error {
	name := ContainerName(taskID, attempt)
	l.mu.Lock()
	if c := l.cancels[name]; c != nil {
		c()
	}
	l.mu.Unlock()
	_, err := l.R.Run(ctx, runner.Command{Exe: "docker", Args: []string{"rm", "-f", name}, Timeout: 30 * time.Second})
	return err
}

func (l *Local) track(name string, cancel context.CancelFunc) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cancels == nil {
		l.cancels = map[string]context.CancelFunc{}
	}
	if cancel == nil {
		delete(l.cancels, name)
	} else {
		l.cancels[name] = cancel
	}
}

// ── Ollama /api/chat ──

type chatMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolName  string     `json:"tool_name,omitempty"`
}

type ToolCall struct {
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

type chatReply struct {
	Message         chatMessage `json:"message"`
	PromptEvalCount int64       `json:"prompt_eval_count"`
	EvalCount       int64       `json:"eval_count"`
	Error           string      `json:"error"`
}

func (l *Local) chat(ctx context.Context, msgs []chatMessage, tools []map[string]any) (chatReply, error) {
	opts := map[string]any{"temperature": 0.2}
	if l.NumCtx > 0 {
		opts["num_ctx"] = l.NumCtx
	}
	body, _ := json.Marshal(map[string]any{"model": l.Model, "messages": msgs, "tools": tools, "stream": false, "options": opts})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(l.BaseURL, "/")+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return chatReply{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	hc := l.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return chatReply{}, err
	}
	defer resp.Body.Close()
	var r chatReply
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&r); err != nil {
		return chatReply{}, fmt.Errorf("ollama: HTTP %d, %v", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK || r.Error != "" {
		return chatReply{}, fmt.Errorf("ollama: HTTP %d %s", resp.StatusCode, r.Error)
	}
	return r, nil
}

// trimHistory — старые результаты инструментов сокращаются, когда разговор не влезает в окно
// (грубо: ~3 символа на токен). Системный промпт, задача и последние шаги не трогаются.
func trimHistory(msgs []chatMessage, numCtx int) []chatMessage {
	if numCtx <= 0 {
		return msgs
	}
	budget := numCtx * 3 * 3 / 4 // четверть окна — на ответ модели
	total := 0
	for _, m := range msgs {
		total += len(m.Content)
	}
	out := append([]chatMessage(nil), msgs...)
	for i := 2; i < len(out)-6 && total > budget; i++ {
		if out[i].Role == "tool" && len(out[i].Content) > 200 {
			total -= len(out[i].Content) - 60
			out[i].Content = out[i].Content[:60] + " …[trimmed: old tool output]"
		}
	}
	return out
}

// ── ремонт вызовов, присланных текстом ──

var (
	xmlCall  = regexp.MustCompile(`(?s)<function=([\w.-]+)>(.*?)(?:</function>|$)`)
	xmlParam = regexp.MustCompile(`(?s)<parameter=([\w.-]+)>\n?(.*?)\n?</parameter>`)
	jsonCall = regexp.MustCompile(`(?s)\{\s*"name"\s*:\s*"([\w.-]+)"\s*,\s*"arguments"\s*:\s*(\{.*\})\s*\}`)
)

// RepairToolCalls достаёт вызовы инструментов из текста ответа: формат Qwen3-Coder
// (<function=Write><parameter=path>…</parameter></function>, иногда без обёртки <tool_call>)
// и JSON {"name": …, "arguments": {…}} (Qwen2.5).
func RepairToolCalls(text string) []ToolCall {
	var calls []ToolCall
	for _, m := range xmlCall.FindAllStringSubmatch(text, -1) {
		args := map[string]any{}
		for _, p := range xmlParam.FindAllStringSubmatch(m[2], -1) {
			args[p[1]] = p[2]
		}
		var c ToolCall
		c.Function.Name, c.Function.Arguments = m[1], args
		calls = append(calls, c)
	}
	if len(calls) > 0 {
		return calls
	}
	for _, m := range jsonCall.FindAllStringSubmatch(text, -1) {
		args := map[string]any{}
		if json.Unmarshal([]byte(m[2]), &args) != nil {
			continue
		}
		var c ToolCall
		c.Function.Name, c.Function.Arguments = m[1], args
		calls = append(calls, c)
	}
	return calls
}

// ── сессия задачи: промпт, инструменты, их выполнение ──

type localSession struct {
	l         *Local
	req       Request
	container string
	started   bool
}

func (s *localSession) systemPrompt() string {
	var b strings.Builder
	b.WriteString("You are Junior, a careful software engineer in the AiCrew team. You change the repository in the current " +
		"directory by calling tools. Work in small steps: look at the relevant files first, then edit, then run the checks.\n" +
		"Rules:\n" +
		"- Act only through tool calls. Paths are relative to the repository root.\n" +
		"- Prefer edit_file for small changes to existing files; write_file for new files or full rewrites.\n" +
		"- Keep the existing style of the code. Do not touch files unrelated to the task.\n" +
		"- A check passes only when run returns exit code 0. If a command is not found or fails, fix the cause; never claim it passed.\n" +
		"- When the work is done and the checks pass, call finish with a short summary of what you changed.\n")
	if s.req.ReadOnly {
		b.WriteString("- This is a read-only review: you cannot change files.\n")
	}
	for _, sk := range s.skills() {
		b.WriteString("\n---\nSkill " + sk.name + " (follow it):\n" + sk.body + "\n")
	}
	return b.String()
}

func (s *localSession) taskPrompt() string {
	if s.req.Raw {
		return s.req.Prompt
	}
	return BuildPrompt(s.req)
}

type skillText struct{ name, body string }

// skills — тексты SKILL.md из смонтированного каталога скиллов задачи (Claude Code читает их сам, здесь — промпт).
func (s *localSession) skills() []skillText {
	var out []skillText
	for _, m := range s.req.Mounts {
		if m.Container != SkillsMount {
			continue
		}
		entries, _ := os.ReadDir(m.Host)
		for _, e := range entries {
			b, err := os.ReadFile(filepath.Join(m.Host, e.Name(), "SKILL.md"))
			if err == nil {
				out = append(out, skillText{e.Name(), string(b)})
			}
		}
	}
	return out
}

func tool(name, desc string, props map[string]string, required ...string) map[string]any {
	p := map[string]any{}
	for k, d := range props {
		p[k] = map[string]any{"type": "string", "description": d}
	}
	return map[string]any{"type": "function", "function": map[string]any{"name": name, "description": desc,
		"parameters": map[string]any{"type": "object", "properties": p, "required": required}}}
}

func (s *localSession) tools() []map[string]any {
	t := []map[string]any{
		tool("read_file", "Read a text file. Returns numbered lines.", map[string]string{
			"path": "file path", "offset": "first line (1-based, optional)", "limit": "number of lines (optional)"}, "path"),
		tool("list_files", "List files under a directory (recursive, skips .git and build output).",
			map[string]string{"path": "directory, default ."}),
		tool("search", "Search file contents with a regular expression. Returns path:line: text.",
			map[string]string{"pattern": "regular expression", "path": "directory or file, default ."}, "pattern"),
		tool("run", "Run a shell command in the repository (sandbox container). Use it for tests, analyzers, builds.",
			map[string]string{"command": "shell command"}, "command"),
		tool("finish", "Finish the task with a short summary of what you changed.", map[string]string{"summary": "summary"}, "summary"),
	}
	if !s.req.ReadOnly {
		t = append(t,
			tool("write_file", "Create or overwrite a file with the given content.",
				map[string]string{"path": "file path", "content": "full file content"}, "path", "content"),
			tool("edit_file", "Replace an exact, unique fragment of a file.",
				map[string]string{"path": "file path", "old_string": "exact text to replace (must occur once)", "new_string": "replacement"},
				"path", "old_string", "new_string"))
	}
	return t
}

func argString(args map[string]any, k string) string {
	switch v := args[k].(type) {
	case string:
		return v
	case nil:
		return ""
	case float64:
		return fmt.Sprintf("%g", v)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// call выполняет инструмент; ошибки возвращаются модели текстом — пусть исправится сама.
func (s *localSession) call(ctx context.Context, name string, args map[string]any) string {
	out, err := s.dispatch(ctx, name, args)
	if err != nil {
		return "ERROR: " + err.Error()
	}
	return out
}

func (s *localSession) dispatch(ctx context.Context, name string, args map[string]any) (string, error) {
	switch name {
	case "read_file":
		return s.readFile(argString(args, "path"), argString(args, "offset"), argString(args, "limit"))
	case "list_files":
		return s.listFiles(argString(args, "path"))
	case "search":
		return s.search(argString(args, "pattern"), argString(args, "path"))
	case "run":
		return s.run(ctx, argString(args, "command"))
	case "write_file", "edit_file":
		if s.req.ReadOnly {
			return "", errors.New("read-only task: files cannot be changed")
		}
		if name == "write_file" {
			return s.writeFile(argString(args, "path"), argString(args, "content"))
		}
		return s.editFile(argString(args, "path"), argString(args, "old_string"), argString(args, "new_string"))
	}
	return "", fmt.Errorf("unknown tool %q; available: read_file, list_files, search, run, write_file, edit_file, finish", name)
}

// resolve — путь внутри worktree; выход за его пределы (.., абсолютные пути, ссылки наружу) запрещён.
func (s *localSession) resolve(p string) (string, error) {
	root, err := filepath.Abs(s.req.Workdir)
	if err != nil {
		return "", err
	}
	p = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(p)), "/work/")
	if p == "" || p == "/work" {
		p = "."
	}
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("use paths relative to the repository root, not %q", p)
	}
	full := filepath.Join(root, filepath.FromSlash(p))
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%q is outside the repository", p)
	}
	if real, err := filepath.EvalSymlinks(full); err == nil {
		if rr, err := filepath.EvalSymlinks(root); err == nil {
			if r2, err := filepath.Rel(rr, real); err != nil || strings.HasPrefix(r2, "..") {
				return "", fmt.Errorf("%q points outside the repository", p)
			}
		}
	}
	if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
		return "", errors.New("the .git directory is managed by the orchestrator")
	}
	return full, nil
}

func (s *localSession) readFile(p, offset, limit string) (string, error) {
	full, err := s.resolve(p)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	from, n := 1, len(lines)
	fmt.Sscan(offset, &from)
	fmt.Sscan(limit, &n)
	if from < 1 {
		from = 1
	}
	var out strings.Builder
	for i := from - 1; i < len(lines) && i < from-1+n; i++ {
		if out.Len() > localMaxRead {
			fmt.Fprintf(&out, "…[file continues: read_file with offset=%d]\n", i+1)
			break
		}
		fmt.Fprintf(&out, "%5d  %s\n", i+1, lines[i])
	}
	return out.String(), nil
}

func (s *localSession) writeFile(p, content string) (string, error) {
	full, err := s.resolve(p)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %s (%d lines)", p, strings.Count(content, "\n")+1), nil
}

func (s *localSession) editFile(p, oldS, newS string) (string, error) {
	full, err := s.resolve(p)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	text := string(b)
	crlf := strings.Contains(text, "\r\n")
	if crlf {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	switch n := strings.Count(text, oldS); {
	case oldS == "":
		return "", errors.New("old_string is empty; use write_file to create a file")
	case n == 0:
		return "", errors.New("old_string not found; read the file again and copy the exact text")
	case n > 1:
		return "", fmt.Errorf("old_string occurs %d times; include more surrounding lines to make it unique", n)
	}
	text = strings.Replace(text, oldS, newS, 1)
	if crlf {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
		return "", err
	}
	return "edited " + p, nil
}

func (s *localSession) walk(dir string, fn func(rel string, full string) bool) error {
	root, _ := filepath.Abs(s.req.Workdir)
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if !fn(filepath.ToSlash(rel), path) {
			return filepath.SkipAll
		}
		return nil
	})
}

func (s *localSession) listFiles(p string) (string, error) {
	dir, err := s.resolve(p)
	if err != nil {
		return "", err
	}
	var files []string
	err = s.walk(dir, func(rel, _ string) bool {
		files = append(files, rel)
		return len(files) < localMaxFiles
	})
	sort.Strings(files)
	out := strings.Join(files, "\n")
	if len(files) >= localMaxFiles {
		out += "\n…[more files: list a subdirectory]"
	}
	return out, err
}

func (s *localSession) search(pattern, p string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("bad regular expression: %v", err)
	}
	dir, err := s.resolve(p)
	if err != nil {
		return "", err
	}
	var out []string
	err = s.walk(dir, func(rel, full string) bool {
		b, err := os.ReadFile(full)
		if err != nil || bytes.IndexByte(b[:min(len(b), 8000)], 0) >= 0 {
			return true // бинарные файлы пропускаются
		}
		for i, line := range strings.Split(string(b), "\n") {
			if re.MatchString(line) {
				out = append(out, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
				if len(out) >= localMaxMatches {
					return false
				}
			}
		}
		return true
	})
	if len(out) == 0 {
		return "no matches", err
	}
	return strings.Join(out, "\n"), err
}

// run — команда в контейнере задачи; контейнер поднимается при первой команде и живёт до конца попытки.
func (s *localSession) run(ctx context.Context, command string) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", errors.New("empty command")
	}
	if !s.started {
		mount := s.req.Workdir + ":/work"
		if s.req.ReadOnly {
			mount += ":ro"
		}
		args := []string{"run", "-d", "--rm", "--name", s.container, "--memory", "6g", "--cpus", "4", "--pids-limit", "512",
			"-v", mount, "-w", "/work"}
		if s.l.Network != "" {
			args = append(args, "--network", s.l.Network)
		}
		args = append(args, s.l.Image, "sleep", "infinity")
		res, err := s.l.R.Run(ctx, runner.Command{Exe: "docker", Args: args, Timeout: 2 * time.Minute})
		if err != nil || res.ExitCode != 0 {
			return "", fmt.Errorf("sandbox did not start: %v %s", err, tail(res.Stderr, 300))
		}
		s.started = true
	}
	// bash -c, не -lc: login-оболочка сбрасывает PATH образа, и flutter «не находится».
	res, err := s.l.R.Run(ctx, runner.Command{Exe: "docker", Args: []string{"exec", "-w", "/work", s.container, "bash", "-c", command},
		Timeout: 15 * time.Minute})
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
	if res.TimedOut {
		out += "\n[timed out after 15 minutes]"
	}
	return fmt.Sprintf("exit code %d\n%s", res.ExitCode, tail(out, localMaxOutput)), nil
}

func (s *localSession) stopContainer() {
	if !s.started {
		return
	}
	_, _ = s.l.R.Run(context.Background(), runner.Command{Exe: "docker", Args: []string{"rm", "-f", s.container}, Timeout: 30 * time.Second})
}
