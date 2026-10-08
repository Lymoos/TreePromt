package tree

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"prompttree/backend/internal/testdb"
)

// ── fixture ──

type fixture struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	svc  *Service
	seq  atomic.Int64
}

func newFixture(t *testing.T) *fixture {
	pool := testdb.New(t)
	return &fixture{t: t, ctx: context.Background(), pool: pool,
		svc: NewService(pool, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))}
}

// user creates a user with two devices, as if logged in on a phone and a PC.
func (f *fixture) user(email string) (Actor, Actor) {
	f.t.Helper()
	uid := uuid.Must(uuid.NewV7())
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO core.users (id, email, password_hash) VALUES ($1, $2, 'x')`, uid, email); err != nil {
		f.t.Fatal(err)
	}
	return Actor{UserID: uid, DeviceID: uuid.New()}, Actor{UserID: uid, DeviceID: uuid.New()}
}

func (f *fixture) op(typ string, entity uuid.UUID, base int64, payload any) Op {
	b, err := json.Marshal(payload)
	if err != nil {
		f.t.Fatal(err)
	}
	return Op{OperationID: uuid.Must(uuid.NewV7()), ClientSeq: f.seq.Add(1), Type: typ, EntityID: entity, BaseRevision: base, Payload: b}
}

func (f *fixture) push(a Actor, ops ...Op) []Result {
	f.t.Helper()
	res, err := f.svc.Push(f.ctx, a, ops)
	if err != nil {
		f.t.Fatalf("push: %v", err)
	}
	return res
}

func (f *fixture) mustApply(a Actor, op Op) Result {
	f.t.Helper()
	r := f.push(a, op)[0]
	if r.Result != ResultApplied {
		f.t.Fatalf("%s: want applied, got %s %+v", op.Type, r.Result, r.Error)
	}
	return r
}

func (f *fixture) project(a Actor) uuid.UUID {
	id := uuid.Must(uuid.NewV7())
	f.mustApply(a, f.op(OpCreateProject, id, 0, map[string]any{"name": "PromptTree"}))
	return id
}

func (f *fixture) node(a Actor, project uuid.UUID, parent *uuid.UUID, kind, name, text string) NodeState {
	id := uuid.Must(uuid.NewV7())
	r := f.mustApply(a, f.op(OpCreateNode, id, 0, map[string]any{
		"project_id": project, "parent_id": parent, "kind": kind, "name": name, "sort_key": "a0", "raw_content": text}))
	return *r.Node
}

func (f *fixture) versions(node uuid.UUID) map[string]string {
	rows, err := f.pool.Query(f.ctx, `SELECT reason, content FROM tree.content_versions WHERE node_id = $1`, node)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var reason, content string
		if err := rows.Scan(&reason, &content); err != nil {
			f.t.Fatal(err)
		}
		out[reason] = content
	}
	return out
}

func (f *fixture) count(query string, args ...any) int {
	var n int
	if err := f.pool.QueryRow(f.ctx, query, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

// ── сценарии из ТЗ п. 16 и архитектуры п. 4.3 ──

func TestOfflineTextConflictKeepsBothVersions(t *testing.T) {
	f := newFixture(t)
	phone, pc := f.user("max@example.com")
	p := f.project(phone)
	n := f.node(phone, p, nil, KindAITask, "Дерево", "исходный текст")
	base := n.RawRevision // обе стороны видели эту ревизию и ушли в офлайн

	f.mustApply(phone, f.op(OpSetRawContent, n.ID, base, map[string]any{"content": "правка с телефона"}))
	r := f.push(pc, f.op(OpSetRawContent, n.ID, base, map[string]any{"content": "правка с ПК 💻"}))[0]

	if r.Result != ResultConflict {
		t.Fatalf("want conflict, got %s", r.Result)
	}
	if r.Node == nil || !r.Node.HasConflict || r.Node.RawContent != "правка с телефона" {
		t.Fatalf("server text must stay and be marked: %+v", r.Node)
	}
	v := f.versions(n.ID)
	if v["conflict_server"] != "правка с телефона" || v["conflict_local"] != "правка с ПК 💻" {
		t.Fatalf("both versions must be kept: %v", v)
	}

	// Оба устройства получают обе версии через pull.
	res, err := f.svc.Pull(f.ctx, pc.UserID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Versions) != 2 {
		t.Fatalf("pull must deliver both versions, got %d", len(res.Versions))
	}

	r = f.mustApply(pc, f.op(OpResolveConflict, n.ID, 0, map[string]any{"content": "итог"}))
	if r.Node.HasConflict || r.Node.RawContent != "итог" {
		t.Fatalf("resolve: %+v", r.Node)
	}
}

func TestSameDeviceSequentialEditsAreNotAConflict(t *testing.T) {
	f := newFixture(t)
	phone, _ := f.user("max@example.com")
	p := f.project(phone)
	n := f.node(phone, p, nil, KindRawNote, "Черновик", "v0")
	// Вторая правка отправлена до получения ответа на первую: base у обеих старый.
	res := f.push(phone,
		f.op(OpSetRawContent, n.ID, n.RawRevision, map[string]any{"content": "v1"}),
		f.op(OpSetRawContent, n.ID, n.RawRevision, map[string]any{"content": "v2"}))
	for _, r := range res {
		if r.Result != ResultApplied {
			t.Fatalf("want applied, got %s %+v", r.Result, r.Error)
		}
	}
	if res[1].Node.RawContent != "v2" {
		t.Fatalf("got %q", res[1].Node.RawContent)
	}
}

func TestDifferentFieldsMergeWithoutConflict(t *testing.T) {
	f := newFixture(t)
	phone, pc := f.user("max@example.com")
	p := f.project(phone)
	n := f.node(phone, p, nil, KindAITask, "Старое имя", "текст")

	f.mustApply(phone, f.op(OpRenameNode, n.ID, n.Revision, map[string]any{"name": "Новое имя"}))
	r := f.mustApply(pc, f.op(OpSetRawContent, n.ID, n.RawRevision, map[string]any{"content": "новый текст"}))
	if r.Node.Name != "Новое имя" || r.Node.RawContent != "новый текст" || r.Node.HasConflict {
		t.Fatalf("both changes must survive: %+v", r.Node)
	}
}

func TestRenameLastWinsAndLoserIsAudited(t *testing.T) {
	f := newFixture(t)
	phone, pc := f.user("max@example.com")
	p := f.project(phone)
	n := f.node(phone, p, nil, KindRawNote, "Имя", "")

	f.mustApply(phone, f.op(OpRenameNode, n.ID, n.Revision, map[string]any{"name": "С телефона"}))
	r := f.mustApply(pc, f.op(OpRenameNode, n.ID, n.Revision, map[string]any{"name": "С ПК"}))
	if r.Node.Name != "С ПК" {
		t.Fatalf("got %q", r.Node.Name)
	}
	if f.count(`SELECT count(*) FROM tree.audit_events WHERE entity_id = $1 AND action = 'rename' AND before->>'name' = 'С телефона'`, n.ID) != 1 {
		t.Fatal("overwritten name must be kept in audit")
	}
}

func TestReplayedOperationIsAppliedOnce(t *testing.T) {
	f := newFixture(t)
	phone, _ := f.user("max@example.com")
	p := f.project(phone)
	n := f.node(phone, p, nil, KindRawNote, "Заметка", "")

	op := f.op(OpSetRawContent, n.ID, n.RawRevision, map[string]any{"content": "один раз"})
	r1 := f.mustApply(phone, op)
	r2 := f.mustApply(phone, op) // ответ потерялся в сети, клиент повторил
	if *r1.ServerRevision != *r2.ServerRevision {
		t.Fatalf("replay got a new revision: %d vs %d", *r1.ServerRevision, *r2.ServerRevision)
	}
	if f.count(`SELECT count(*) FROM tree.operations WHERE operation_id = $1`, op.OperationID) != 1 {
		t.Fatal("operation stored twice")
	}
	if strings.Contains(string(f.payload(op.OperationID)), "один раз") {
		t.Fatal("note text must not be stored in the operation log")
	}
}

func (f *fixture) payload(id uuid.UUID) []byte {
	var b []byte
	if err := f.pool.QueryRow(f.ctx, `SELECT payload FROM tree.operations WHERE operation_id = $1`, id).Scan(&b); err != nil {
		f.t.Fatal(err)
	}
	return b
}

func TestMoveIntoOwnDescendantIsRejected(t *testing.T) {
	f := newFixture(t)
	a, _ := f.user("max@example.com")
	p := f.project(a)
	outer := f.node(a, p, nil, KindFolder, "Внешняя", "")
	inner := f.node(a, p, &outer.ID, KindFolder, "Внутренняя", "")

	r := f.push(a, f.op(OpMoveNode, outer.ID, 0, map[string]any{"parent_id": inner.ID, "sort_key": "b"}))[0]
	if r.Result != ResultRejected || r.Error.Code != CodeCycle {
		t.Fatalf("want cycle rejection, got %s %+v", r.Result, r.Error)
	}
	if r.Node == nil || r.Node.ParentID != nil {
		t.Fatalf("node must stay in place and be returned for rebase: %+v", r.Node)
	}
}

func TestMoveIntoNoteIsRejected(t *testing.T) {
	f := newFixture(t)
	a, _ := f.user("max@example.com")
	p := f.project(a)
	note := f.node(a, p, nil, KindRawNote, "Заметка", "")
	other := f.node(a, p, nil, KindRawNote, "Другая", "")
	r := f.push(a, f.op(OpMoveNode, other.ID, 0, map[string]any{"parent_id": note.ID, "sort_key": "b"}))[0]
	if r.Result != ResultRejected || r.Error.Code != CodeInvalidParent {
		t.Fatalf("got %s %+v", r.Result, r.Error)
	}
}

func TestEditInDeletedFolderRestoresIt(t *testing.T) {
	f := newFixture(t)
	phone, pc := f.user("max@example.com")
	p := f.project(phone)
	folder := f.node(phone, p, nil, KindFolder, "UI", "")
	note := f.node(phone, p, &folder.ID, KindAITask, "Дерево", "текст")

	f.mustApply(phone, f.op(OpDeleteNode, folder.ID, 0, nil))
	f.mustApply(pc, f.op(OpSetRawContent, note.ID, note.RawRevision, map[string]any{"content": "правка офлайн"}))

	if f.count(`SELECT count(*) FROM tree.nodes WHERE id IN ($1, $2) AND deleted_at IS NULL`, folder.ID, note.ID) != 2 {
		t.Fatal("folder and note must be restored")
	}
}

func TestCreateInDeletedFolderRestoresIt(t *testing.T) {
	f := newFixture(t)
	phone, pc := f.user("max@example.com")
	p := f.project(phone)
	folder := f.node(phone, p, nil, KindFolder, "Идеи", "")
	f.mustApply(phone, f.op(OpDeleteNode, folder.ID, 0, nil))
	f.node(pc, p, &folder.ID, KindRawNote, "Мысль офлайн", "не потерять")
	if f.count(`SELECT count(*) FROM tree.nodes WHERE id = $1 AND deleted_at IS NULL`, folder.ID) != 1 {
		t.Fatal("folder must be restored")
	}
}

func TestRejectedOperationLeavesNoPartialWrites(t *testing.T) {
	f := newFixture(t)
	a, _ := f.user("max@example.com")
	p := f.project(a)
	id := uuid.Must(uuid.NewV7())
	missing := uuid.New()
	r := f.push(a, f.op(OpCreateNode, id, 0, map[string]any{
		"project_id": p, "parent_id": missing, "kind": KindRawNote, "name": "x", "sort_key": "a"}))[0]
	if r.Result != ResultRejected {
		t.Fatalf("got %s", r.Result)
	}
	if f.count(`SELECT count(*) FROM tree.nodes WHERE id = $1`, id) != 0 {
		t.Fatal("rejected create left a node")
	}
	if f.count(`SELECT count(*) FROM tree.operations WHERE entity_id = $1 AND result = 'rejected'`, id) != 1 {
		t.Fatal("rejected op must be logged for idempotency")
	}
}

func TestFolderCannotBecomeNote(t *testing.T) {
	f := newFixture(t)
	a, _ := f.user("max@example.com")
	p := f.project(a)
	folder := f.node(a, p, nil, KindFolder, "Папка", "")
	r := f.push(a, f.op(OpChangeKind, folder.ID, 0, map[string]any{"kind": KindAITask}))[0]
	if r.Result != ResultRejected || r.Error.Code != CodeKindChange {
		t.Fatalf("got %s %+v", r.Result, r.Error)
	}
	note := f.node(a, p, nil, KindRawNote, "Заметка", "")
	f.mustApply(a, f.op(OpChangeKind, note.ID, 0, map[string]any{"kind": KindAITask}))
}

func TestOtherUserCannotSeeOrChangeProject(t *testing.T) {
	f := newFixture(t)
	owner, _ := f.user("max@example.com")
	stranger, _ := f.user("other@example.com")
	p := f.project(owner)
	n := f.node(owner, p, nil, KindRawNote, "Личное", "секрет")

	r := f.push(stranger, f.op(OpSetRawContent, n.ID, n.RawRevision, map[string]any{"content": "взлом"}))[0]
	if r.Result != ResultRejected || r.Error.Code != CodeNotFound {
		t.Fatalf("got %s %+v", r.Result, r.Error)
	}
	if r.Node != nil {
		t.Fatal("stranger must not receive node state")
	}
	res, err := f.svc.Pull(f.ctx, stranger.UserID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Projects)+len(res.Nodes)+len(res.Versions) != 0 {
		t.Fatalf("stranger sees data: %+v", res)
	}
	// Чужой operation_id нельзя «переиграть».
	op := f.op(OpRenameNode, n.ID, 0, map[string]any{"name": "мой"})
	f.mustApply(owner, op)
	if r := f.push(stranger, op)[0]; r.Result != ResultRejected || r.Node != nil {
		t.Fatalf("replay by stranger: %s %+v", r.Result, r.Node)
	}
}

func TestRestoreVersionKeepsCurrentText(t *testing.T) {
	f := newFixture(t)
	phone, pc := f.user("max@example.com")
	p := f.project(phone)
	n := f.node(phone, p, nil, KindAITask, "Задача", "v0")
	f.mustApply(phone, f.op(OpSetRawContent, n.ID, n.RawRevision, map[string]any{"content": "телефон"}))
	f.push(pc, f.op(OpSetRawContent, n.ID, n.RawRevision, map[string]any{"content": "ПК"}))

	var vid uuid.UUID
	if err := f.pool.QueryRow(f.ctx, `SELECT id FROM tree.content_versions WHERE node_id = $1 AND reason = 'conflict_local'`, n.ID).Scan(&vid); err != nil {
		t.Fatal(err)
	}
	r := f.mustApply(pc, f.op(OpRestoreVersion, n.ID, 0, map[string]any{"version_id": vid}))
	if r.Node.RawContent != "ПК" {
		t.Fatalf("got %q", r.Node.RawContent)
	}
	if f.versions(n.ID)["before_restore"] != "телефон" {
		t.Fatal("text before restore must be saved as a version")
	}
}

func TestEditMarksStructureStale(t *testing.T) {
	f := newFixture(t)
	a, _ := f.user("max@example.com")
	p := f.project(a)
	n := f.node(a, p, nil, KindAITask, "Задача", "v0")
	if _, err := f.pool.Exec(f.ctx, `UPDATE tree.node_content SET structure_status = 'done' WHERE node_id = $1`, n.ID); err != nil {
		t.Fatal(err)
	}
	r := f.mustApply(a, f.op(OpSetRawContent, n.ID, n.RawRevision, map[string]any{"content": "v1"}))
	if r.Node.StructureStatus != "stale" {
		t.Fatalf("got %q", r.Node.StructureStatus)
	}
}

func TestCheckpointAfterEditingPause(t *testing.T) {
	f := newFixture(t)
	a, _ := f.user("max@example.com")
	p := f.project(a)
	n := f.node(a, p, nil, KindRawNote, "Заметка", "утренний текст")
	f.svc.now = func() time.Time { return time.Now().Add(checkpointAfter + time.Minute) }
	f.mustApply(a, f.op(OpSetRawContent, n.ID, n.RawRevision, map[string]any{"content": "вечерний текст"}))
	if f.versions(n.ID)["checkpoint"] != "утренний текст" {
		t.Fatal("checkpoint version expected after a pause")
	}
}

func TestLargeUnicodeContentRoundTrip(t *testing.T) {
	f := newFixture(t)
	a, _ := f.user("max@example.com")
	p := f.project(a)
	text := strings.Repeat("Кириллица, emoji 🌳🤖 и\nпереносы строк. ", 25_000) // ~1 МБ
	n := f.node(a, p, nil, KindRawNote, "Большая", "")
	r := f.mustApply(a, f.op(OpSetRawContent, n.ID, n.RawRevision, map[string]any{"content": text}))
	if r.Node.RawContent != text {
		t.Fatal("text changed in round trip")
	}
}

func TestPullPagesWithoutLossOrDuplicates(t *testing.T) {
	f := newFixture(t)
	a, _ := f.user("max@example.com")
	p := f.project(a)
	want := map[uuid.UUID]bool{}
	for i := 0; i < 25; i++ {
		want[f.node(a, p, nil, KindRawNote, "n", "").ID] = true
	}
	got := map[uuid.UUID]bool{}
	var cursor int64
	for pages := 0; ; pages++ {
		if pages > 20 {
			t.Fatal("pull does not terminate")
		}
		res, err := f.svc.Pull(f.ctx, a.UserID, cursor, 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range res.Nodes {
			if got[n.ID] {
				t.Fatalf("duplicate node %s", n.ID)
			}
			got[n.ID] = true
		}
		if res.Cursor < cursor {
			t.Fatal("cursor went backwards")
		}
		cursor = res.Cursor
		if !res.HasMore {
			break
		}
	}
	if len(got) != len(want) {
		t.Fatalf("got %d nodes, want %d", len(got), len(want))
	}
}

// Писатели и читатель работают одновременно: читатель, двигаясь по курсору,
// не должен пропустить ни одного изменения.
func TestConcurrentWritersPullNeverMisses(t *testing.T) {
	f := newFixture(t)
	a, _ := f.user("max@example.com")
	p := f.project(a)

	const writers, perWriter = 6, 15
	created := make(chan uuid.UUID, writers*perWriter)
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dev := Actor{UserID: a.UserID, DeviceID: uuid.New()}
			for i := 0; i < perWriter; i++ {
				id := uuid.Must(uuid.NewV7())
				res, err := f.svc.Push(f.ctx, dev, []Op{f.op(OpCreateNode, id, 0, map[string]any{
					"project_id": p, "kind": KindRawNote, "name": "c", "sort_key": "a"})})
				if err != nil || res[0].Result != ResultApplied {
					t.Errorf("push: %v %+v", err, res)
					return
				}
				created <- id
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	seen := map[uuid.UUID]bool{}
	var cursor int64
	pull := func() {
		res, err := f.svc.Pull(f.ctx, a.UserID, cursor, 7)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range res.Nodes {
			seen[n.ID] = true
		}
		cursor = res.Cursor
	}
	for running := true; running; {
		select {
		case <-done:
			running = false
		default:
			pull()
		}
	}
	for i := 0; i < 50; i++ { // дочитать хвост
		pull()
	}
	close(created)
	for id := range created {
		if !seen[id] {
			t.Fatalf("pull missed node %s", id)
		}
	}
}
