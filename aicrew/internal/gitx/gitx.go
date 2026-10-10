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

// TempWorktreePath — временный worktree очереди слияния (merge-<id>, rollback-<id>, promote-…).
func TempWorktreePath(repo, name string) string {
	return filepath.Join(repo, WorktreeDir, name)
}

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

// Diff — полный diff результата относительно base (факт для ревьюера, ТЗ п. 9).
func (g Git) Diff(ctx context.Context, worktree, base, head string) (string, error) {
	return g.run(ctx, worktree, "diff", "--no-color", "--no-ext-diff", base, head)
}

// ShowFile — содержимое файла в коммите; false — файла там нет.
// Контракт архитектуры читается из base_commit: исполнитель не может ослабить его своей правкой.
func (g Git) ShowFile(ctx context.Context, repo, commit, path string) ([]byte, bool) {
	res, err := g.R.Run(ctx, runner.Command{Exe: "git", Args: []string{"show", commit + ":" + path}, Dir: repo, Timeout: time.Minute})
	if err != nil || res.ExitCode != 0 {
		return nil, false
	}
	return []byte(res.Stdout), true
}

// ── очередь слияния (этап 7.3) ──

func (g Git) RevParse(ctx context.Context, repo, ref string) (string, error) {
	return g.run(ctx, repo, "rev-parse", "--verify", ref+"^{commit}")
}

// EnsureBranch создаёт ветку от from, если её ещё нет (integration-ветка при первом запуске).
func (g Git) EnsureBranch(ctx context.Context, repo, branch, from string) error {
	if _, err := g.RevParse(ctx, repo, "refs/heads/"+branch); err == nil {
		return nil
	}
	_, err := g.run(ctx, repo, "branch", branch, from)
	return err
}

// IsAncestor — содержится ли коммит a в истории b.
func (g Git) IsAncestor(ctx context.Context, repo, a, b string) bool {
	_, err := g.run(ctx, repo, "merge-base", "--is-ancestor", a, b)
	return err == nil
}

// AddTempWorktree — отдельный detached worktree на коммите: слияние и откат не трогают
// ни checkout пользователя, ни саму integration-ветку, пока проверка не пройдёт.
func (g Git) AddTempWorktree(ctx context.Context, repo, name, commit string) (string, error) {
	path := TempWorktreePath(repo, name)
	g.RemoveTempWorktree(ctx, repo, name)
	if err := g.ensureIgnored(repo); err != nil {
		return "", err
	}
	if _, err := g.run(ctx, repo, "worktree", "add", "--detach", path, commit); err != nil {
		return "", err
	}
	return path, nil
}

func (g Git) RemoveTempWorktree(ctx context.Context, repo, name string) {
	path := TempWorktreePath(repo, name)
	if _, err := g.run(ctx, repo, "worktree", "remove", "--force", path); err != nil {
		_ = os.RemoveAll(path)
		_, _ = g.run(ctx, repo, "worktree", "prune")
	}
}

// ListTempWorktrees — временные worktree очереди слияния; после перезапуска хоста все они мусор.
func ListTempWorktrees(repo string) []string {
	entries, _ := os.ReadDir(filepath.Join(repo, WorktreeDir))
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), "task-") {
			names = append(names, e.Name())
		}
	}
	return names
}

// unmerged — файлы с конфликтом после неудачного merge/revert.
func (g Git) unmerged(ctx context.Context, worktree string) []string {
	out, _ := g.run(ctx, worktree, "diff", "--name-only", "--diff-filter=U")
	var files []string
	for _, f := range strings.Split(out, "\n") {
		if f = strings.TrimSpace(f); f != "" {
			files = append(files, f)
		}
	}
	return files
}

// Merge сливает ветку в HEAD временного worktree отдельным коммитом слияния (--no-ff),
// чтобы задачу можно было откатить целиком. При конфликте слияние отменяется
// и возвращается список конфликтных файлов.
func (g Git) Merge(ctx context.Context, worktree, branch, message string) (commit string, conflicts []string, err error) {
	if _, err = g.run(ctx, worktree, "merge", "--no-ff", "--no-verify", "-m", message, branch); err != nil {
		if conflicts = g.unmerged(ctx, worktree); len(conflicts) > 0 {
			_, _ = g.run(ctx, worktree, "merge", "--abort")
			return "", conflicts, nil
		}
		return "", nil, err
	}
	commit, err = g.run(ctx, worktree, "rev-parse", "HEAD")
	return commit, nil, err
}

// Revert откатывает коммит слияния задачи (-m 1: относительно integration до слияния).
func (g Git) Revert(ctx context.Context, worktree, mergeCommit, message string) (commit string, conflicts []string, err error) {
	if _, err = g.run(ctx, worktree, "revert", "--no-edit", "-m", "1", mergeCommit); err != nil {
		if conflicts = g.unmerged(ctx, worktree); len(conflicts) > 0 {
			_, _ = g.run(ctx, worktree, "revert", "--abort")
			return "", conflicts, nil
		}
		return "", nil, err
	}
	if message != "" {
		if _, err = g.run(ctx, worktree, "commit", "--amend", "--no-verify", "-m", message); err != nil {
			return "", nil, err
		}
	}
	commit, err = g.run(ctx, worktree, "rev-parse", "HEAD")
	return commit, nil, err
}

// AdvanceBranch переводит ветку на commit, только если она всё ещё указывает на old
// (атомарно, compare-and-swap): integration не перескочит через чужое изменение.
func (g Git) AdvanceBranch(ctx context.Context, repo, branch, commit, old string) error {
	_, err := g.run(ctx, repo, "update-ref", "refs/heads/"+branch, commit, old)
	return err
}

// FastForward переносит ветку target на source только перемоткой (integration → main).
// Если target выгружена в рабочую папку пользователя, папка должна быть чистой: тогда
// используется git merge --ff-only, и файлы обновляются вместе с веткой.
func (g Git) FastForward(ctx context.Context, repo, target, source string) (from, to string, err error) {
	if from, err = g.RevParse(ctx, repo, target); err != nil {
		return "", "", err
	}
	if to, err = g.RevParse(ctx, repo, source); err != nil {
		return "", "", err
	}
	if from == to {
		return from, to, nil
	}
	if !g.IsAncestor(ctx, repo, from, to) {
		return "", "", fmt.Errorf("%s has commits that %s does not have: fast-forward is impossible, merge by hand", target, source)
	}
	current, _ := g.run(ctx, repo, "symbolic-ref", "--quiet", "--short", "HEAD")
	if current != target {
		return from, to, g.AdvanceBranch(ctx, repo, target, to, from)
	}
	status, err := g.run(ctx, repo, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return "", "", err
	}
	if status != "" {
		return "", "", fmt.Errorf("%s is checked out and has uncommitted changes: commit or stash them first", target)
	}
	_, err = g.run(ctx, repo, "merge", "--ff-only", source)
	return from, to, err
}
