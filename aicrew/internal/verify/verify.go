// Package verify — детерминированная проверка результата (ТЗ п. 9, шаг 1): команды профиля
// репозитория запускаются кодом, без LLM (docs/stage7-4-checks.md, раздел 2).
package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"prompttree/aicrew/internal/runner"
)

// Этапы, на которых выполняется шаг.
const (
	StageAttempt = "attempt" // проверка попытки задачи
	StageMerge   = "merge"   // проверка результата слияния, отката и переноса в main
)

// Step — одна команда профиля (argv, без оболочки). Kind определяет класс ошибки:
// test → TEST_FAILURE, иначе BUILD_FAILURE.
type Step struct {
	Name string   `json:"name"`
	Kind string   `json:"kind"` // build | test | lint
	Argv []string `json:"argv"`
	// FailOnOutput — шаг провален, если напечатал что-то в stdout (gofmt -l перечисляет неотформатированные файлы).
	FailOnOutput bool `json:"fail_on_output,omitempty"`
	// Stages — где выполнять шаг; пусто — везде (решение 7.4: сборка только при слиянии и переносе).
	Stages []string `json:"stages,omitempty"`
	// Report — формат машинного вывода тестов: flutter_machine | go_json.
	Report string `json:"report,omitempty"`
}

type Profile struct {
	// Template — встроенный профиль: flutter | go | web. Steps, если заданы, заменяют шаги шаблона.
	Template string `json:"template,omitempty"`
	Image    string `json:"image,omitempty"`
	Steps    []Step `json:"steps,omitempty"`
}

var templates = map[string]Profile{
	"flutter": {Image: "aicrew-agent-flutter:local", Steps: []Step{
		{Name: "flutter pub get", Kind: "build", Argv: []string{"flutter", "pub", "get"}},
		{Name: "dart format", Kind: "lint", Argv: []string{"dart", "format", "--output=none", "--set-exit-if-changed", "."}},
		{Name: "flutter analyze", Kind: "lint", Argv: []string{"flutter", "analyze", "--no-fatal-infos"}},
		{Name: "flutter test", Kind: "test", Argv: []string{"flutter", "test", "--machine"}, Report: "flutter_machine"},
		{Name: "flutter build web", Kind: "build", Argv: []string{"flutter", "build", "web"}, Stages: []string{StageMerge}},
	}},
	"go": {Image: "aicrew-agent-go:local", Steps: []Step{
		{Name: "gofmt", Kind: "lint", Argv: []string{"gofmt", "-l", "."}, FailOnOutput: true},
		{Name: "go vet", Kind: "lint", Argv: []string{"go", "vet", "./..."}},
		{Name: "go test", Kind: "test", Argv: []string{"go", "test", "-json", "./..."}, Report: "go_json"},
		{Name: "go build", Kind: "build", Argv: []string{"go", "build", "./..."}},
	}},
	"web": {Image: "aicrew-agent-web:local", Steps: []Step{
		{Name: "npm ci", Kind: "build", Argv: []string{"npm", "ci"}},
		{Name: "npm run lint", Kind: "lint", Argv: []string{"npm", "run", "lint", "--if-present"}},
		{Name: "npm test", Kind: "test", Argv: []string{"npm", "test", "--if-present"}},
		{Name: "npm run build", Kind: "build", Argv: []string{"npm", "run", "build"}},
	}},
}

// Templates — имена встроенных профилей.
func Templates() []string { return []string{"flutter", "go", "web"} }

func ParseProfile(raw json.RawMessage) (Profile, error) {
	var p Profile
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return p, nil
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, fmt.Errorf("verification profile: %w", err)
	}
	if p.Template != "" {
		t, ok := templates[p.Template]
		if !ok {
			return p, fmt.Errorf("verification profile: unknown template %q (known: %s)", p.Template, strings.Join(Templates(), ", "))
		}
		if p.Image == "" {
			p.Image = t.Image
		}
		if len(p.Steps) == 0 {
			p.Steps = append([]Step(nil), t.Steps...)
		}
	}
	for _, s := range p.Steps {
		if len(s.Argv) == 0 {
			return p, fmt.Errorf("verification step %q has no command", s.Name)
		}
		for _, st := range s.Stages {
			if st != StageAttempt && st != StageMerge {
				return p, fmt.Errorf("verification step %q: unknown stage %q", s.Name, st)
			}
		}
		if s.Report != "" && s.Report != "flutter_machine" && s.Report != "go_json" {
			return p, fmt.Errorf("verification step %q: unknown report format %q", s.Name, s.Report)
		}
	}
	return p, nil
}

