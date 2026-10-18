package exec

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Этап 7.9: окно проекта, скиллы задачи, привязка кода (docs/stage7-9-projects.md).

func TestProjectContextDescriptionAndFiles(t *testing.T) {
	f := newFx(t)
	if err := f.svc.SetProjectDescription(f.ctx, f.user, f.project, "  Игра в Roblox для детей 8–12 лет.  "); err != nil {
		t.Fatal(err)
	}
	file, err := f.svc.AddProjectFile(f.ctx, f.user, f.project, `..\..\ТЗ: v2.md`, "text/markdown", []byte("# ТЗ"))
	if err != nil {
		t.Fatal(err)
	}
	if file.Name != "ТЗ_ v2.md" || file.Size != 6 || len(file.SHA256) != 64 {
		t.Fatalf("file name must lose paths and unsafe characters: %+v", file)
	}
	pc, err := f.svc.ProjectContext(f.ctx, f.user, f.project)
	if err != nil || pc.Description != "Игра в Roblox для детей 8–12 лет." || len(pc.Files) != 1 {
		t.Fatalf("%+v %v", pc, err)
	}
	_, data, err := f.svc.ProjectFileData(f.ctx, f.user, f.project, file.ID)
	if err != nil || string(data) != "# ТЗ" {
		t.Fatalf("%q %v", data, err)
	}

	// Лимиты: файл больше 20 МБ и сумма файлов проекта больше 200 МБ.
	if _, err := f.svc.AddProjectFile(f.ctx, f.user, f.project, "big.bin", "", bytes.Repeat([]byte{1}, MaxProjectFile+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("file over 20 MB: %v", err)
	}
	f.exec(`UPDATE exec.project_files SET size = $1 WHERE id = $2`, int64(MaxProjectFiles), file.ID)
	if _, err := f.svc.AddProjectFile(f.ctx, f.user, f.project, "x.txt", "", []byte("x")); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("project over 200 MB: %v", err)
	}

	// Чужой проект не виден; читатель видит, но не меняет.
	stranger := uuid.Must(uuid.NewV7())
	f.exec(`INSERT INTO core.users (id, email, password_hash) VALUES ($1, $2, 'x')`, stranger, stranger.String()+"@e.com")
	if _, err := f.svc.ProjectContext(f.ctx, stranger, f.project); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stranger: %v", err)
	}
	f.exec(`INSERT INTO core.project_access (project_id, user_id, role) VALUES ($1, $2, 'viewer')`, f.project, stranger)
	if _, err := f.svc.ProjectContext(f.ctx, stranger, f.project); err != nil {
		t.Fatalf("viewer reads: %v", err)
	}
	if err := f.svc.SetProjectDescription(f.ctx, stranger, f.project, "взлом"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer must not write: %v", err)
	}
	if err := f.svc.DeleteProjectFile(f.ctx, f.user, f.project, file.ID); err != nil {
		t.Fatal(err)
	}
}

