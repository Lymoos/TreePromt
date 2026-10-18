package host

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/runner"
)

// Привязка кода из окна проекта (этап 7.9, docs/stage7-9-projects.md): папка на ПК и/или git-адрес.
// Папка есть — проверяем и работаем в ней (если указан адрес, он должен совпасть с origin);
// папки нет — клонируем адрес в неё (пусто — в рабочую папку WorkRoot\<имя>).

type bindAPI interface {
	PendingRepositories(ctx context.Context) ([]api.Repository, error)
	ReportRepository(ctx context.Context, repoID, localPath, failure string) error
}

type Binder struct {
	API      bindAPI
	R        runner.Runner // git
	WorkRoot string        // куда клонировать без указанной папки: D:\aicrew-work
	Owner    string        // профиль владельца: туда код не кладём
	Log      *slog.Logger
}

// Run — заявки проверяются при старте и затем каждые every (или по сигналу wake).
func (b Binder) Run(ctx context.Context, every time.Duration, wake <-chan struct{}) {
	for {
		b.Once(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		case <-wake:
		}
	}
}

func (b Binder) Once(ctx context.Context) {
	repos, err := b.API.PendingRepositories(ctx)
	if err != nil {
		if b.Log != nil && ctx.Err() == nil {
			b.Log.Warn("pending repositories", "err", err)
		}
		return
	}
	for _, repo := range repos {
		path, err := b.bind(ctx, repo)
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		if b.Log != nil {
			b.Log.Info("repository bind", "repo", repo.Name, "path", path, "error", failure)
		}
		if err := b.API.ReportRepository(ctx, repo.ID, path, failure); err != nil && b.Log != nil {
			b.Log.Warn("report repository", "repo", repo.Name, "err", err)
		}
	}
}

func (b Binder) bind(ctx context.Context, repo api.Repository) (string, error) {
	path := strings.TrimSpace(repo.LocalPath)
	if path == "" {
		if b.WorkRoot == "" {
			return "", errors.New("не указана папка, а рабочая папка AiCrew не настроена")
		}
		path = filepath.Join(b.WorkRoot, repo.Name)
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("укажите полный путь к папке (например D:\\aicrew-work\\%s)", repo.Name)
	}
	if b.Owner != "" && within(path, b.Owner) {
		return "", errors.New("папка внутри профиля пользователя Windows: агентам туда нельзя, выберите другую (например на D:)")
	}
	if st, err := os.Stat(path); err == nil {
		if !st.IsDir() {
			return "", errors.New("по этому пути файл, а не папка")
		}
		top, err := b.git(ctx, path, time.Minute, "rev-parse", "--show-toplevel")
		if err != nil {
			if empty(path) && repo.GitURL != "" {
				return path, b.clone(ctx, repo.GitURL, path)
			}
			return "", errors.New("папка не git-репозиторий: выполните в ней git init или укажите git-адрес и пустую папку")
		}
		if !samePath(top, path) {
			return "", fmt.Errorf("папка внутри другого репозитория (%s): укажите его корень", top)
		}
		if repo.GitURL != "" {
			origin, err := b.git(ctx, path, time.Minute, "remote", "get-url", "origin")
			if err != nil {
				return "", fmt.Errorf("у папки нет origin, а указан адрес %s: выполните git remote add origin %s", repo.GitURL, repo.GitURL)
			}
			if normalizeGitURL(origin) != normalizeGitURL(repo.GitURL) {
				return "", fmt.Errorf("в папке другой репозиторий: origin %s, а указан %s", origin, repo.GitURL)
			}
		}
		return path, nil
	}
	if repo.GitURL == "" {
		return "", errors.New("папка не найдена, а git-адрес не указан")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, b.clone(ctx, repo.GitURL, path)
}

func (b Binder) clone(ctx context.Context, url, path string) error {
	if _, err := b.git(ctx, "", 30*time.Minute, "clone", "--origin", "origin", url, path); err != nil {
		return fmt.Errorf("не удалось скачать %s: %v (для приватного репозитория войдите в GitHub на этом ПК: gh auth login)", url, err)
	}
	return nil
}

func (b Binder) git(ctx context.Context, dir string, timeout time.Duration, args ...string) (string, error) {
	res, err := b.R.Run(ctx, runner.Command{Exe: "git", Args: args, Dir: dir, Timeout: timeout})
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		if len(msg) > 300 {
			msg = msg[len(msg)-300:]
		}
		return "", errors.New(msg)
	}
	return strings.TrimSpace(res.Stdout), nil
}

func empty(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) == 0
}

func within(path, root string) bool {
	p, r := strings.ToLower(filepath.Clean(path)), strings.ToLower(filepath.Clean(root))
	return p == r || strings.HasPrefix(p, r+string(filepath.Separator))
}

// samePath — одна и та же папка. Сравниваются сами папки: на Windows путь может прийти в коротком виде
// (C:\Users\RUNNER~1\…), а git отвечает полным.
func samePath(a, b string) bool {
	a, b = filepath.Clean(filepath.FromSlash(a)), filepath.Clean(filepath.FromSlash(b))
	if strings.EqualFold(a, b) {
		return true
	}
	sa, errA := os.Stat(a)
	sb, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(sa, sb)
}

var sshURL = regexp.MustCompile(`^git@([^:]+):(.+)$`)

// normalizeGitURL — один вид для https и ssh: «github.com/owner/repo».
func normalizeGitURL(u string) string {
	u = strings.TrimSpace(u)
	if m := sshURL.FindStringSubmatch(u); m != nil {
		u = m[1] + "/" + m[2]
	}
	u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	if i := strings.Index(u, "@"); i >= 0 && i < strings.Index(u+"/", "/") {
		u = u[i+1:] // https://user@github.com/…
	}
	u = strings.TrimSuffix(strings.TrimSuffix(u, "/"), ".git")
	return strings.ToLower(u)
}
