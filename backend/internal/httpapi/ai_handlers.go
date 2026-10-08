package httpapi

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"prompttree/backend/internal/ai"
	"prompttree/backend/internal/core"
	"prompttree/backend/internal/tree"
)

// handleStructure ставит задачу в очередь структурирования (этап 6). Ответ сразу —
// 202 с request_id; результат придёт через синхронизацию.
func (s *Server) handleStructure(w http.ResponseWriter, r *http.Request, id core.Identity) {
	var req struct {
		NodeID         uuid.UUID `json:"node_id"`
		SourceRevision int64     `json:"source_revision"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	reqID, err := s.ai.Request(r.Context(), tree.Actor{UserID: id.UserID, DeviceID: id.DeviceID}, req.NodeID, req.SourceRevision)
	switch {
	case errors.Is(err, ai.ErrDisabled):
		writeError(w, http.StatusServiceUnavailable, "ai_disabled", "structuring is not configured on this server")
	case errors.Is(err, ai.ErrDailyLimit):
		writeError(w, http.StatusTooManyRequests, "ai_daily_limit", err.Error())
	case errors.Is(err, tree.ErrStaleSource):
		writeError(w, http.StatusConflict, "stale_source", err.Error())
	case errors.Is(err, tree.ErrNotTask):
		writeError(w, http.StatusBadRequest, "not_a_task", err.Error())
	case errors.Is(err, tree.ErrNoAccess):
		writeError(w, http.StatusNotFound, "not_found", "node not found")
	case err != nil:
		s.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusAccepted, map[string]any{"request_id": reqID})
	}
}

func (s *Server) handleStructureStatus(w http.ResponseWriter, r *http.Request, id core.Identity) {
	reqID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid request id")
		return
	}
	info, err := s.tree.StructureRequest(r.Context(), id.UserID, reqID)
	switch {
	case errors.Is(err, tree.ErrNoAccess):
		writeError(w, http.StatusNotFound, "not_found", "request not found")
	case err != nil:
		s.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, info)
	}
}
