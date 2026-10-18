package host

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/secretscan"
	"prompttree/aicrew/internal/verify"
)

// serverWithFiles — имитация сервера, которая отдаёт файлы проекта.
type serverWithFiles struct {
	*fakeServer
	files map[string][]byte
}

func (s serverWithFiles) ProjectFileData(_ context.Context, _, fileID string) ([]byte, error) {
	return s.files[fileID], nil
}

// Этап 7.9: агент получает описание проекта и его файлы только для чтения; Junior — только описание.
func TestProjectContextReachesTheAgent(t *testing.T) {
	spec := []byte("# ТЗ\nДесять пасхалок на карте.")
	sum := sha256.Sum256(spec)
	pc := &api.ProjectContext{ProjectID: "p1", Description: "Игра для детей. Ключ sk-ant-api03-SECRETSECRETSECRETSECRETSECRET.",
		Files: []api.ProjectFile{{ID: "f1", Name: "ТЗ.md", Size: int64(len(spec)), SHA256: hex.EncodeToString(sum[:])}}}

	srv := &fakeServer{}
	var sawFile []byte
	ag := &fakeAgent{run: func(ctx context.Context, req agent.Request) (agent.Result, error) {
		for _, m := range req.Mounts {
			if m.Container == ProjectMount && m.ReadOnly {
				sawFile, _ = os.ReadFile(filepath.Join(m.Host, "ТЗ.md"))
			}
		}
		return writes("a", "x")(ctx, req)
	}}
	h := newHost(t, srv, ag, fakeVerifier{rep: verify.Report{Passed: true}})
	h.API = serverWithFiles{srv, map[string][]byte{"f1": spec}}
	h.Scanner = secretscan.New(nil)
	tk := task(gitRepo(t))
	tk.Project = pc
	h.Process(context.Background(), tk)

	p := ag.req.Prompt
	if !strings.Contains(p, "## About the project") || !strings.Contains(p, "<untrusted>\nИгра для детей.") ||
		!strings.Contains(p, "ТЗ.md") || !strings.Contains(p, "Добавить кнопку") {
		t.Fatalf("prompt: %s", p)
	}
	if strings.Contains(p, "SECRETSECRET") {
		t.Fatal("secrets in the project description must be redacted")
	}
	if string(sawFile) != string(spec) {
		t.Fatalf("the agent must see the project file read-only, got %q", sawFile)
	}

	// Junior: описание есть, файлов нет (его инструменты видят только worktree).
	srv2 := &fakeServer{}
	repo := gitRepo(t)
	commitFile(t, repo, "t_test", "t.Skip(\"aicrew-contract\")\n")
	junior := &fakeAgent{run: writes("b", "y")}
	h2 := newHost(t, srv2, &fakeAgent{}, fakeVerifier{rep: verify.Report{Passed: true}})
	h2.API = serverWithFiles{srv2, map[string][]byte{"f1": spec}}
	h2.Junior = junior
	tk2 := task(repo)
	tk2.Executor, tk2.Contract, tk2.Project = "junior", &api.Contract{Files: []string{"b"}, Tests: []string{"t_test"}}, pc
	h2.Process(context.Background(), tk2)
	if !strings.Contains(junior.req.Prompt, "Игра для детей") || strings.Contains(junior.req.Prompt, "ТЗ.md") {
		t.Fatalf("junior prompt: %s", junior.req.Prompt)
	}
}
