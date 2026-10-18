package host

import (
	"context"
	"testing"

	"prompttree/aicrew/internal/sysmem"
	"prompttree/aicrew/internal/verify"
)

// Этап 7.8: Codex — второй исполнитель в своей очереди (docs/stage7-8-codex.md).
func TestCodexTasksGoToCodex(t *testing.T) {
	srv := &fakeServer{}
	claude := &fakeAgent{run: writes("a", "x")}
	codex := &fakeAgent{run: writes("b", "y")}
	h := newHost(t, srv, claude, fakeVerifier{rep: verify.Report{Passed: true}})
	h.Codex = codex
	tk := task(gitRepo(t))
	tk.Executor = "codex"
	h.Process(context.Background(), tk)
	if codex.req.TaskID == "" || claude.req.TaskID != "" {
		t.Fatal("codex task must run on Codex")
	}
	if d := srv.facts[1].Details.(map[string]any); d["executor"] != "codex" {
		t.Fatalf("executor must be recorded: %v", d["executor"])
	}
	srv2 := &fakeServer{}
	h2 := newHost(t, srv2, claude, fakeVerifier{})
	tk2 := task(gitRepo(t))
	tk2.Executor = "codex"
	h2.Process(context.Background(), tk2)
	if _, f := last(srv2); f.FailureCode != "no_codex" {
		t.Fatalf("%+v", f)
	}
}

func TestSecondLaneWaitsForJuniorAndMemory(t *testing.T) {
	free := uint64(16 << 30)
	l := &Lanes{MinFreeRAM: 8 << 30, Memory: func() (sysmem.Status, error) { return sysmem.Status{Total: 32 << 30, Available: free}, nil }}
	if !l.mayStart(LaneMain) || !l.mayStart(LaneCodex) {
		t.Fatal("both lanes may start on an idle PC")
	}
	l.set(LaneMain, "claude")
	if !l.mayStart(LaneCodex) {
		t.Fatal("Codex runs in parallel with Claude")
	}
	l.set(LaneMain, "junior")
	if l.mayStart(LaneCodex) {
		t.Fatal("Codex must wait while the local model works")
	}
	l.set(LaneMain, "")
	free = 6 << 30
	if l.mayStart(LaneCodex) || !l.mayStart(LaneMain) {
		t.Fatal("with little free memory only the main lane starts")
	}
}
