package exec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Этап 7.9 (docs/stage7-9-projects.md): окно проекта — описание для ИИ, файлы, привязка кода — и скиллы задачи.

const (
	MaxProjectDescription = 20000    // символов
	MaxProjectFile        = 20 << 20 // байт на файл
	MaxProjectFiles       = 200 << 20
	MaxTaskSkills         = 20
)

// ErrTooLarge — файл или сумма файлов проекта больше лимита.
var ErrTooLarge = errors.New("file is too large")

type ProjectFile struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	SHA256      string    `json:"sha256"`
	CreatedAt   time.Time `json:"created_at"`
}

// ProjectContext — то, что агенты знают о проекте: описание и список файлов.
type ProjectContext struct {
	ProjectID   uuid.UUID     `json:"project_id"`
	Description string        `json:"description"`
	Files       []ProjectFile `json:"files"`
	UpdatedAt   *time.Time    `json:"updated_at"`
}

func requireProjectRead(ctx context.Context, q pgx.Tx, userID, projectID uuid.UUID) error {
	var role string
	err := q.QueryRow(ctx, `SELECT role FROM core.project_access WHERE project_id = $1 AND user_id = $2`, projectID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Service) ProjectContext(ctx context.Context, userID, projectID uuid.UUID) (ProjectContext, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProjectContext{}, err
	}
	defer tx.Rollback(ctx)
	if err := requireProjectRead(ctx, tx, userID, projectID); err != nil {
		return ProjectContext{}, err
	}
	return loadProjectContext(ctx, tx, projectID)
}

func loadProjectContext(ctx context.Context, q pgx.Tx, projectID uuid.UUID) (ProjectContext, error) {
	pc := ProjectContext{ProjectID: projectID, Files: []ProjectFile{}}
	err := q.QueryRow(ctx, `SELECT description, updated_at FROM exec.project_context WHERE project_id = $1`, projectID).
		Scan(&pc.Description, &pc.UpdatedAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return pc, err
	}
	rows, err := q.Query(ctx, `SELECT id, name, content_type, size, sha256, created_at FROM exec.project_files
		WHERE project_id = $1 ORDER BY created_at`, projectID)
	if err != nil {
		return pc, err
	}
	pc.Files, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (ProjectFile, error) {
		var f ProjectFile
		err := r.Scan(&f.ID, &f.Name, &f.ContentType, &f.Size, &f.SHA256, &f.CreatedAt)
		return f, err
	})
	if pc.Files == nil {
		pc.Files = []ProjectFile{}
	}
	return pc, err
}

