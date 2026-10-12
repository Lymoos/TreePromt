package httpapi

import (
	"encoding/json"
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
		// 7.5: реестры хоста — по ним проверяются планы Manager'а.
		Skills      []string `json:"skills"`
		MCPProfiles []string `json:"mcp_profiles"`
		Executors   []string `json:"executors"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	h, err := s.exec.RegisterHost(r.Context(), execIdentity(id), req.Name, req.OS, req.Capabilities, req.Environments, req.Workers)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	// Реестры присылает только служба (aicrew-host run); add-repo их не трогает.
	if req.Skills != nil || req.MCPProfiles != nil || req.Executors != nil {
		if err := s.exec.SetHostRegistries(r.Context(), execIdentity(id), h.ID, req.Skills, req.MCPProfiles, req.Executors); err != nil {
			s.execError(w, r, err)
			return
		}
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

// handleExecMerge — «Слить»: проверенная задача встаёт в очередь слияния в integration (этап 7.3).
func (s *Server) handleExecMerge(w http.ResponseWriter, r *http.Request, id core.Identity) {
	taskID, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.exec.Merge(r.Context(), id.UserID, taskID); err != nil {
		s.execError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleExecRollback — «Откатить»: revert слияния задачи через очередь, с проверкой.
func (s *Server) handleExecRollback(w http.ResponseWriter, r *http.Request, id core.Identity) {
	taskID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if r.ContentLength > 0 && !decodeBody(w, r, &req) {
		return
	}
	if err := s.exec.Rollback(r.Context(), id.UserID, taskID, req.Reason); err != nil {
		s.execError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleExecUpdateProfile — профиль проверки репозитория (шаблон flutter/go/web и шаги).
func (s *Server) handleExecUpdateProfile(w http.ResponseWriter, r *http.Request, id core.Identity) {
	repoID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Profile json.RawMessage `json:"verification_profile"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.exec.UpdateVerificationProfile(r.Context(), id.UserID, repoID, req.Profile); err != nil {
		s.execError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── протокол хоста ──

// handleExecReview — Logic QA моделью сервера. 503 not_configured — ключа нет, хост ревьюит сам.
func (s *Server) handleExecReview(w http.ResponseWriter, r *http.Request, id core.Identity) {
	taskID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		WorkerID uuid.UUID `json:"worker_id"`
		Attempt  int       `json:"attempt"`
		System   string    `json:"system"`
		User     string    `json:"user"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	v, err := s.exec.Review(r.Context(), execIdentity(id), taskID, req.WorkerID, req.Attempt, req.System, req.User)
	if errors.Is(err, exec.ErrReviewerNotConfigured) {
		writeError(w, http.StatusServiceUnavailable, "not_configured", err.Error())
		return
	}
	switch {
	case err == nil:
	case errors.Is(err, exec.ErrNotOwner), errors.Is(err, exec.ErrForbidden), errors.Is(err, exec.ErrInvalid), errors.Is(err, exec.ErrNotFound):
		s.execError(w, r, err)
		return
	default:
		// Сбой модели: хост классифицирует его как ENVIRONMENT_FAILURE.
		s.log.Warn("logic qa model failed", "task", taskID, "err", err)
		writeError(w, http.StatusBadGateway, "model_failed", "review model failed")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

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

// ── планы Manager'а (этап 7.5) ──

func (s *Server) handleExecCreatePlan(w http.ResponseWriter, r *http.Request, id core.Identity) {
	var req exec.NewPlan
	if !decodeBody(w, r, &req) {
		return
	}
	planID, err := s.exec.CreatePlan(r.Context(), id.UserID, req)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": planID})
}

func (s *Server) handleExecListPlans(w http.ResponseWriter, r *http.Request, id core.Identity) {
	plans, err := s.exec.ListPlans(r.Context(), id.UserID)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans})
}

func (s *Server) handleExecApprovePlan(w http.ResponseWriter, r *http.Request, id core.Identity) {
	planID, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.exec.ApprovePlan(r.Context(), id.UserID, planID); err != nil {
		s.execError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleExecRejectPlan(w http.ResponseWriter, r *http.Request, id core.Identity) {
	planID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if r.ContentLength > 0 && !decodeBody(w, r, &req) {
		return
	}
	if err := s.exec.RejectPlan(r.Context(), id.UserID, planID, req.Reason); err != nil {
		s.execError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleExecPlanResult — хост сдаёт предложение Manager'а; сервер проверяет и применяет.
func (s *Server) handleExecPlanResult(w http.ResponseWriter, r *http.Request, id core.Identity) {
	planID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		WorkerID uuid.UUID      `json:"worker_id"`
		Attempt  int            `json:"attempt"`
		Proposal *exec.Proposal `json:"proposal"`
		Failure  string         `json:"failure"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	status, reason, err := s.exec.SubmitPlan(r.Context(), execIdentity(id), planID, req.WorkerID, req.Attempt, req.Proposal, req.Failure)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status, "error": reason})
}

