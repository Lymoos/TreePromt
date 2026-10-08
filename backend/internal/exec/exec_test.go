package exec

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"prompttree/backend/internal/testdb"
)

func TestTransitionTable(t *testing.T) {
	allowed := [][2]string{
		{StatusClaimed, StatusInProgress}, {StatusInProgress, StatusVerifying}, {StatusVerifying, StatusLLMReview},
		{StatusLLMReview, StatusMergeable}, {StatusInProgress, StatusFailed}, {StatusCancelling, StatusCancelled},
		{StatusMerging, StatusPostMergeVerify}, {StatusPostMergeVerify, StatusMerged}, {StatusMerged, StatusDone},
		{StatusRollingBack, StatusRolledBack},
	}
	for _, p := range allowed {
		if !WorkerCanMove(p[0], p[1]) {
			t.Errorf("%s → %s must be allowed", p[0], p[1])
		}
	}
	forbidden := [][2]string{
		{StatusQueued, StatusInProgress}, {StatusClaimed, StatusMergeable}, {StatusInProgress, StatusDone},
		{StatusVerifying, StatusMergeable}, {StatusDone, StatusQueued}, {StatusCancelled, StatusInProgress},
		{StatusCancelling, StatusVerifying},
		// Сливать может только очередь: воркер не двигает MERGEABLE сам и не пропускает проверку слияния.
		{StatusMergeable, StatusMerged}, {StatusMergeable, StatusMergeQueued}, {StatusMerged, StatusPostMergeVerify},
		{StatusRolledBack, StatusDone},
	}
	for _, p := range forbidden {
		if WorkerCanMove(p[0], p[1]) {
			t.Errorf("%s → %s must be forbidden", p[0], p[1])
		}
	}
}

func TestRetryPolicy(t *testing.T) {
	cases := []struct {
		class   string
		attempt int
		want    string
		delay   bool
		spend   bool
	}{
		{ClassAuth, 1, StatusAwaitingHuman, false, false},
		{ClassMerge, 1, StatusQueued, false, true}, // 7.3: конфликт → переделать поверх integration
		{ClassIntegration, 3, StatusAwaitingHuman, false, false},
		{ClassRateLimit, 3, StatusQueued, true, false},
		{ClassNetwork, 1, StatusQueued, true, true},
		{ClassTest, 1, StatusQueued, false, true},
		{ClassTest, 3, StatusAwaitingHuman, false, false},
	}
	for _, c := range cases {
		d := decideRetry(c.class, c.attempt, 3, nil)
		if d.status != c.want || (d.delay > 0) != c.delay || (c.want == StatusQueued && d.spendAttempt != c.spend) {
			t.Errorf("%s attempt %d: got %+v", c.class, c.attempt, d)
		}
	}
}

// ── на базе ──

type fx struct {
	t       *testing.T
	ctx     context.Context
	pool    *pgxpool.Pool
	svc     *Service
	user    uuid.UUID
	host    Identity
	worker  uuid.UUID
	project uuid.UUID
	repo    uuid.UUID
	clock   time.Time
}

func newFx(t *testing.T) *fx {
	pool := testdb.New(t)
	f := &fx{t: t, ctx: context.Background(), pool: pool, clock: time.Now()}
	f.svc = NewService(pool, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	f.svc.now = func() time.Time { return f.clock }
	f.user = uuid.Must(uuid.NewV7())
	f.exec(`INSERT INTO core.users (id, email, password_hash) VALUES ($1, $2, 'x')`, f.user, f.user.String()+"@e.com")
	f.host = f.device(f.user)
	f.project = uuid.Must(uuid.NewV7())
	f.exec(`INSERT INTO tree.projects (id, name, revision, created_by) VALUES ($1, 'Sandbox', 1, $2)`, f.project, f.user)
	f.exec(`INSERT INTO core.project_access (project_id, user_id, role) VALUES ($1, $2, 'owner')`, f.project, f.user)
	h, err := f.svc.RegisterHost(f.ctx, f.host, "home-pc", "windows", []string{"flutter", "go"}, []string{"container"},
		[]WorkerSpec{{Kind: "claude_code", Model: "sonnet"}})
	if err != nil {
		t.Fatal(err)
	}
	f.worker = uuid.MustParse(h.Workers["claude_code/sonnet"])
	r, err := f.svc.CreateRepository(f.ctx, f.user, Repository{ProjectID: f.project, HostID: h.ID, Name: "sandbox", LocalPath: `C:\repos\sandbox`})
	if err != nil {
		t.Fatal(err)
	}
	f.repo = r.ID
	return f
}

func (f *fx) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(f.ctx, sql, args...); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fx) device(user uuid.UUID) Identity {
	d := uuid.New()
	f.exec(`INSERT INTO core.devices (id, user_id) VALUES ($1, $2)`, d, user)
	return Identity{UserID: user, DeviceID: d}
}