func (s *Service) SetProjectDescription(ctx context.Context, userID, projectID uuid.UUID, description string) error {
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > MaxProjectDescription {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := requireProjectWrite(ctx, tx, userID, projectID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO exec.project_context (project_id, description, updated_by, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (project_id) DO UPDATE SET description = $2, updated_by = $3, updated_at = now()`,
		projectID, description, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

var unsafeName = regexp.MustCompile(`[\x00-\x1f<>:"/\\|?*]`)

// cleanFileName — только имя, без путей и служебных символов (файл попадает агенту в папку /project).
func cleanFileName(name string) string {
	name = path.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	name = strings.TrimSpace(unsafeName.ReplaceAllString(name, "_"))
	if name == "." || name == ".." || name == "/" {
		return ""
	}
	if utf8.RuneCountInString(name) > 120 {
		name = string([]rune(name)[:120])
	}
	return name
}

func (s *Service) AddProjectFile(ctx context.Context, userID, projectID uuid.UUID, name, contentType string, data []byte) (ProjectFile, error) {
	name = cleanFileName(name)
	if name == "" || len(data) == 0 {
		return ProjectFile{}, ErrInvalid
	}
	if len(data) > MaxProjectFile {
		return ProjectFile{}, ErrTooLarge
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProjectFile{}, err
	}
	defer tx.Rollback(ctx)
	if err := requireProjectWrite(ctx, tx, userID, projectID); err != nil {
		return ProjectFile{}, err
	}
	var total int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(size), 0) FROM exec.project_files WHERE project_id = $1`, projectID).Scan(&total); err != nil {
		return ProjectFile{}, err
	}
	if total+int64(len(data)) > MaxProjectFiles {
		return ProjectFile{}, ErrTooLarge
	}
	sum := sha256.Sum256(data)
	f := ProjectFile{ID: uuid.Must(uuid.NewV7()), Name: name, ContentType: contentType, Size: int64(len(data)),
		SHA256: hex.EncodeToString(sum[:]), CreatedAt: s.now()}
	if _, err := tx.Exec(ctx, `INSERT INTO exec.project_files (id, project_id, name, content_type, size, sha256, data, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, f.ID, projectID, f.Name, f.ContentType, f.Size, f.SHA256, data, userID, f.CreatedAt); err != nil {
		return ProjectFile{}, err
	}
	return f, tx.Commit(ctx)
}

// ProjectFileData — файл целиком: для приложений и для службы, которая кладёт его агенту.
func (s *Service) ProjectFileData(ctx context.Context, userID, projectID, fileID uuid.UUID) (ProjectFile, []byte, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProjectFile{}, nil, err
	}
	defer tx.Rollback(ctx)
	if err := requireProjectRead(ctx, tx, userID, projectID); err != nil {
		return ProjectFile{}, nil, err
	}
	var f ProjectFile
	var data []byte
	err = tx.QueryRow(ctx, `SELECT id, name, content_type, size, sha256, created_at, data FROM exec.project_files
		WHERE id = $1 AND project_id = $2`, fileID, projectID).Scan(&f.ID, &f.Name, &f.ContentType, &f.Size, &f.SHA256, &f.CreatedAt, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		return f, nil, ErrNotFound
	}
	return f, data, err
}

func (s *Service) DeleteProjectFile(ctx context.Context, userID, projectID, fileID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := requireProjectWrite(ctx, tx, userID, projectID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM exec.project_files WHERE id = $1 AND project_id = $2`, fileID, projectID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

// ── скиллы задачи ──

var skillName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func (s *Service) NodeSkills(ctx context.Context, userID, nodeID uuid.UUID) ([]string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var projectID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT project_id FROM tree.nodes WHERE id = $1`, nodeID).Scan(&projectID); err != nil {
		return nil, ErrNotFound
	}
	if err := requireProjectRead(ctx, tx, userID, projectID); err != nil {
		return nil, err
	}
	return nodeSkills(ctx, tx, nodeID)
}

func nodeSkills(ctx context.Context, q pgx.Tx, nodeID uuid.UUID) ([]string, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT skills FROM exec.node_skills WHERE node_id = $1`, nodeID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []string{}
	return out, json.Unmarshal(raw, &out)
}

func (s *Service) SetNodeSkills(ctx context.Context, userID, nodeID uuid.UUID, skills []string) ([]string, error) {
	seen := map[string]bool{}
	clean := []string{}
	for _, sk := range skills {
		sk = strings.TrimSpace(sk)
		if !skillName.MatchString(sk) {
			return nil, ErrInvalid
		}
		if !seen[sk] {
			seen[sk] = true
			clean = append(clean, sk)
		}
	}
	if len(clean) > MaxTaskSkills {
		return nil, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var projectID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT project_id FROM tree.nodes WHERE id = $1`, nodeID).Scan(&projectID); err != nil {
		return nil, ErrNotFound
	}
	if err := requireProjectWrite(ctx, tx, userID, projectID); err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(clean)
	if _, err := tx.Exec(ctx, `INSERT INTO exec.node_skills (node_id, skills, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (node_id) DO UPDATE SET skills = $2, updated_at = now()`, nodeID, raw); err != nil {
		return nil, err
	}
	return clean, tx.Commit(ctx)
}

// SkillInfo — скилл из каталогов хостов пользователя: для окна выбора.
type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Service) ListSkills(ctx context.Context, userID uuid.UUID) ([]SkillInfo, error) {
	rows, err := s.pool.Query(ctx, `SELECT skills, skill_info FROM exec.hosts WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	desc := map[string]string{}
	for rows.Next() {
		var names, info []byte
		if err := rows.Scan(&names, &info); err != nil {
			return nil, err
		}
		var ns []string
		var in map[string]string
		_ = json.Unmarshal(names, &ns)
		_ = json.Unmarshal(info, &in)
		for _, n := range ns {
			if desc[n] == "" {
				desc[n] = in[n]
			}
		}
	}
	out := []SkillInfo{}
	for n, d := range desc {
		out = append(out, SkillInfo{Name: n, Description: d})
	}
	sortSkills(out)
	return out, rows.Err()
}

func sortSkills(xs []SkillInfo) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j].Name < xs[j-1].Name; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}

// SetSkillInfo — описания скиллов хоста (присылает служба вместе с реестрами).
func (s *Service) SetSkillInfo(ctx context.Context, id Identity, hostID uuid.UUID, info map[string]string) error {
	raw, _ := json.Marshal(info)
	_, err := s.pool.Exec(ctx, `UPDATE exec.hosts SET skill_info = $3 WHERE id = $1 AND user_id = $2 AND device_id = $4`,
		hostID, id.UserID, raw, id.DeviceID)
	return err
}

// ── привязка кода из окна проекта ──

// BindRequest — папка на ПК и/или git-адрес (решение 19.10.2026: оба поля).
type BindRequest struct {
	ProjectID           uuid.UUID       `json:"project_id"`
	HostID              uuid.UUID       `json:"host_id"`
	LocalPath           string          `json:"local_path"`
	GitURL              string          `json:"git_url"`
	DefaultBranch       string          `json:"default_branch"`
	VerificationProfile json.RawMessage `json:"verification_profile"`
}

var gitURL = regexp.MustCompile(`^(https://[^\s]+|git@[^\s:]+:[^\s]+)$`)

// BindRepository создаёт или меняет привязку кода проекта к ПК; служба этого ПК выполнит заявку (status pending).
func (s *Service) BindRepository(ctx context.Context, userID uuid.UUID, in BindRequest) (Repository, error) {
	in.LocalPath, in.GitURL = strings.TrimSpace(in.LocalPath), strings.TrimSpace(in.GitURL)
	if in.LocalPath == "" && in.GitURL == "" {
		return Repository{}, ErrInvalid
	}
	if in.GitURL != "" && !gitURL.MatchString(in.GitURL) {
		return Repository{}, ErrInvalid
	}
	if in.DefaultBranch == "" {
		in.DefaultBranch = "main"
	}
	if in.DefaultBranch == "aicrew/integration" {
		return Repository{}, ErrInvalid
	}
	if len(in.VerificationProfile) == 0 {
		in.VerificationProfile = json.RawMessage(`{}`)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Repository{}, err
	}
	defer tx.Rollback(ctx)
	if err := requireProjectWrite(ctx, tx, userID, in.ProjectID); err != nil {
		return Repository{}, err
	}
	var owner uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT user_id FROM exec.hosts WHERE id = $1`, in.HostID).Scan(&owner); err != nil || owner != userID {
		return Repository{}, ErrNotFound
	}
	name := repoName(in.LocalPath, in.GitURL)
	r := Repository{ProjectID: in.ProjectID, HostID: in.HostID, Name: name, LocalPath: in.LocalPath, GitURL: in.GitURL,
		DefaultBranch: in.DefaultBranch, IntegrationBranch: "aicrew/integration", VerificationProfile: in.VerificationProfile,
		Status: "pending"}
	err = tx.QueryRow(ctx, `SELECT id FROM exec.repositories WHERE project_id = $1 AND host_id = $2 ORDER BY created_at LIMIT 1`,
		in.ProjectID, in.HostID).Scan(&r.ID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		r.ID = uuid.Must(uuid.NewV7())
		_, err = tx.Exec(ctx, `INSERT INTO exec.repositories (id, project_id, host_id, name, local_path, default_branch,
				integration_branch, verification_profile, git_url, status, status_error)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending', '')`, r.ID, r.ProjectID, r.HostID, r.Name, r.LocalPath,
			r.DefaultBranch, r.IntegrationBranch, r.VerificationProfile, r.GitURL)
	case err == nil:
		_, err = tx.Exec(ctx, `UPDATE exec.repositories SET name = $2, local_path = $3, git_url = $4, default_branch = $5,
				verification_profile = $6, status = 'pending', status_error = '' WHERE id = $1`,
			r.ID, r.Name, r.LocalPath, r.GitURL, r.DefaultBranch, r.VerificationProfile)
	}
	if err != nil {
		return Repository{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Repository{}, err
	}
	s.notify(userID) // служба ПК узнаёт о заявке сразу, а не на следующем опросе
	return r, nil
}

func repoName(localPath, url string) string {
	src := strings.TrimRight(strings.ReplaceAll(localPath, "\\", "/"), "/")
	if src == "" {
		src = strings.TrimSuffix(strings.TrimRight(url, "/"), ".git")
		if i := strings.LastIndexAny(src, "/:"); i >= 0 {
			src = src[i+1:]
		}
	}
	name := path.Base(src)
	if name == "." || name == "/" || name == "" {
		name = "project"
	}
	return name
}

// PendingRepositories — заявки на привязку для ПК этого устройства.
func (s *Service) PendingRepositories(ctx context.Context, id Identity) ([]Repository, error) {
	rows, err := s.pool.Query(ctx, `SELECT r.id, r.project_id, r.host_id, r.name, r.local_path, r.default_branch, r.integration_branch,
			r.verification_profile, r.git_url, r.status, r.status_error
		FROM exec.repositories r JOIN exec.hosts h ON h.id = r.host_id
		WHERE h.user_id = $1 AND h.device_id = $2 AND r.status = 'pending' ORDER BY r.created_at`, id.UserID, id.DeviceID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanRepository)
}

// ReportRepository — итог заявки: служба привязала папку (возможно, после клона) или объясняет отказ.
func (s *Service) ReportRepository(ctx context.Context, id Identity, repoID uuid.UUID, localPath, failure string) error {
	status, path := "ready", strings.TrimSpace(localPath)
	if failure != "" {
		status = "failed"
	} else if path == "" {
		return ErrInvalid
	}
	if utf8.RuneCountInString(failure) > 1000 {
		failure = string([]rune(failure)[:1000])
	}
	tag, err := s.pool.Exec(ctx, `UPDATE exec.repositories r SET status = $4, status_error = $5,
			local_path = CASE WHEN $6 = '' THEN r.local_path ELSE $6 END
		FROM exec.hosts h WHERE r.id = $1 AND h.id = r.host_id AND h.user_id = $2 AND h.device_id = $3 AND r.status = 'pending'`,
		repoID, id.UserID, id.DeviceID, status, failure, path)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	s.notify(id.UserID) // окно проекта показывает итог привязки
	return nil
}

func scanRepository(r pgx.CollectableRow) (Repository, error) {
	var v Repository
	err := r.Scan(&v.ID, &v.ProjectID, &v.HostID, &v.Name, &v.LocalPath, &v.DefaultBranch, &v.IntegrationBranch,
		&v.VerificationProfile, &v.GitURL, &v.Status, &v.StatusError)
	return v, err
}
