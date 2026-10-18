package host

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/runner"
)

type fakeBindAPI struct {
	pending []api.Repository
	reports map[string][2]string // id → {path, failure}
}

func (f *fakeBindAPI) PendingRepositories(context.Context) ([]api.Repository, error) {
	return f.pending, nil
}
func (f *fakeBindAPI) ReportRepository(_ context.Context, id, path, failure string) error {
	f.reports[id] = [2]string{path, failure}
	return nil
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

// Этап 7.9: заявки на привязку кода из окна проекта — папка и/или git-адрес.
func TestBinderBindsFoldersAndClones(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	base := t.TempDir()
	// Готовая папка с origin.
	game := filepath.Join(base, "game")
	_ = os.MkdirAll(game, 0o755)
	gitIn(t, game, "init", "-q", "-b", "main")
	gitIn(t, game, "remote", "add", "origin", "git@github.com:Lymoos/easter-hunt.git")
	// «Удалённый» репозиторий для клона — локальный bare.
	remote := filepath.Join(base, "remote.git")
	gitIn(t, base, "init", "-q", "--bare", remote)
	notGit := filepath.Join(base, "plain")
	_ = os.MkdirAll(notGit, 0o755)
	_ = os.WriteFile(filepath.Join(notGit, "a.txt"), []byte("x"), 0o644)
	owner := filepath.Join(base, "Users", "Lymoos")
	_ = os.MkdirAll(filepath.Join(owner, "code"), 0o755)

	f := &fakeBindAPI{reports: map[string][2]string{}, pending: []api.Repository{
		{ID: "same", Name: "game", LocalPath: game, GitURL: "https://github.com/Lymoos/easter-hunt"},
		{ID: "other", Name: "game", LocalPath: game, GitURL: "https://github.com/Lymoos/another"},
		{ID: "clone", Name: "cloned", GitURL: remote},
		{ID: "plain", Name: "plain", LocalPath: notGit},
		{ID: "profile", Name: "code", LocalPath: filepath.Join(owner, "code")},
		{ID: "missing", Name: "x", LocalPath: filepath.Join(base, "nope")},
	}}
	b := Binder{API: f, R: runner.New("git"), WorkRoot: filepath.Join(base, "work"), Owner: owner}
	b.Once(context.Background())

	check := func(id, wantPath, wantErr string) {
		t.Helper()
		got := f.reports[id]
		if wantErr == "" && (got[1] != "" || !samePath(got[0], wantPath)) {
			t.Errorf("%s: %q %q", id, got[0], got[1])
		}
		if wantErr != "" && !strings.Contains(got[1], wantErr) {
			t.Errorf("%s: want error with %q, got %q", id, wantErr, got[1])
		}
	}
	check("same", game, "")
	check("other", "", "другой репозиторий")
	check("clone", filepath.Join(base, "work", "cloned"), "")
	check("plain", "", "не git-репозиторий")
	check("profile", "", "профиля пользователя")
	check("missing", "", "не найдена")
	if _, err := os.Stat(filepath.Join(base, "work", "cloned", ".git")); err != nil {
		t.Fatal("the repository must be cloned into the work folder")
	}
}

func TestNormalizeGitURL(t *testing.T) {
	for _, u := range []string{"https://github.com/Lymoos/TreePromt.git", "git@github.com:Lymoos/TreePromt.git",
		"https://user@github.com/lymoos/treepromt/", "http://github.com/Lymoos/TreePromt"} {
		if got := normalizeGitURL(u); got != "github.com/lymoos/treepromt" {
			t.Errorf("%s → %s", u, got)
		}
	}
}
