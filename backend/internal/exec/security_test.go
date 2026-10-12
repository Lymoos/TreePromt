package exec

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Этап 7.6: Kill Switch, аудит, ApprovalRequest, замена секретов (docs/stage7-6-security-audit.md).

func secretKey() string {
	return strings.Join([]string{"AIza", "SyA1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6Q"}, "")
}

func (f *fx) openIncidents() []Incident {
	f.t.Helper()
	list, err := f.svc.ListIncidents(f.ctx, f.user, true)
	if err != nil {
		f.t.Fatal(err)
	}
	return list
}

func TestSecretLeakGoesToHumanAndPausesRepository(t *testing.T) {
	f := newFx(t)
	f.task()
	other := f.task()
	c := f.claim()
	f.move(c, StatusInProgress, Facts{})
	if st := f.move(c, StatusFailed, Facts{FailureClass: ClassSecret, FailureCode: "secret_in_changes",
		Reason: "lib/config.dart:1 (google_api_key)"}); st != StatusAwaitingHuman {
		t.Fatalf("secret leak must not be retried: %s", st)
	}
	inc := f.openIncidents()
	if len(inc) != 1 || inc[0].Kind != IncidentSecretLeak {
		t.Fatalf("%+v", inc)
	}
	if got := f.claim(); got != nil {
		t.Fatalf("paused repository must not give work, got %v (other=%s)", got.ID, other)
	}
	approvals, _ := f.svc.ListApprovals(f.ctx, f.user, ApprovalPending)
	if len(approvals) != 1 || approvals[0].Type != "incident" {
		t.Fatalf("incident needs a decision: %+v", approvals)
	}
	if err := f.svc.ResolveApproval(f.ctx, f.user, approvals[0].ID, Decision{Decision: "approve", Comment: "ключ отозван"}); err != nil {
		t.Fatal(err)
	}
	if c2 := f.claim(); c2 == nil || c2.ID != other {
		t.Fatal("after «Возобновить» work continues")
	}
}

func TestThreeRedIntegrationsOpenIncident(t *testing.T) {
	f := newFx(t)
	for i := 0; i < 3; i++ {
		id := f.mergeable()
		_ = f.svc.Merge(f.ctx, f.user, id)
		j := f.claim()
		f.move(j, StatusPostMergeVerify, Facts{})
		f.move(j, StatusFailed, Facts{FailureClass: ClassIntegration, FailureCode: "post_merge_verification_failed"})
		if n := len(f.openIncidents()); (i < 2 && n != 0) || (i == 2 && n != 1) {
			t.Fatalf("after %d red merges: %d incidents", i+1, n)
		}
		f.exec(`UPDATE exec.tasks SET status = 'CANCELLED' WHERE id = $1`, id) // не мешать следующей итерации
	}
	if f.openIncidents()[0].Kind != IncidentIntegrationRed {
		t.Fatal("wrong kind")
	}
}

func TestSuccessfulMergeResetsRedCounter(t *testing.T) {
	f := newFx(t)
	for i := 0; i < 2; i++ {
		id := f.mergeable()
		_ = f.svc.Merge(f.ctx, f.user, id)
		j := f.claim()
		f.move(j, StatusPostMergeVerify, Facts{})
		f.move(j, StatusFailed, Facts{FailureClass: ClassIntegration})
		f.exec(`UPDATE exec.tasks SET status = 'CANCELLED' WHERE id = $1`, id)
	}
	f.done(f.mergeable(), "m1")
	var n int
	_ = f.pool.QueryRow(f.ctx, `SELECT integration_failures FROM exec.repositories WHERE id = $1`, f.repo).Scan(&n)
	if n != 0 {
		t.Fatalf("green merge must reset the counter: %d", n)
	}
}

func TestNightShiftStopsOnAnyIncident(t *testing.T) {
	f := newFx(t)
	f.task(func(n *NewTask) { n.ExecutionMode = "night" })
	if _, err := f.svc.Stop(f.ctx, f.user, nil, "уезжаю"); err != nil {
		t.Fatal(err)
	}
	if f.claim() != nil {
		t.Fatal("«Остановить всё» stops everything")
	}
}

func TestManualStopRepository(t *testing.T) {
	f := newFx(t)
	f.task()
	id, err := f.svc.Stop(f.ctx, f.user, &f.repo, "проверяю")
	if err != nil {
		t.Fatal(err)
	}
	if f.claim() != nil {
		t.Fatal("stopped repository")
	}
	if err := f.svc.ResolveIncident(f.ctx, f.user, id, "ок"); err != nil {
		t.Fatal(err)
	}
	if f.claim() == nil {
		t.Fatal("resumed")
	}
}

func TestQASpikeOpensIncident(t *testing.T) {
	f := newFx(t)
	reject := &ReviewVerdict{Verdict: VerdictReject, RejectionType: "code_bug", Issues: []ReviewIssue{{Severity: "major", Message: "x"}}}
	for i := 0; i < qaWindow; i++ {
		f.task(func(n *NewTask) { n.MaxAttempts = 1 })
		c := f.claim()
		f.toReview(c)
		if i < 2 {
			f.move(c, StatusMergeable, Facts{Review: approve()})
			continue
		}
		f.move(c, StatusFailed, Facts{FailureClass: ClassQAReject, FailureCode: "logic_qa_rejected", Review: reject})
	}
	inc := f.openIncidents()
	if len(inc) != 1 || inc[0].Kind != IncidentQASpike {
		t.Fatalf("8 of 10 rejected → QA kill switch: %+v", inc)
	}
}