// ForStage — шаги, которые выполняются на этом этапе.
func (p Profile) ForStage(stage string) Profile {
	out := Profile{Template: p.Template, Image: p.Image}
	for _, s := range p.Steps {
		if len(s.Stages) == 0 || contains(s.Stages, stage) {
			out.Steps = append(out.Steps, s)
		}
	}
	return out
}

// Commands — шаги профиля для подсказки исполнителю: прогони их сам до завершения.
func (p Profile) Commands() []string {
	var out []string
	for _, s := range p.Steps {
		out = append(out, strings.Join(s.Argv, " "))
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

type StepResult struct {
	Name     string      `json:"name"`
	ExitCode int         `json:"exit_code"`
	Seconds  int         `json:"seconds"`
	Output   string      `json:"output_tail"`
	Tests    *TestReport `json:"tests,omitempty"`
}

type Report struct {
	Passed bool         `json:"passed"`
	Steps  []StepResult `json:"steps"`
	// FailedKind — kind первой упавшей команды.
	FailedKind string `json:"failed_kind,omitempty"`
	TimedOut   bool   `json:"timed_out,omitempty"`
}

// Tests — итог тестов из отчёта шага с машинным выводом (test_report для ревьюера).
func (r Report) Tests() *TestReport {
	for _, s := range r.Steps {
		if s.Tests != nil {
			return s.Tests
		}
	}
	return nil
}

// Verifier запускает шаги профиля в одноразовом контейнере с примонтированным worktree.
type Verifier struct {
	R            runner.Runner
	DefaultImage string
}

func (v Verifier) Run(ctx context.Context, p Profile, workdir, name string, timeout time.Duration) (Report, error) {
	rep := Report{Passed: true}
	if len(p.Steps) == 0 {
		return rep, nil
	}
	image := p.Image
	if image == "" {
		image = v.DefaultImage
	}
	deadline := time.Now().Add(timeout)

	// Один контейнер на всю проверку: шаги видят то, что подготовили предыдущие
	// (кэш пакетов в $HOME и т.п.), а не только файлы в /work.
	start, err := v.R.Run(ctx, runner.Command{Exe: "docker", Timeout: 2 * time.Minute,
		Args: []string{"run", "-d", "--rm", "--name", name, "-v", workdir + ":/work", "-w", "/work", "--entrypoint", "sleep", image, "infinity"}})
	if err != nil {
		return rep, err
	}
	if start.ExitCode != 0 {
		return rep, fmt.Errorf("verify container: %s", strings.TrimSpace(start.Stderr))
	}
	defer func() {
		_, _ = v.R.Run(context.WithoutCancel(ctx), runner.Command{Exe: "docker", Args: []string{"rm", "-f", name}, Timeout: 30 * time.Second})
	}()

	for _, s := range p.Steps {
		left := time.Until(deadline)
		if left <= 0 {
			rep.Passed, rep.TimedOut, rep.FailedKind = false, true, s.Kind
			break
		}
		args := append([]string{"exec", name}, s.Argv...)
		res, err := v.R.Run(ctx, runner.Command{Exe: "docker", Args: args, Timeout: left})
		if err != nil {
			return rep, err
		}
		sr := StepResult{Name: s.Name, ExitCode: res.ExitCode, Seconds: int(res.Duration.Seconds())}
		out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
		switch s.Report {
		case "flutter_machine":
			sr.Tests = ParseFlutterMachine(res.Stdout)
		case "go_json":
			sr.Tests = ParseGoJSON(res.Stdout)
		}
		if sr.Tests != nil {
			// Машинный вывод нечитаем: в хвост — понятная сводка и stderr.
			out = strings.TrimSpace(sr.Tests.Summary() + "\n" + res.Stderr)
		}
		if s.FailOnOutput && sr.ExitCode == 0 && strings.TrimSpace(res.Stdout) != "" {
			sr.ExitCode = 1
			out = "command printed output, which this step treats as failure:\n" + out
		}
		sr.Output = tail(out, 4000)
		rep.Steps = append(rep.Steps, sr)
		if res.TimedOut {
			rep.Passed, rep.TimedOut, rep.FailedKind = false, true, s.Kind
			break
		}
		if sr.ExitCode != 0 {
			rep.Passed, rep.FailedKind = false, s.Kind
			break
		}
	}
	return rep, nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
