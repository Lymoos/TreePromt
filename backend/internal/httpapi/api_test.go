package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"prompttree/backend/internal/core"
	"prompttree/backend/internal/realtime"
	"prompttree/backend/internal/testdb"
	"prompttree/backend/internal/tree"
)

func newTestServer(t *testing.T) *httptest.Server {
	pool := testdb.New(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	tokens := core.NewTokenIssuer([]byte("0123456789abcdef0123456789abcdef"))
	auth, err := core.NewAuthService(pool, tokens, false)
	if err != nil {
		t.Fatal(err)
	}
	hub := realtime.NewHub()
	srv := httptest.NewServer(New(auth, tokens, tree.NewService(pool, hub, log), hub, log, Options{}).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func call(t *testing.T, srv *httptest.Server, method, path, token string, body any, out any) int {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, srv.URL+path, rd)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func TestEndToEnd(t *testing.T) {
	srv := newTestServer(t)

	if code := call(t, srv, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": "max@example.com", "password": "длинный-пароль-1", "name": "Максим"}, nil); code != http.StatusCreated {
		t.Fatalf("register: %d", code)
	}
	if code := call(t, srv, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": "intruder@example.com", "password": "длинный-пароль-1"}, nil); code != http.StatusForbidden {
		t.Fatalf("second register must be closed: %d", code)
	}

	login := func() core.Tokens {
		var tok core.Tokens
		code := call(t, srv, "POST", "/api/v1/auth/login", "", map[string]any{
			"email": "max@example.com", "password": "длинный-пароль-1",
			"device": map[string]any{"id": uuid.New(), "name": "test", "platform": "test"}}, &tok)
		if code != http.StatusOK {
			t.Fatalf("login: %d", code)
		}
		return tok
	}
	phone, pc := login(), login()

	if code := call(t, srv, "GET", "/api/v1/sync/pull", "", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("pull without token: %d", code)
	}

	// ПК слушает WebSocket.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/api/v1/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	auth, _ := json.Marshal(map[string]string{"type": "auth", "token": pc.AccessToken})
	if err := ws.Write(ctx, websocket.MessageText, auth); err != nil {
		t.Fatal(err)
	}
	if _, msg, err := ws.Read(ctx); err != nil || !strings.Contains(string(msg), "ready") {
		t.Fatalf("ws ready: %s %v", msg, err)
	}

	// Телефон создаёт проект и заметку.
	projectID, nodeID := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	var push struct {
		Results []tree.Result `json:"results"`
	}
	code := call(t, srv, "POST", "/api/v1/sync/push", phone.AccessToken, map[string]any{"operations": []map[string]any{
		{"operation_id": uuid.Must(uuid.NewV7()), "client_seq": 1, "type": "create_project", "entity_id": projectID,
			"payload": map[string]any{"name": "PromptTree"}},
		{"operation_id": uuid.Must(uuid.NewV7()), "client_seq": 2, "type": "create_node", "entity_id": nodeID,
			"payload": map[string]any{"project_id": projectID, "kind": "ai_task", "name": "Дерево", "sort_key": "a0", "raw_content": "идея"}},
	}}, &push)
	if code != http.StatusOK || len(push.Results) != 2 || push.Results[1].Result != tree.ResultApplied {
		t.Fatalf("push: %d %+v", code, push.Results)
	}

	// ПК получает уведомление и забирает изменения.
	if _, msg, err := ws.Read(ctx); err != nil || !strings.Contains(string(msg), `"revision"`) {
		t.Fatalf("ws revision: %s %v", msg, err)
	}
	var pull tree.PullResult
	if code := call(t, srv, "GET", "/api/v1/sync/pull?cursor=0", pc.AccessToken, nil, &pull); code != http.StatusOK {
		t.Fatalf("pull: %d", code)
	}
	if len(pull.Projects) != 1 || len(pull.Nodes) != 1 || pull.Nodes[0].RawContent != "идея" {
		t.Fatalf("pull: %+v", pull)
	}

	// Refresh выдаёт новую пару токенов.
	var refreshed core.Tokens
	if code := call(t, srv, "POST", "/api/v1/auth/refresh", "", map[string]string{"refresh_token": phone.RefreshToken}, &refreshed); code != http.StatusOK || refreshed.AccessToken == "" {
		t.Fatalf("refresh: %d", code)
	}
}

func TestBadRequests(t *testing.T) {
	srv := newTestServer(t)
	if code := call(t, srv, "POST", "/api/v1/auth/login", "", map[string]any{"email": "x", "unknown": 1}, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown field: %d", code)
	}
	if code := call(t, srv, "GET", "/api/v1/sync/pull", "garbage", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("bad token: %d", code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	srv := newTestServer(t)
	body := map[string]any{"email": "a@example.com", "password": "неверный-пароль",
		"device": map[string]any{"id": uuid.New()}}
	limited := false
	for i := 0; i < 15; i++ {
		if call(t, srv, "POST", "/api/v1/auth/login", "", body, nil) == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Fatal("login attempts are not rate limited")
	}
}