func TestAuditIsAppendOnlyAndRedacted(t *testing.T) {
	f := newFx(t)
	id := f.task()
	att := 1
	data, _ := json.Marshal(map[string]any{"exe": "docker", "args": []string{"run", "-e", "KEY=" + secretKey()}, "exit": 0})
	if err := f.svc.WriteAudit(f.ctx, f.host, []AuditEntry{{TaskID: &id, Attempt: &att, Kind: "command", Data: data}},
		[]PromptSnapshot{{Hash: strings.Repeat("a", 64), Text: "задача, ключ " + secretKey()}}); err != nil {
		t.Fatal(err)
	}
	recs, err := f.svc.TaskAudit(f.ctx, f.user, id)
	if err != nil || len(recs) != 1 || strings.Contains(string(recs[0].Data), secretKey()) {
		t.Fatalf("%+v %v", recs, err)
	}
	text, _ := f.svc.PromptSnapshot(f.ctx, f.user, strings.Repeat("a", 64))
	if strings.Contains(text, secretKey()) || !strings.Contains(text, "[REDACTED:google_api_key]") {
		t.Fatal(text)
	}
	if _, err := f.pool.Exec(f.ctx, `UPDATE exec.audit SET data = '{}'`); err == nil {
		t.Fatal("audit must be append-only (update)")
	}
	if _, err := f.pool.Exec(f.ctx, `DELETE FROM exec.audit`); err == nil {
		t.Fatal("audit must be append-only (delete)")
	}
	stranger := Identity{UserID: uuid.Must(uuid.NewV7()), DeviceID: uuid.New()}
	if err := f.svc.WriteAudit(f.ctx, stranger, nil, nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("only a registered host writes audit: %v", err)
	}
}

func TestTransitionFactsAreRedactedOnServer(t *testing.T) {
	f := newFx(t)
	id := f.task()
	c := f.claim()
	f.move(c, StatusInProgress, Facts{Details: json.RawMessage(`{"agent":{"summary":"used ` + secretKey() + `"}}`)})
	var facts string
	_ = f.pool.QueryRow(f.ctx, `SELECT facts::text FROM exec.attempts WHERE task_id = $1`, id).Scan(&facts)
	if strings.Contains(facts, secretKey()) {
		t.Fatal(facts)
	}
}

func TestRejectWithCommentSendsTaskBackWithLesson(t *testing.T) {
	f := newFx(t)
	id := f.mergeable()
	approvals, _ := f.svc.ListApprovals(f.ctx, f.user, ApprovalPending)
	if len(approvals) != 1 || approvals[0].Type != "merge" || *approvals[0].TaskID != id {
		t.Fatalf("manual MERGEABLE creates a request: %+v", approvals)
	}
	if err := f.svc.ResolveApproval(f.ctx, f.user, approvals[0].ID, Decision{Decision: "reject"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("rejection needs a comment: %v", err)
	}
	if err := f.svc.ResolveApproval(f.ctx, f.user, approvals[0].ID,
		Decision{Decision: "reject", Comment: "выбор приоритета — рядом с полем ввода, без диалога"}); err != nil {
		t.Fatal(err)
	}
	st, _, maxAtt := f.status(id)
	if st != StatusQueued || maxAtt < 2 {
		t.Fatalf("%s %d", st, maxAtt)
	}
	c := f.claim()
	if c == nil || c.Attempt != 2 || !strings.Contains(string(c.PreviousAttempts[0].Facts), "рядом с полем ввода") {
		t.Fatalf("the owner's comment must reach the next attempt: %+v", c)
	}
}

func TestOldMergeButtonClosesTheRequest(t *testing.T) {
	f := newFx(t)
	id := f.mergeable()
	_ = f.svc.Merge(f.ctx, f.user, id)
	if pending, _ := f.svc.ListApprovals(f.ctx, f.user, ApprovalPending); len(pending) != 0 {
		t.Fatalf("%+v", pending)
	}
}

func TestPlanPartialApprovalAndReplan(t *testing.T) {
	f := newFx(t)
	f.registries([]string{"dart-clean-code", "flutter-widget-tests"}, []string{"web"}, []string{"claude", "junior"})
	id, c := f.plan("manual")
	_, _, _ = f.svc.SubmitPlan(f.ctx, f.host, id, f.worker, c.Attempt, goodProposal(), "")
	approvals, _ := f.svc.ListApprovals(f.ctx, f.user, ApprovalPending)
	if len(approvals) != 1 || approvals[0].Type != "plan" {
		t.Fatalf("%+v", approvals)
	}
	// Задача 1 зависит от 0: без неё выбрать нельзя.
	if err := f.svc.ResolveApproval(f.ctx, f.user, approvals[0].ID, Decision{Decision: "partial", Tasks: []int{1}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("dependency check: %v", err)
	}
	if err := f.svc.ResolveApproval(f.ctx, f.user, approvals[0].ID, Decision{Decision: "partial", Tasks: []int{0, 1}}); err != nil {
		t.Fatal(err)
	}
	if tasks, _ := f.svc.ListTasks(f.ctx, f.user, nil); len(tasks) != 2 {
		t.Fatalf("only chosen tasks: %d", len(tasks))
	}

	id2, c2 := f.plan("manual")
	_, _, _ = f.svc.SubmitPlan(f.ctx, f.host, id2, f.worker, c2.Attempt, goodProposal(), "")
	pending, _ := f.svc.ListApprovals(f.ctx, f.user, ApprovalPending)
	if err := f.svc.ResolveApproval(f.ctx, f.user, pending[0].ID, Decision{Decision: "reject", Comment: "слишком мелко, объедини"}); err != nil {
		t.Fatal(err)
	}
	again := f.claim()
	if again == nil || again.Kind != KindPlan || !strings.Contains(again.Prompt, "слишком мелко, объедини") {
		t.Fatalf("rejected plan with a comment goes back to the Manager: %+v", again)
	}
}