func TestTaskSkillsFromTheNodeAndProjectContextReachTheHost(t *testing.T) {
	f := newFx(t)
	node := uuid.Must(uuid.NewV7())
	f.exec(`INSERT INTO tree.nodes (id, project_id, kind, name, sort_key, revision) VALUES ($1, $2, 'ai_task', 'Магазин', 'V', 1)`, node, f.project)
	f.exec(`INSERT INTO tree.node_content (node_id, raw_content) VALUES ($1, 'Сделай магазин скинов')`, node)
	if _, err := f.svc.SetNodeSkills(f.ctx, f.user, node, []string{"roblox-ui-ux", "roblox-ui-ux", "Bad Name"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad skill name must be rejected: %v", err)
	}
	if got, err := f.svc.SetNodeSkills(f.ctx, f.user, node, []string{"roblox-ui-ux", "roblox-ui-ux", "testing-strategy"}); err != nil ||
		strings.Join(got, ",") != "roblox-ui-ux,testing-strategy" {
		t.Fatalf("%v %v", got, err)
	}
	if err := f.svc.SetProjectDescription(f.ctx, f.user, f.project, "Мультяшный стиль, крупные кнопки."); err != nil {
		t.Fatal(err)
	}
	f.task(func(n *NewTask) { n.Title, n.Prompt, n.NodeID = "", "", &node })
	c := f.claim()
	if c == nil || strings.Join(c.RequiredSkills, ",") != "roblox-ui-ux,testing-strategy" {
		t.Fatalf("skills chosen in the task must go with it: %+v", c)
	}
	if c.Project == nil || c.Project.Description != "Мультяшный стиль, крупные кнопки." {
		t.Fatalf("the host must get the project context: %+v", c.Project)
	}
}

func TestBindRepositoryFromTheProjectWindow(t *testing.T) {
	f := newFx(t)
	var hostID uuid.UUID
	if err := f.pool.QueryRow(f.ctx, `SELECT id FROM exec.hosts WHERE device_id = $1`, f.host.DeviceID).Scan(&hostID); err != nil {
		t.Fatal(err)
	}
	project := uuid.Must(uuid.NewV7())
	f.exec(`INSERT INTO tree.projects (id, name, revision, created_by) VALUES ($1, 'Пасхалки', 1, $2)`, project, f.user)
	f.exec(`INSERT INTO core.project_access (project_id, user_id, role) VALUES ($1, $2, 'owner')`, project, f.user)

	if _, err := f.svc.BindRepository(f.ctx, f.user, BindRequest{ProjectID: project, HostID: hostID}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("folder or git address is required: %v", err)
	}
	if _, err := f.svc.BindRepository(f.ctx, f.user, BindRequest{ProjectID: project, HostID: hostID, GitURL: "file:///etc"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("only https and ssh git addresses: %v", err)
	}
	r, err := f.svc.BindRepository(f.ctx, f.user, BindRequest{ProjectID: project, HostID: hostID,
		GitURL: "https://github.com/Lymoos/easter-hunt.git", VerificationProfile: []byte(`{"template":"roblox"}`)})
	if err != nil || r.Status != "pending" || r.Name != "easter-hunt" {
		t.Fatalf("%+v %v", r, err)
	}
	// Пока служба не привязала код, задачи по нему не выдаются.
	f.exec(`INSERT INTO exec.tasks (id, project_id, repository_id, created_by, title, prompt, status)
		VALUES ($1, $2, $3, $4, 't', 'p', 'QUEUED')`, uuid.Must(uuid.NewV7()), project, r.ID, f.user)
	if c := f.claim(); c != nil {
		t.Fatalf("pending repository must not get tasks: %+v", c)
	}
	pending, err := f.svc.PendingRepositories(f.ctx, f.host)
	if err != nil || len(pending) != 1 || pending[0].GitURL != r.GitURL {
		t.Fatalf("%+v %v", pending, err)
	}
	if err := f.svc.ReportRepository(f.ctx, f.host, r.ID, `D:\aicrew-work\easter-hunt`, ""); err != nil {
		t.Fatal(err)
	}
	if c := f.claim(); c == nil || c.Repository.LocalPath != `D:\aicrew-work\easter-hunt` {
		t.Fatalf("bound repository gets tasks: %+v", c)
	}
	// Повторная привязка того же проекта к тому же ПК меняет заявку, а не создаёт второй репозиторий.
	r2, err := f.svc.BindRepository(f.ctx, f.user, BindRequest{ProjectID: project, HostID: hostID, LocalPath: `D:\games\easter`})
	if err != nil || r2.ID != r.ID || r2.Status != "pending" {
		t.Fatalf("%+v %v", r2, err)
	}
	if err := f.svc.ReportRepository(f.ctx, f.host, r.ID, "", "папка не найдена"); err != nil {
		t.Fatal(err)
	}
	repos, _ := f.svc.ListRepositories(f.ctx, f.user)
	for _, x := range repos {
		if x.ID == r.ID && (x.Status != "failed" || x.StatusError != "папка не найдена") {
			t.Fatalf("failure must be visible in the window: %+v", x)
		}
	}
}
