// Package verify — детерминированная проверка результата (ТЗ п. 9, шаг 1): команды профиля
// репозитория запускаются кодом, без LLM. Полные профили Go/Flutter/веб/Roblox — этап 7.4.
package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"prompttree/aicrew/internal/runner"
)

// Step — одна команда профиля. Kind определяет класс ошибки: test → TEST_FAILURE, иначе BUILD_FAILURE.
type Step struct {
	Name string   `json:"name"`
	Kind string   `json:"kind"` // build | test | lint
	Argv []string `json:"argv"`
}

type Profile struct {
	Image string `json:"image"`
	Steps []Step `json:"steps"`
}

func ParseProfile(raw json.RawMessage) (Profile, error) {
	var p Profile
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return p, nil
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, fmt.Errorf("verification profile: %w", err)
	}
	for _, s := range p.Steps {
		if len(s.Argv) == 0 {
			return p, fmt.Errorf("verification step %q has no command", s.Name)
		}
	}
	return p, nil
}

type StepResult struct {
	Name     string `json:"name"`
	ExitCode int    `json:"exit_code"`
	Seconds  int    `json:"seconds"`
	Output   string `json:"output_tail"`
}

type Report struct {
	Passed bool         `json:"passed"`
	Steps  []StepResult `json:"steps"`
	// FailedKind — kind первой упавшей команды.
	FailedKind string `json:"failed_kind,omitempty"`
	TimedOut   bool   `json:"timed_out,omitempty"`
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
		out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
		rep.Steps = append(rep.Steps, StepResult{Name: s.Name, ExitCode: res.ExitCode, Seconds: int(res.Duration.Seconds()), Output: tail(out, 4000)})
		if res.TimedOut {
			rep.Passed, rep.TimedOut, rep.FailedKind = false, true, s.Kind
			break
		}
		if res.ExitCode != 0 {
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