func (f *fx) task(opts ...func(*NewTask)) uuid.UUID {
	f.t.Helper()
	nt := NewTask{RepositoryID: f.repo, Title: "Добавить кнопку", Prompt: "Act as X.\n\nДобавить кнопку"}
	for _, o := range opts {
		o(&nt)
	}
	id, err := f.svc.CreateTask(f.ctx, f.user, nt)
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *fx) status(id uuid.UUID) (status string, attempt, maxAttempts int) {
	f.t.Helper()
	if err := f.pool.QueryRow(f.ctx, `SELECT status, attempt, max_attempts FROM exec.tasks WHERE id = $1`, id).Scan(&status, &attempt, &maxAttempts); err != nil {
		f.t.Fatal(err)
	}
	return
}

func (f *fx) claim() *ClaimedTask {
	f.t.Helper()
	c, err := f.svc.Claim(f.ctx, f.host, f.worker)
	if err != nil {
		f.t.Fatal(err)
	}
	return c
}

func (f *fx) move(c *ClaimedTask, to string, facts Facts) string {
	f.t.Helper()
	st, err := f.svc.Transition(f.ctx, f.host, c.ID, f.worker, c.Attempt, to, facts)
	if err != nil {
		f.t.Fatalf("%s: %v", to, err)
	}
	return st
}

func TestHappyPathToMergeable(t *testing.T) {
	f := newFx(t)
	id := f.task()
	c := f.claim()
	if c == nil || c.ID != id || c.Attempt != 1 || c.Repository.LocalPath != `C:\repos\sandbox` || c.Prompt == "" {
		t.Fatalf("claim: %+v", c)
	}
	f.move(c, StatusInProgress, Facts{BaseCommit: "aaa"})
	f.move(c, StatusVerifying, Facts{ResultCommit: "bbb", Details: json.RawMessage(`{"changed_files":["lib/main.dart"]}`)})
	f.move(c, StatusLLMReview, Facts{})
	f.move(c, StatusMergeable, Facts{Reason: "review skipped until 7.4"})
	st, _, _ := f.status(id)
	if st != StatusMergeable {
		t.Fatal(st)
	}
	var facts []byte
	var base, result string
	_ = f.pool.QueryRow(f.ctx, `SELECT base_commit, result_commit, facts FROM exec.attempts WHERE task_id = $1`, id).Scan(&base, &result, &facts)
	if base != "aaa" || result != "bbb" || len(facts) < 10 {
		t.Fatalf("attempt facts: %s %s %s", base, result, facts)
	}
	var events int
	_ = f.pool.QueryRow(f.ctx, `SELECT count(*) FROM exec.task_events WHERE task_id = $1`, id).Scan(&events)
	if events != 6 {
		t.Fatalf("every transition must be audited, got %d events", events)
	}
	// Воркер освободился: задача вне lease.
	if c2 := f.claim(); c2 != nil {
		t.Fatal("nothing else to claim")
	}
}

