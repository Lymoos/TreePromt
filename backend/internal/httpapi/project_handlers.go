package httpapi

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"

	"prompttree/backend/internal/core"
	"prompttree/backend/internal/exec"
)

// Этап 7.9 (docs/stage7-9-projects.md): окно проекта, файлы, скиллы задачи, привязка кода.

func (s *Server) projectError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, exec.ErrTooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "file_too_large",
			"file is larger than 20 MB or the project files exceed 200 MB")
		return
	}
	s.execError(w, r, err)
}

func (s *Server) handleProjectContext(w http.ResponseWriter, r *http.Request, id core.Identity) {
	projectID, ok := pathID(w, r)
	if !ok {
		return
	}
	pc, err := s.exec.ProjectContext(r.Context(), id.UserID, projectID)
	if err != nil {
		s.projectError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pc)
}

func (s *Server) handleSetProjectContext(w http.ResponseWriter, r *http.Request, id core.Identity) {
	projectID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Description string `json:"description"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.exec.SetProjectDescription(r.Context(), id.UserID, projectID, req.Description); err != nil {
		s.projectError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleAddProjectFile — тело запроса — сам файл; имя — в заголовке X-File-Name (URL-кодированное).
func (s *Server) handleAddProjectFile(w http.ResponseWriter, r *http.Request, id core.Identity) {
	projectID, ok := pathID(w, r)
	if !ok {
		return
	}
	name, err := url.PathUnescape(r.Header.Get("X-File-Name"))
	if err != nil || name == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "X-File-Name header is required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, exec.MaxProjectFile+1)
	data, err := io.ReadAll(r.Body)
	if err != nil || len(data) > exec.MaxProjectFile {
		s.projectError(w, r, exec.ErrTooLarge)
		return
	}
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	f, err := s.exec.AddProjectFile(r.Context(), id.UserID, projectID, name, ct, data)
	if err != nil {
		s.projectError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) handleProjectFile(w http.ResponseWriter, r *http.Request, id core.Identity) {
	projectID, ok := pathID(w, r)
	if !ok {
		return
	}
	fileID, err := uuid.Parse(r.PathValue("file"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid file id")
		return
	}
	f, data, err := s.exec.ProjectFileData(r.Context(), id.UserID, projectID, fileID)
	if err != nil {
		s.projectError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", f.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

func (s *Server) handleDeleteProjectFile(w http.ResponseWriter, r *http.Request, id core.Identity) {
	projectID, ok := pathID(w, r)
	if !ok {
		return
	}
	fileID, err := uuid.Parse(r.PathValue("file"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid file id")
		return
	}
	if err := s.exec.DeleteProjectFile(r.Context(), id.UserID, projectID, fileID); err != nil {
		s.projectError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleNodeSkills(w http.ResponseWriter, r *http.Request, id core.Identity) {
	nodeID, ok := pathID(w, r)
	if !ok {
		return
	}
	skills, err := s.exec.NodeSkills(r.Context(), id.UserID, nodeID)
	if err != nil {
		s.projectError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": skills})
}

func (s *Server) handleSetNodeSkills(w http.ResponseWriter, r *http.Request, id core.Identity) {
	nodeID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Skills []string `json:"skills"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	skills, err := s.exec.SetNodeSkills(r.Context(), id.UserID, nodeID, req.Skills)
	if err != nil {
		s.projectError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": skills})
}

func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request, id core.Identity) {
	skills, err := s.exec.ListSkills(r.Context(), id.UserID)
	if err != nil {
		s.projectError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": skills})
}

func (s *Server) handleBindRepository(w http.ResponseWriter, r *http.Request, id core.Identity) {
	var req exec.BindRequest
	if !decodeBody(w, r, &req) {
		return
	}
	repo, err := s.exec.BindRepository(r.Context(), id.UserID, req)
	if err != nil {
		s.projectError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

func (s *Server) handlePendingRepositories(w http.ResponseWriter, r *http.Request, id core.Identity) {
	repos, err := s.exec.PendingRepositories(r.Context(), execIdentity(id))
	if err != nil {
		s.projectError(w, r, err)
		return
	}
	if repos == nil {
		repos = []exec.Repository{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": repos})
}

func (s *Server) handleReportRepository(w http.ResponseWriter, r *http.Request, id core.Identity) {
	repoID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		LocalPath string `json:"local_path"`
		Error     string `json:"error"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.exec.ReportRepository(r.Context(), execIdentity(id), repoID, req.LocalPath, req.Error); err != nil {
		s.projectError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
