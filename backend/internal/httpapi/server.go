// Package httpapi exposes the REST + WebSocket API under /api/v1.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"

	"prompttree/backend/internal/ai"
	"prompttree/backend/internal/core"
	"prompttree/backend/internal/realtime"
	"prompttree/backend/internal/tree"
)

const maxBodyBytes = 8 << 20 // пакет операций с несколькими большими заметками

type Server struct {
	auth           *core.AuthService
	tokens         *core.TokenIssuer
	tree           *tree.Service
	ai             *ai.Service
	hub            *realtime.Hub
	log            *slog.Logger
	allowedOrigins []string
	trustProxy     bool
	limiter        *ipLimiter
}

type Options struct {
	AllowedOrigins []string
	TrustProxy     bool
}

func New(auth *core.AuthService, tokens *core.TokenIssuer, treeSvc *tree.Service, aiSvc *ai.Service, hub *realtime.Hub, log *slog.Logger, opt Options) *Server {
	return &Server{auth: auth, tokens: tokens, tree: treeSvc, ai: aiSvc, hub: hub, log: log,
		allowedOrigins: opt.AllowedOrigins, trustProxy: opt.TrustProxy, limiter: newIPLimiter()}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/v1/auth/register", s.limited(s.handleRegister))
	mux.HandleFunc("POST /api/v1/auth/login", s.limited(s.handleLogin))
	mux.HandleFunc("POST /api/v1/auth/refresh", s.limited(s.handleRefresh))
	mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/v1/me", s.authed(s.handleMe))
	mux.HandleFunc("POST /api/v1/sync/push", s.authed(s.handlePush))
	mux.HandleFunc("GET /api/v1/sync/pull", s.authed(s.handlePull))
	mux.HandleFunc("POST /api/v1/ai/structure", s.authed(s.handleStructure))
	mux.HandleFunc("GET /api/v1/ai/structure/{id}", s.authed(s.handleStructureStatus))
	mux.HandleFunc("GET /api/v1/ws", s.handleWS)
	return s.recoverer(s.cors(mux))
}

// ── ответы ──

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]apiError{"error": {Code: code, Message: msg}})
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "internal server error")
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "unexpected data after JSON body")
		return false
	}
	return true
}

// ── middleware ──

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				s.log.Error("panic", "path", r.URL.Path, "panic", v)
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	return origin != "" && slices.Contains(s.allowedOrigins, origin)
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); s.originAllowed(origin) {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-PT-Client")
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Add("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authed(next func(http.ResponseWriter, *http.Request, core.Identity)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}
		id, err := s.tokens.VerifyAccess(tok)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}
		next(w, r, id)
	}
}

func (s *Server) clientIP(r *http.Request) string {
	if s.trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			return strings.TrimSpace(first)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) limited(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.allow(s.clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again in a minute")
			return
		}
		next(w, r)
	}
}
