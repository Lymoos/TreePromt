// Package audit — аудит хоста (ТЗ п. 14, docs/stage7-6-security-audit.md, раздел 3): каждая команда и
// каждый вызов модели (хэш промпта и снимок без секретов). Записи сначала пишутся в очередь на диске
// и досылаются на сервер пачками — обрыв сети их не теряет.
package audit

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"prompttree/aicrew/internal/runner"
	"prompttree/aicrew/internal/secretscan"
)

type Entry struct {
	TaskID  string          `json:"task_id,omitempty"`
	Attempt *int            `json:"attempt,omitempty"`
	Kind    string          `json:"kind"` // command | model | attach
	At      time.Time       `json:"at"`
	Data    json.RawMessage `json:"data"`
}

type Prompt struct {
	Hash string `json:"hash"`
	Text string `json:"text"`
}

type ctxKey struct{}

type taskInfo struct {
	id      string
	attempt int
}

// WithTask — команды в этом контексте относятся к задаче и попытке.
func WithTask(ctx context.Context, id string, attempt int) context.Context {
	return context.WithValue(ctx, ctxKey{}, taskInfo{id, attempt})
}

// Sender — сервер (api.Client.WriteAudit).
type Sender interface {
	WriteAudit(ctx context.Context, entries []Entry, prompts []Prompt) error
}

// Sink — очередь записей на диске.
type Sink struct {
	Dir     string
	Scanner *secretscan.Scanner
	mu      sync.Mutex
}

func (s *Sink) path(name string) string { return filepath.Join(s.Dir, name) }

func (s *Sink) append(name string, v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(s.path(name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(v)
	_, _ = f.Write(append(b, '\n'))
}

func (s *Sink) redact(v string) string {
	if s.Scanner == nil {
		return v
	}
	return s.Scanner.Redact(v)
}

// Add — запись о задаче из контекста (если есть). data проходит замену секретов.
func (s *Sink) Add(ctx context.Context, kind string, data map[string]any) {
	if s == nil {
		return
	}
	e := Entry{Kind: kind, At: time.Now().UTC()}
	if ti, ok := ctx.Value(ctxKey{}).(taskInfo); ok {
		e.TaskID = ti.id
		a := ti.attempt
		e.Attempt = &a
	}
	b, _ := json.Marshal(data)
	e.Data = json.RawMessage(s.redact(string(b)))
	if !json.Valid(e.Data) {
		e.Data = json.RawMessage(`{}`)
	}
	s.append("audit.jsonl", e)
}

// Snapshot сохраняет промпт без секретов и возвращает его хэш (хэш — от текста после замены).
func (s *Sink) Snapshot(text string) string {
	red := s.redact(text)
	sum := sha256.Sum256([]byte(red))
	h := hex.EncodeToString(sum[:])
	s.append("prompts.jsonl", Prompt{Hash: h, Text: red})
	return h
}

// Flush отправляет очередь пачками; отправленное удаляется, остальное ждёт следующего раза.
func (s *Sink) Flush(ctx context.Context, to Sender) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := readLines[Entry](s.path("audit.jsonl"))
	if err != nil {
		return err
	}
	prompts, err := readLines[Prompt](s.path("prompts.jsonl"))
	if err != nil {
		return err
	}
	for len(entries) > 0 || len(prompts) > 0 {
		ne, np := min(len(entries), 500), min(len(prompts), 100)
		if err := to.WriteAudit(ctx, entries[:ne], prompts[:np]); err != nil {
			_ = writeLines(s.path("audit.jsonl"), entries)
			_ = writeLines(s.path("prompts.jsonl"), prompts)
			return err
		}
		entries, prompts = entries[ne:], prompts[np:]
	}
	_ = os.Remove(s.path("audit.jsonl"))
	_ = os.Remove(s.path("prompts.jsonl"))
	return nil
}

// Run — фоновая досылка раз в interval.
func (s *Sink) Run(ctx context.Context, to Sender, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = s.Flush(context.Background(), to)
			return
		case <-t.C:
			_ = s.Flush(ctx, to)
		}
	}
}

func readLines[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []T
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	for sc.Scan() {
		var v T
		if json.Unmarshal(sc.Bytes(), &v) == nil {
			out = append(out, v)
		}
	}
	return out, sc.Err()
}

func writeLines[T any](path string, xs []T) error {
	if len(xs) == 0 {
		return os.Remove(path)
	}
	var b strings.Builder
	for _, x := range xs {
		j, _ := json.Marshal(x)
		b.Write(j)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

// Runner — обёртка: каждая команда хоста в аудит. Длинные аргументы обрезаются, текст на вход
// (промпт агента) сохраняется снимком, в записи — только его хэш.
type Runner struct {
	Inner runner.Runner
	Sink  *Sink
}

func (r Runner) Run(ctx context.Context, cmd runner.Command) (runner.Result, error) {
	res, err := r.Inner.Run(ctx, cmd)
	args := make([]string, len(cmd.Args))
	for i, a := range cmd.Args {
		if len(a) > 300 {
			a = a[:300] + "…"
		}
		args[i] = a
	}
	data := map[string]any{"exe": cmd.Exe, "args": args, "dir": cmd.Dir, "exit": res.ExitCode,
		"ms": res.Duration.Milliseconds(), "timed_out": res.TimedOut}
	if err != nil {
		data["error"] = err.Error()
	}
	kind := "command"
	if cmd.Stdin != "" {
		data["prompt_sha256"] = r.Sink.Snapshot(cmd.Stdin)
	}
	if isModelCall(cmd) {
		kind = "model"
		for i, a := range cmd.Args {
			if a == "--model" && i+1 < len(cmd.Args) {
				data["model"] = cmd.Args[i+1]
			}
		}
	}
	r.Sink.Add(ctx, kind, data)
	return res, err
}

func isModelCall(cmd runner.Command) bool {
	if strings.HasSuffix(strings.ToLower(cmd.Exe), "claude.exe") {
		return true
	}
	for _, a := range cmd.Args {
		if a == "claude" {
			return true
		}
	}
	return false
}