// ТЗ п. 16: атомарность CLAIMED — параллельные захваты не берут одну задачу дважды.
func TestConcurrentClaimsNeverShareATask(t *testing.T) {
	f := newFx(t)
	const tasks = 5
	for i := 0; i < tasks; i++ {
		f.task()
	}
	var mu sync.Mutex
	got := map[uuid.UUID]int{}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := f.svc.Claim(f.ctx, f.host, f.worker)
			if err != nil {
				t.Error(err)
				return
			}
			if c != nil {
				mu.Lock()
				got[c.ID]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(got) != tasks {
		t.Fatalf("claimed %d distinct tasks, want %d", len(got), tasks)
	}
	for id, n := range got {
		if n != 1 {
			t.Fatalf("task %s claimed %d times", id, n)
		}
	}
}

// ТЗ п. 16: убитый воркер — lease истёк, задача вернулась в очередь.
func TestKilledWorkerTaskIsRequeued(t *testing.T) {
	f := newFx(t)
	id := f.task()
	c := f.claim()
	f.move(c, StatusInProgress, Facts{BaseCommit: "aaa"})

	f.clock = f.clock.Add(LeaseTTL - 10*time.Second)
	if n, _ := f.svc.ReapExpired(f.ctx); n != 0 {
		t.Fatal("lease not expired yet")
	}
	f.clock = f.clock.Add(20 * time.Second)
	if n, err := f.svc.ReapExpired(f.ctx); err != nil || n != 1 {
		t.Fatalf("reaped %d, %v", n, err)
	}
	st, attempt, _ := f.status(id)
	if st != StatusQueued || attempt != 1 {
		t.Fatalf("status %s attempt %d", st, attempt)
	}
	// Старый воркер «проснулся» — его переходы и heartbeat отклоняются.
	if _, err := f.svc.Transition(f.ctx, f.host, id, f.worker, 1, StatusVerifying, Facts{}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("stale worker: %v", err)
	}
	// Повтор через минуту — следующая попытка видит факты прошлой.
	f.clock = f.clock.Add(2 * time.Minute)
	c2 := f.claim()
	if c2 == nil || c2.Attempt != 2 || len(c2.PreviousAttempts) != 1 || *c2.PreviousAttempts[0].FailureCode != "lease_expired" {
		t.Fatalf("retry claim: %+v", c2)
	}
}

func TestHeartbeatKeepsLeaseAlive(t *testing.T) {
	f := newFx(t)
	f.task()
	c := f.claim()
	for i := 0; i < 5; i++ {
		f.clock = f.clock.Add(90 * time.Second)
		if _, _, err := f.svc.Heartbeat(f.ctx, f.host, c.ID, f.worker, c.Attempt); err != nil {
			t.Fatal(err)
		}
		if n, _ := f.svc.ReapExpired(f.ctx); n != 0 {
			t.Fatal("heartbeat must prevent reaping")
		}
	}
}

func TestFailuresFollowClassPolicy(t *testing.T) {
	f := newFx(t)
	id := f.task()

	// Тесты красные трижды → человек.
	for attempt := 1; attempt <= 3; attempt++ {
		c := f.claim()
		if c == nil || c.Attempt != attempt {
			t.Fatalf("attempt %d: %+v", attempt, c)
		}
		f.move(c, StatusInProgress, Facts{})
		st := f.move(c, StatusFailed, Facts{FailureClass: ClassTest, FailureCode: "flutter_test_failed"})
		want := StatusQueued
		if attempt == 3 {
			want = StatusAwaitingHuman
		}
		if st != want {
			t.Fatalf("attempt %d: %s, want %s", attempt, st, want)
		}
	}
	if err := f.svc.Retry(f.ctx, f.user, id); err != nil {
		t.Fatal(err)
	}
	if c := f.claim(); c == nil || c.Attempt != 4 {
		t.Fatalf("user retry must give one more attempt: %+v", c)
	}
}

func TestAuthErrorGoesStraightToHuman(t *testing.T) {
	f := newFx(t)
	f.task()
	c := f.claim()
	if st := f.move(c, StatusFailed, Facts{FailureClass: ClassAuth, Reason: "claude token expired"}); st != StatusAwaitingHuman {
		t.Fatal(st)
	}
}

func TestRateLimitDoesNotSpendAttemptsAndWaits(t *testing.T) {
	f := newFx(t)
	id := f.task()
	c := f.claim()
	if st := f.move(c, StatusFailed, Facts{FailureClass: ClassRateLimit, RetryAfterSec: 600}); st != StatusQueued {
		t.Fatal(st)
	}
	if f.claim() != nil {
		t.Fatal("must wait retry_after")
	}
	f.clock = f.clock.Add(11 * time.Minute)
	c2 := f.claim()
	_, _, maxAtt := f.status(id)
	if c2 == nil || maxAtt != 4 {
		t.Fatalf("rate limit must not consume the attempt budget: claim=%+v max=%d", c2, maxAtt)
	}
}

func TestUnknownFailureClassIsRejected(t *testing.T) {
	f := newFx(t)
	f.task()
	c := f.claim()
	if _, err := f.svc.Transition(f.ctx, f.host, c.ID, f.worker, c.Attempt, StatusFailed, Facts{FailureClass: "OOPS"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestCancelInEveryStage(t *testing.T) {
	f := newFx(t)
	// Не взята — отменяется сразу.
	queued := f.task()
	if st, _ := f.svc.Cancel(f.ctx, f.user, queued); st != StatusCancelled {
		t.Fatal(st)
	}
	// Взята — CANCELLING, хост узнаёт из heartbeat и подтверждает.
	running := f.task()
	c := f.claim()
	if c.ID != running {
		t.Fatal("wrong task")
	}
	f.move(c, StatusInProgress, Facts{})
	if st, _ := f.svc.Cancel(f.ctx, f.user, running); st != StatusCancelling {
		t.Fatal(st)
	}
	if _, cancel, _ := f.svc.Heartbeat(f.ctx, f.host, running, f.worker, c.Attempt); !cancel {
		t.Fatal("heartbeat must report cancellation")
	}
	if _, err := f.svc.Transition(f.ctx, f.host, running, f.worker, c.Attempt, StatusVerifying, Facts{}); !errors.Is(err, ErrBadTransition) {
		t.Fatalf("work must not continue after cancel: %v", err)
	}
	f.move(c, StatusCancelled, Facts{Reason: "agent stopped"})
	// Хост умер во время отмены — сборщик завершает отмену сам.
	dead := f.task()
	c3 := f.claim()
	_, _ = f.svc.Cancel(f.ctx, f.user, dead)
	f.clock = f.clock.Add(LeaseTTL + time.Second)
	_, _ = f.svc.ReapExpired(f.ctx)
	if st, _, _ := f.status(c3.ID); st != StatusCancelled {
		t.Fatal(st)
	}
}

func TestDependenciesGateClaiming(t *testing.T) {
	f := newFx(t)
	first := f.task()
	second := f.task(func(n *NewTask) { n.DependsOn = []uuid.UUID{first} })
	c := f.claim()
	if c.ID != first {
		t.Fatal("first task must be claimed first")
	}
	if f.claim() != nil {
		t.Fatal("dependent task must wait")
	}
	f.exec(`UPDATE exec.tasks SET status = 'DONE' WHERE id = $1`, first)
	if c2 := f.claim(); c2 == nil || c2.ID != second {
		t.Fatalf("dependent task after DONE: %+v", c2)
	}
}

func TestEnvironmentAndCapabilitiesAreRespected(t *testing.T) {
	f := newFx(t)
	f.task(func(n *NewTask) { n.ExecutionEnv = "windows_user" })                    // хост умеет только container
	f.task(func(n *NewTask) { n.RequiredCapabilities = []string{"roblox_studio"} }) // и не умеет Roblox
	if f.claim() != nil {
		t.Fatal("host must not get tasks it cannot run")
	}
}

func TestStrangersCannotTouchTasks(t *testing.T) {
	f := newFx(t)
	id := f.task()
	c := f.claim()
	stranger := uuid.Must(uuid.NewV7())
	f.exec(`INSERT INTO core.users (id, email, password_hash) VALUES ($1, 'x@e.com', 'x')`, stranger)
	other := f.device(stranger)
	if _, err := f.svc.Transition(f.ctx, other, id, f.worker, c.Attempt, StatusInProgress, Facts{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign device: %v", err)
	}
	if _, err := f.svc.Cancel(f.ctx, stranger, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign user cancel: %v", err)
	}
	if tasks, _ := f.svc.ListTasks(f.ctx, stranger, nil); len(tasks) != 0 {
		t.Fatal("stranger sees tasks")
	}
}

func TestTaskPromptIsSnapshotOfFinalTask(t *testing.T) {
	f := newFx(t)
	node := uuid.Must(uuid.NewV7())
	f.exec(`INSERT INTO tree.nodes (id, project_id, kind, name, sort_key, revision) VALUES ($1, $2, 'ai_task', 'Кнопка', 'V', 1)`, node, f.project)
	f.exec(`INSERT INTO tree.node_content (node_id, raw_content, structured_content) VALUES ($1, 'сырой текст',
		'{"role":"Act as a Flutter dev.","formatted_text":"## Задача\n- Кнопка"}')`, node)
	id := f.task(func(n *NewTask) { n.Title, n.Prompt, n.NodeID = "", "", &node })
	c := f.claim()
	if c.ID != id || c.Title != "Кнопка" || c.Prompt != "Act as a Flutter dev.\n\n## Задача\n- Кнопка" {
		t.Fatalf("prompt snapshot: %q %q", c.Title, c.Prompt)
	}
}

func TestKeepWorktreesListsLeasedAndMergeable(t *testing.T) {
	f := newFx(t)
	a, b := f.task(), f.task()
	ca := f.claim()
	f.move(ca, StatusInProgress, Facts{})
	f.move(ca, StatusFailed, Facts{FailureClass: ClassAuth}) // AWAITING_HUMAN: worktree не нужен
	cb := f.claim()
	if cb.ID != b {
		t.Fatal("order")
	}
	ids, err := f.svc.KeepWorktrees(f.ctx, f.host)
	if err != nil || len(ids) != 1 || ids[0] != b {
		t.Fatalf("keep %v (a=%s) %v", ids, a, err)
	}
}
