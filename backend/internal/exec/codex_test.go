package exec

import (
	"testing"

	"github.com/google/uuid"
)

// Этап 7.8: очередь Codex идёт параллельно с Claude (docs/stage7-8-codex.md).
func TestCodexWorkerTakesOnlyCodexTasksAndNoMerges(t *testing.T) {
	f := newFx(t)
	f.registries(nil, nil, []string{"claude", "codex"})
	h, err := f.svc.RegisterHost(f.ctx, f.host, "home-pc", "windows", []string{"flutter", "go"}, []string{"container"},
		[]WorkerSpec{{Kind: "claude_code", Model: "sonnet"}, {Kind: WorkerCodex, Model: ""}})
	if err != nil {
		t.Fatal(err)
	}
	codexWorker := uuid.MustParse(h.Workers[WorkerCodex+"/"])
	claudeTask := f.task()
	codexTask := f.task(func(n *NewTask) { n.Executor = "codex"; n.Title = "Через Codex" })

	byCodex, err := f.svc.Claim(f.ctx, f.host, codexWorker)
	if err != nil || byCodex == nil || byCodex.ID != codexTask || byCodex.Executor != "codex" {
		t.Fatalf("codex worker: %+v %v", byCodex, err)
	}
	// Одновременно основной работник берёт задачу Claude — две задачи идут параллельно.
	byClaude := f.claim()
	if byClaude == nil || byClaude.ID != claudeTask {
		t.Fatalf("claude worker: %+v", byClaude)
	}
	if f.claim() != nil {
		t.Fatal("claude worker must not take codex tasks")
	}
	// Слияние — только у основного работника.
	f.exec(`UPDATE exec.tasks SET status = 'MERGE_QUEUED', worker_id = NULL WHERE id = $1`, claudeTask)
	if c, _ := f.svc.Claim(f.ctx, f.host, codexWorker); c != nil {
		t.Fatalf("codex worker must not take merge jobs: %+v", c)
	}
}