func (s *Server) handleExecSkillPolicy(w http.ResponseWriter, r *http.Request, id core.Identity) {
	repoID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Required  []string `json:"required"`
		Forbidden []string `json:"forbidden"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.exec.SetSkillPolicy(r.Context(), id.UserID, repoID, req.Required, req.Forbidden); err != nil {
		s.execError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── 7.6: инциденты, аудит, запросы на решение ──

func (s *Server) handleExecListIncidents(w http.ResponseWriter, r *http.Request, id core.Identity) {
	list, err := s.exec.ListIncidents(r.Context(), id.UserID, r.URL.Query().Get("open") != "")
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"incidents": list})
}

// handleExecStop — ручной рубильник: «Остановить всё» (без repository_id) или репозиторий.
func (s *Server) handleExecStop(w http.ResponseWriter, r *http.Request, id core.Identity) {
	var req struct {
		RepositoryID *uuid.UUID `json:"repository_id"`
		Reason       string     `json:"reason"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	incident, err := s.exec.Stop(r.Context(), id.UserID, req.RepositoryID, req.Reason)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": incident})
}

func (s *Server) handleExecResolveIncident(w http.ResponseWriter, r *http.Request, id core.Identity) {
	incidentID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Comment string `json:"comment"`
	}
	if r.ContentLength > 0 && !decodeBody(w, r, &req) {
		return
	}
	if err := s.exec.ResolveIncident(r.Context(), id.UserID, incidentID, req.Comment); err != nil {
		s.execError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleExecIntegrationRed(w http.ResponseWriter, r *http.Request, id core.Identity) {
	repoID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.exec.ReportIntegrationRed(r.Context(), id.UserID, repoID, req.Reason); err != nil {
		s.execError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleExecWriteAudit(w http.ResponseWriter, r *http.Request, id core.Identity) {
	var req struct {
		Entries []exec.AuditEntry     `json:"entries"`
		Prompts []exec.PromptSnapshot `json:"prompts"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.exec.WriteAudit(r.Context(), execIdentity(id), req.Entries, req.Prompts); err != nil {
		s.execError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleExecTaskAudit(w http.ResponseWriter, r *http.Request, id core.Identity) {
	taskID, ok := pathID(w, r)
	if !ok {
		return
	}
	records, err := s.exec.TaskAudit(r.Context(), id.UserID, taskID)
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit": records})
}

func (s *Server) handleExecPrompt(w http.ResponseWriter, r *http.Request, id core.Identity) {
	text, err := s.exec.PromptSnapshot(r.Context(), id.UserID, r.PathValue("hash"))
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"hash": r.PathValue("hash"), "text": text})
}

func (s *Server) handleExecListApprovals(w http.ResponseWriter, r *http.Request, id core.Identity) {
	list, err := s.exec.ListApprovals(r.Context(), id.UserID, r.URL.Query().Get("status"))
	if err != nil {
		s.execError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"approvals": list})
}

func (s *Server) handleExecResolveApproval(w http.ResponseWriter, r *http.Request, id core.Identity) {
	approvalID, ok := pathID(w, r)
	if !ok {
		return
	}
	var d exec.Decision
	if !decodeBody(w, r, &d) {
		return
	}
	if err := s.exec.ResolveApproval(r.Context(), id.UserID, approvalID, d); err != nil {
		s.execError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
