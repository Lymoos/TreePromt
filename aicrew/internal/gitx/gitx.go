// Package gitx — git-операции хоста (ТЗ п. 8.5): worktree на задачу, автокоммит,
// машинные факты (base/result commit, изменённые файлы, diff --stat).
package gitx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"prompttree/aicrew/internal/runner"
)

const WorktreeDir = ".aiworktrees"

type Git struct {
	R runner.Runner
}

func (g Git) run(ctx context.Context, dir string, args ...string) (string, error) {
	res, err := g.R.Run(ctx, runner.Command{Exe: "git", Args: args, Dir: dir, Timeout: 2 * time.Minute,
		// Коммиты агента подписываются ботом и не используют личные настройки пользователя.
		Env: []string{"GIT_AUTHOR_NAME=AiCrew", "GIT_AUTHOR_EMAIL=aicrew@localhost",
			"GIT_COMMITTER_NAME=AiCrew", "GIT_COMMITTER_EMAIL=aicrew@localhost", "GIT_TERMINAL_PROMPT=0"}})
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("git %s: exit %d: %s", strings.Join(args, " "), res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return strings.TrimSpace(res.Stdout), nil
}

func WorktreePath(repo, taskID string) string {
	return filepath.Join(repo, WorktreeDir, "task-"+taskID)
}

func Branch(taskID string) string { return "aicrew/task-" + taskID }

// AddWorktree создаёт чистый worktree задачи от HEAD основной ветки и возвращает base_commit.
// Остатки прошлой попытки удаляются: каждая попытка начинается с чистого листа.
func (g Git) AddWorktree(ctx context.Context, repo, taskID, baseBranch string) (path, base string, err error) {
	path = WorktreePath(repo, taskID)
	_ = g.RemoveWorktree(ctx, repo, taskID)
	if base, err = g.run(ctx, repo, "rev-parse", baseBranch); err != nil {
		return "", "", err
	}
	if err = g.ensureIgnored(repo); err != nil {
		return "", "", err
	}
	if _, err = g.run(ctx, repo, "worktree", "add", "-B", Branch(taskID), path, base); err != nil {
		return "", "", err
	}
	return path, base, nil
}

// ensureIgnored: каталог worktree не должен попадать в git status основного checkout.
func (g Git) ensureIgnored(repo string) error {
	excl := filepath.Join(repo, ".git", "info", "exclude")
	b, _ := os.ReadFile(excl)
	if strings.Contains(string(b), WorktreeDir+"/") {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(excl), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(excl, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("\n" + WorktreeDir + "/\n")
	return err
}

// RemoveWorktree удаляет worktree и ветку задачи (провал до слияния — ТЗ п. 8.5).
func (g Git) RemoveWorktree(ctx context.Context, repo, taskID string) error {
	path := WorktreePath(repo, taskID)
	_, err := g.run(ctx, repo, "worktree", "remove", "--force", path)
	if err != nil {
		_ = os.RemoveAll(path)
		_, _ = g.run(ctx, repo, "worktree", "prune")
	}
	_, _ = g.run(ctx, repo, "branch", "-D", Branch(taskID))
	return nil
}

// ListWorktreeTasks — id задач, для которых на диске есть worktree.
func ListWorktreeTasks(repo string) []string {
	entries, _ := os.ReadDir(filepath.Join(repo, WorktreeDir))
	var ids []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "task-") {
			ids = append(ids, strings.TrimPrefix(e.Name(), "task-"))
		}
	}
	return ids
}

// Facts — то, что хост сообщает серверу о результате агента. Собирается кодом.
type Facts struct {
	ResultCommit string   `json:"result_commit"`
	ChangedFiles []string `json:"changed_files"`
	DiffStat     string   `json:"diff_stat"`
	NoChanges    bool     `json:"no_changes"`
}

// Autocommit фиксирует всё, что сделал агент, и собирает факты относительно base.
func (g Git) Autocommit(ctx context.Context, worktree, base, message string) (Facts, error) {
	if _, err := g.run(ctx, worktree, "add", "-A"); err != nil {
		return Facts{}, err
	}
	status, err := g.run(ctx, worktree, "status", "--porcelain")
	if err != nil {
		return Facts{}, err
	}
	if status != "" {
		if _, err := g.run(ctx, worktree, "commit", "--no-verify", "-m", message); err != nil {
			return Facts{}, err
		}
	}
	head, err := g.run(ctx, worktree, "rev-parse", "HEAD")
	if err != nil {
		return Facts{}, err
	}
	f := Facts{ResultCommit: head, NoChanges: head == base}
	names, err := g.run(ctx, worktree, "diff", "--name-only", base, head)
	if err != nil {
		return Facts{}, err
	}
	for _, n := range strings.Split(names, "\n") {
		if n = strings.TrimSpace(n); n != "" {
			f.ChangedFiles = append(f.ChangedFiles, n)
		}
	}
	if f.ChangedFiles == nil {
		f.ChangedFiles = []string{}
	}
	f.DiffStat, err = g.run(ctx, worktree, "diff", "--stat", base, head)
	return f, err
}
