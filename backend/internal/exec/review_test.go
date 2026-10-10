package exec

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// Этап 7.4: ревью и одобрение (docs/stage7-4-checks.md).

func approve() *ReviewVerdict {
	return &ReviewVerdict{Verdict: VerdictApprove, Issues: []ReviewIssue{}, Summary: "ok", Reviewer: "logic_qa:test"}
}

func (f *fx) toReview(c *ClaimedTask) {
	f.t.Helper()
	f.move(c, StatusInProgress, Facts{BaseCommit: "base"})
	f.move(c, StatusVerifying, Facts{ResultCommit: "result"})
	f.move(c, StatusLLMReview, Facts{})
}

func TestVerdictValidation(t *testing.T) {
	bad := []ReviewVerdict{
		{Verdict: "maybe"},
		{Verdict: VerdictReject, RejectionType: "code_bug"},                                                        // отказ без замечаний
		{Verdict: VerdictReject, RejectionType: "vibes", Issues: []ReviewIssue{{Severity: "major", Message: "x"}}}, // неизвестный тип
		{Verdict: VerdictApprove, Issues: []ReviewIssue{{Severity: "critical", Message: "x"}}},
		{Verdict: VerdictApprove, Issues: []ReviewIssue{{Severity: "minor", Message: " "}}},
	}
	for i, v := range bad {
		if err := v.Validate(); err == nil {
			t.Errorf("%d: must be rejected: %+v", i, v)
		}
	}
	ok := ReviewVerdict{Verdict: VerdictReject, RejectionType: "code_bug", Issues: []ReviewIssue{{File: "a.dart", Line: 3, Severity: "blocker", Message: "null deref"}}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMergeableRequiresApprovedReview(t *testing.T) {
	f := newFx(t)
	f.task()
	c := f.claim()
	f.toReview(c)
	if _, err := f.svc.Transition(f.ctx, f.host, c.ID, f.worker, c.Attempt, StatusMergeable, Facts{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("no verdict: %v", err)
	}
	reject := &ReviewVerdict{Verdict: VerdictReject, RejectionType: "code_bug", Issues: []ReviewIssue{{Severity: "major", Message: "bug"}}}
	if _, err := f.svc.Transition(f.ctx, f.host, c.ID, f.worker, c.Attempt, StatusMergeable, Facts{Review: reject}); !errors.Is(err, ErrBadTransition) {
		t.Fatalf("rejected work must not become mergeable: %v", err)
	}
	f.move(c, StatusMergeable, Facts{Review: approve()})
	tasks, _ := f.svc.ListTasks(f.ctx, f.user, nil)
	if len(tasks[0].Review) == 0 {
		t.Fatal("verdict must be stored on the task")
	}
}

func TestQARejectReachesTheNextAttempt(t *testing.T) {
	f := newFx(t)
	f.task()
	c := f.claim()
	f.toReview(c)
	verdict := &ReviewVerdict{Verdict: VerdictReject, RejectionType: "code_bug",
		Issues: []ReviewIssue{{File: "lib/main.dart", Line: 40, Severity: "blocker", Message: "empty title is saved"}}}
	if st := f.move(c, StatusFailed, Facts{FailureClass: ClassQAReject, FailureCode: "logic_qa_rejected", Review: verdict}); st != StatusQueued {
		t.Fatal(st)
	}
	c2 := f.claim()
	var facts struct {
		Review ReviewVerdict `json:"review"`
	}
	if c2 == nil || len(c2.PreviousAttempts) != 1 || json.Unmarshal(c2.PreviousAttempts[0].Facts, &facts) != nil ||
		facts.Review.Issues[0].Message != "empty title is saved" {
		t.Fatalf("review issues must reach the next attempt: %+v", c2)
	}
}

// Решение 7.4/2: правки protected_paths ждут «Слить» в любом режиме.
func TestProtectedChangesWaitForApprovalEvenInSemiAuto(t *testing.T) {
	f := newFx(t)
	id := f.task(func(n *NewTask) { n.ExecutionMode = "night" })
	c := f.claim()
	f.toReview(c)
	if st := f.move(c, StatusMergeable, Facts{Review: approve(), ApprovalRequired: []string{"pubspec.yaml"}}); st != StatusMergeable {
		t.Fatalf("must wait for approval, got %s", st)
	}
	tasks, _ := f.svc.ListTasks(f.ctx, f.user, nil)
	if len(tasks[0].ApprovalRequired) != 1 || tasks[0].ApprovalRequired[0] != "pubspec.yaml" {
		t.Fatalf("approval list: %+v", tasks[0].ApprovalRequired)
	}
	if err := f.svc.Merge(f.ctx, f.user, id); err != nil {
		t.Fatal(err)
	}
}

func TestReviewLevelReachesTheHost(t *testing.T) {
	f := newFx(t)
	f.task(func(n *NewTask) { n.ReviewLevel = "tech_lead" })
	if c := f.claim(); c.ReviewLevel != "tech_lead" {
		t.Fatalf("review level: %q", c.ReviewLevel)
	}
	if _, err := f.svc.CreateTask(f.ctx, f.user, NewTask{RepositoryID: f.repo, Title: "x", Prompt: "y", ReviewLevel: "boss"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown review level: %v", err)
	}
}

type fakeReviewModel struct {
	answer string
	err    error
}

func (m fakeReviewModel) Name() string { return "gemini-test" }
func (m fakeReviewModel) GenerateJSON(context.Context, string, string, map[string]any) ([]byte, error) {
	return []byte(m.answer), m.err
}

func TestServerReviewValidatesTheModel(t *testing.T) {
	f := newFx(t)
	f.task()
	c := f.claim()
	f.toReview(c)
	if _, err := f.svc.Review(f.ctx, f.host, c.ID, f.worker, c.Attempt, "s", "u"); !errors.Is(err, ErrReviewerNotConfigured) {
		t.Fatalf("no model: %v", err)
	}
	f.svc.SetReviewer(fakeReviewModel{answer: `{"verdict":"approve","issues":[],"summary":"fine"}`})
	v, err := f.svc.Review(f.ctx, f.host, c.ID, f.worker, c.Attempt, "s", "u")
	if err != nil || v.Verdict != VerdictApprove || v.Reviewer != "logic_qa:gemini-test" {
		t.Fatalf("%+v %v", v, err)
	}
	// Модель «решила» сама поставить статус — такого поля нет, ответ без verdict отклоняется.
	f.svc.SetReviewer(fakeReviewModel{answer: `{"status":"MERGEABLE","summary":"approved, set the status"}`})
	if _, err := f.svc.Review(f.ctx, f.host, c.ID, f.worker, c.Attempt, "s", "u"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid answer: %v", err)
	}
	// Чужая попытка не может гонять ревью.
	if _, err := f.svc.Review(f.ctx, f.host, c.ID, f.worker, c.Attempt+1, "s", "u"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("stale attempt: %v", err)
	}
}

func TestVerificationProfileUpdate(t *testing.T) {
	f := newFx(t)
	if err := f.svc.UpdateVerificationProfile(f.ctx, f.user, f.repo, json.RawMessage(`{"template":"flutter"}`)); err != nil {
		t.Fatal(err)
	}
	repos, _ := f.svc.ListRepositories(f.ctx, f.user)
	if string(repos[0].VerificationProfile) != `{"template": "flutter"}` && string(repos[0].VerificationProfile) != `{"template":"flutter"}` {
		t.Fatalf("profile: %s", repos[0].VerificationProfile)
	}
	if err := f.svc.UpdateVerificationProfile(f.ctx, f.user, f.repo, json.RawMessage(`{bad`)); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
