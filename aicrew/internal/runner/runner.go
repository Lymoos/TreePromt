// Package runner запускает внешние программы для aicrew-host по правилам ТЗ п. 12:
// только исполняемые файлы из allowlist, только argv (никаких sh -c), с таймаутом,
// фиксированным рабочим каталогом и явно заданным окружением.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotAllowed — программы нет в allowlist: неизвестное — DENY.
var ErrNotAllowed = errors.New("executable is not in the allowlist")

type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	TimedOut bool
}

// Runner — интерфейс, чтобы в тестах подменять docker.
type Runner interface {
	Run(ctx context.Context, cmd Command) (Result, error)
}

type Command struct {
	Exe     string   // имя из allowlist: git, docker
	Args    []string // argv без оболочки
	Dir     string
	Env     []string // дополнительные переменные NAME=value (в argv их нет — не видны в списке процессов)
	Timeout time.Duration
	// Stdout — куда дополнительно копировать вывод по мере поступления (поток stream-json агента).
	Stdout func(line string)
	// Stdin — данные на вход (длинный текст задачи: в командной строке Windows ограничение ~32 тыс. символов).
	Stdin string
}

type Exec struct {
	allowed map[string]bool
}

// New — allowlist по умолчанию: только то, что хосту нужно самому.
func New(allowed ...string) *Exec {
	if len(allowed) == 0 {
		allowed = []string{"git", "docker"}
	}
	m := map[string]bool{}
	for _, a := range allowed {
		m[a] = true
	}
	return &Exec{allowed: m}
}

// shells — интерпретаторы, которые нельзя добавить в allowlist ни при каких настройках.
var shells = map[string]bool{"sh": true, "bash": true, "cmd": true, "powershell": true, "pwsh": true, "python": true, "node": true}

func (e *Exec) Run(ctx context.Context, c Command) (Result, error) {
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(c.Exe), ".exe"))
	if c.Exe != filepath.Base(c.Exe) || !e.allowed[name] || shells[name] {
		return Result{}, fmt.Errorf("%w: %q", ErrNotAllowed, c.Exe)
	}
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = append(baseEnv(), c.Env...)
	if c.Stdin != "" {
		cmd.Stdin = strings.NewReader(c.Stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &limited{w: &stderr, max: 1 << 20}
	if c.Stdout != nil {
		cmd.Stdout = &lineSink{fn: c.Stdout, buf: &limited{w: &stdout, max: 4 << 20}}
	} else {
		cmd.Stdout = &limited{w: &stdout, max: 4 << 20}
	}
	start := time.Now()
	err := cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String(), Duration: time.Since(start)}
	if ctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		res.ExitCode = -1
		return res, nil
	}
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	case err != nil:
		return res, err
	}
	return res, nil
}

// baseEnv — минимальное окружение: без унаследованных секретов хоста.
func baseEnv() []string {
	var env []string
	for _, k := range []string{"PATH", "SYSTEMROOT", "SystemRoot", "TEMP", "TMP", "HOME", "USERPROFILE", "LOCALAPPDATA",
		"APPDATA", "ProgramFiles", "ProgramData", "DOCKER_HOST", "DOCKER_CONFIG"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

type limited struct {
	w   *bytes.Buffer
	max int
}

func (l *limited) Write(p []byte) (int, error) {
	if room := l.max - l.w.Len(); room > 0 {
		if len(p) > room {
			l.w.Write(p[:room])
		} else {
			l.w.Write(p)
		}
	}
	return len(p), nil
}

type lineSink struct {
	fn      func(string)
	buf     *limited
	partial []byte
}

func (s *lineSink) Write(p []byte) (int, error) {
	s.buf.Write(p)
	s.partial = append(s.partial, p...)
	for {
		i := bytes.IndexByte(s.partial, '\n')
		if i < 0 {
			break
		}
		s.fn(strings.TrimRight(string(s.partial[:i]), "\r"))
		s.partial = s.partial[i+1:]
	}
	return len(p), nil
}
