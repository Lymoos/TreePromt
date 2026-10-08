package httpapi

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"prompttree/backend/internal/core"
	"prompttree/backend/internal/exec"
)

// AiCrew (этап 7.2): пользовательские операции и протокол aicrew-host.

func execIdentity(id core.Identity) exec.Identity {
	return exec.Identity{UserID: id.UserID, DeviceID: id.DeviceID}
}

func (s *Server) execError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, exec.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, exec.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "not allowed")
	case errors.Is(err, exec.ErrNotOwner):
		writeError(w, http.StatusConflict, "not_owner", err.Error())
	case errors.Is(err, exec.ErrBadTransition):
		writeError(w, http.StatusConflict, "bad_transition", err.Error())
	case errors.Is(err, exec.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	default:
		s.internalError(w, r, err)
	}
}

func pathID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func (s *Server) handleExecRegisterHost(w http.ResponseWriter, r *http.Request, id core.Identity) {
	var req struct {
		Name         string            `json:"name"`
		OS           string            `json:"os"`
		Capabilities []string          `json:"capabilities"`
		Environments []string          `json:"environments"`
		Workers      []exec.WorkerSpec `json:"workers"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	h, err := s.exec.RegisterHost(r.Context(), execIdentity(id), req.Name, req.OS, req.Capabilities, req.Environments, req.Workers)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func (s *Server) handleExecCreateRepository(w http.ResponseWriter, r *http.Request, id core.Identity) {
	var req exec.Repository
	if !decodeBody(w, r, &req) {
		return
	}
	repo, err := s.exec.CreateRepository(r.Context(), id.UserID, req)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, repo)
}

func (s *Server) handleExecListRepositories(w http.ResponseWriter, r *http.Request, id core.Identity) {
	repos, err := s.exec.ListRepositories(r.Context(), id.UserID)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": repos})
}

func (s *Server) handleExecCreateTask(w http.ResponseWriter, r *http.Request, id core.Identity) {
	var req exec.NewTask
	if !decodeBody(w, r, &req) {
		return
	}
	taskID, err := s.exec.CreateTask(r.Context(), id.UserID, req)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": taskID})
}

func (s *Server) handleExecListTasks(w http.ResponseWriter, r *http.Request, id core.Identity) {
	var project *uuid.UUID
	if v := r.URL.Query().Get("project_id"); v != "" {
		p, err := uuid.Parse(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "invalid project_id")
			return
		}
		project = &p
	}
	tasks, err := s.exec.ListTasks(r.Context(), id.UserID, project)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (s *Server) handleExecCancel(w http.ResponseWriter, r *http.Request, id core.Identity) {
	taskID, ok := pathID(w, r)
	if !ok {
		return
	}
	status, err := s.exec.Cancel(r.Context(), id.UserID, taskID)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (s *Server) handleExecRetry(w http.ResponseWriter, r *http.Request, id core.Identity) {
	taskID, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.exec.Retry(r.Context(), id.UserID, taskID); err != nil {
		s.execError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── протокол хоста ──

func (s *Server) handleExecClaim(w http.ResponseWriter, r *http.Request, id core.Identity) {
	var req struct {
		WorkerID uuid.UUID `json:"worker_id"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	t, err := s.exec.Claim(r.Context(), execIdentity(id), req.WorkerID)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	if t == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleExecHeartbeat(w http.ResponseWriter, r *http.Request, id core.Identity) {
	taskID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		WorkerID uuid.UUID `json:"worker_id"`
		Attempt  int       `json:"attempt"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	lease, cancel, err := s.exec.Heartbeat(r.Context(), execIdentity(id), taskID, req.WorkerID, req.Attempt)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lease_expires_at": lease, "cancel_requested": cancel})
}

func (s *Server) handleExecTransition(w http.ResponseWriter, r *http.Request, id core.Identity) {
	taskID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		WorkerID uuid.UUID  `json:"worker_id"`
		Attempt  int        `json:"attempt"`
		To       string     `json:"to"`
		Facts    exec.Facts `json:"facts"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	status, err := s.exec.Transition(r.Context(), execIdentity(id), taskID, req.WorkerID, req.Attempt, req.To, req.Facts)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (s *Server) handleExecKeepWorktrees(w http.ResponseWriter, r *http.Request, id core.Identity) {
	ids, err := s.exec.KeepWorktrees(r.Context(), execIdentity(id))
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task_ids": ids})
}
