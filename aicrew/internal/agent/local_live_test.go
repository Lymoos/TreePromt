package agent

import (
	"context"
	"os"
	"testing"
	"time"

	"prompttree/aicrew/internal/runner"
)

// Живая проверка Junior на настоящей модели (по желанию, не в CI):
//
//	AICREW_LIVE_REPO=<копия репозитория> AICREW_LIVE_PROMPT="…" go test -run LocalLive -v ./internal/agent
func TestLocalLive(t *testing.T) {
	repo := os.Getenv("AICREW_LIVE_REPO")
	if repo == "" {
		t.Skip("AICREW_LIVE_REPO is not set")
	}
	model := os.Getenv("AICREW_LIVE_MODEL")
	if model == "" {
		model = "qwen3-coder:30b"
	}
	l := &Local{R: runner.New("docker"), Image: "aicrew-agent-flutter:local", BaseURL: "http://127.0.0.1:11434", Model: model, NumCtx: 16384}
	start := time.Now()
	res, err := l.Run(context.Background(), Request{TaskID: "live", Attempt: 1, Workdir: repo, Prompt: os.Getenv("AICREW_LIVE_PROMPT"),
		Timeout: 40 * time.Minute, Checks: []string{"flutter analyze", "flutter test"}})
	t.Logf("%s, turns %d, tokens in %d out %d\nsummary: %s", time.Since(start).Round(time.Second), res.Usage.Turns,
		res.Usage.InputTokens, res.Usage.OutputTokens, res.Summary)
	if err != nil {
		t.Fatal(err)
	}
}
