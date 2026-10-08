package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"

	"prompttree/backend/internal/core"
	"prompttree/backend/internal/tree"
)

// ── auth ──

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	u, err := s.auth.Register(r.Context(), req.Email, req.Password, req.Name)
	switch {
	case errors.Is(err, core.ErrRegistrationClosed):
		writeError(w, http.StatusForbidden, "registration_closed", err.Error())
	case errors.Is(err, core.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", err.Error())
	case errors.Is(err, core.ErrInvalidEmail), errors.Is(err, core.ErrWeakPassword):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case err != nil:
		s.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusCreated, u)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string      `json:"email"`
		Password string      `json:"password"`
		Device   core.Device `json:"device"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	t, err := s.auth.Login(r.Context(), req.Email, req.Password, req.Device)
	switch {
	case errors.Is(err, core.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", err.Error())
	case errors.Is(err, core.ErrInvalidDevice):
		writeError(w, http.StatusBadRequest, "invalid_device", err.Error())
	case errors.Is(err, core.ErrDeviceOwnedByOther):
		writeError(w, http.StatusConflict, "device_conflict", err.Error())
	case err != nil:
		s.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, s.deliverRefresh(w, r, t))
	}
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	t, err := s.auth.Refresh(r.Context(), s.refreshFromRequest(r, req.RefreshToken))
	switch {
	case errors.Is(err, core.ErrStaleRefresh):
		writeError(w, http.StatusConflict, "stale_refresh", err.Error())
	case errors.Is(err, core.ErrInvalidRefreshToken):
		s.clearRefreshCookie(w, r)
		writeError(w, http.StatusUnauthorized, "invalid_refresh_token", err.Error())
	case err != nil:
		s.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, s.deliverRefresh(w, r, t))
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.auth.Logout(r.Context(), s.refreshFromRequest(r, req.RefreshToken)); err != nil {
		s.internalError(w, r, err)
		return
	}
	s.clearRefreshCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, id core.Identity) {
	u, err := s.auth.Me(r.Context(), id.UserID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "device_id": id.DeviceID})
}

// ── sync ──

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request, id core.Identity) {
	var req struct {
		Operations []tree.Op `json:"operations"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if len(req.Operations) > tree.MaxOpsPerPush {
		writeError(w, http.StatusBadRequest, "too_many_operations", "send at most 500 operations per request")
		return
	}
	results, err := s.tree.Push(r.Context(), tree.Actor{UserID: id.UserID, DeviceID: id.DeviceID}, req.Operations)
	if err != nil {
		// Часть операций могла примениться: отдаём их результаты, клиент повторит остальные.
		s.log.Error("push failed", "user", id.UserID, "applied", len(results), "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"results": results,
			"error":   apiError{Code: "partial_failure", Message: "some operations were not processed; retry them"},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request, id core.Identity) {
	q := r.URL.Query()
	cursor, err := strconv.ParseInt(q.Get("cursor"), 10, 64)
	if q.Get("cursor") == "" {
		cursor, err = 0, nil
	}
	if err != nil || cursor < 0 {
		writeError(w, http.StatusBadRequest, "invalid_cursor", "cursor must be a non-negative integer")
		return
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	res, err := s.tree.Pull(r.Context(), id.UserID, cursor, limit)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ── websocket ──

// handleWS: браузер не умеет ставить заголовок Authorization для WebSocket,
// поэтому токен приходит первым сообщением {"type":"auth","token":"..."} в течение 5 секунд.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	opts := &websocket.AcceptOptions{OriginPatterns: originHosts(s.allowedOrigins)}
	conn, err := websocket.Accept(w, r, opts)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(4096)

	authCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	var msg struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}
	err = readJSON(authCtx, conn, &msg)
	cancel()
	if err != nil || msg.Type != "auth" {
		conn.Close(websocket.StatusPolicyViolation, "auth required")
		return
	}
	id, err := s.tokens.VerifyAccess(msg.Token)
	if err != nil {
		conn.Close(websocket.StatusPolicyViolation, "invalid token")
		return
	}

	client := s.hub.Register(id.UserID)
	defer s.hub.Unregister(client)

	ctx := conn.CloseRead(r.Context()) // входящие сообщения после auth не нужны
	_ = writeText(ctx, conn, []byte(`{"type":"ready"}`))
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-client.Send:
			if err := writeText(ctx, conn, m); err != nil {
				return
			}
		case <-ping.C:
			pctx, pcancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pctx)
			pcancel()
			if err != nil {
				return
			}
		}
	}
}
