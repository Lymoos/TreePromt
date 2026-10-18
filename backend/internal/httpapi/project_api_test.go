package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"prompttree/backend/internal/core"
)

// Этап 7.9: файлы проекта через HTTP — загрузка сырым телом, скачивание как вложение.
func TestProjectFilesOverHTTP(t *testing.T) {
	srv := newTestServer(t)
	if code := call(t, srv, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": "max@example.com", "password": "длинный-пароль-1"}, nil); code != http.StatusCreated {
		t.Fatalf("register: %d", code)
	}
	var tok core.Tokens
	call(t, srv, "POST", "/api/v1/auth/login", "", map[string]any{"email": "max@example.com", "password": "длинный-пароль-1",
		"device": map[string]any{"id": uuid.New(), "name": "test", "platform": "test"}}, &tok)
	projectID := uuid.Must(uuid.NewV7())
	if code := call(t, srv, "POST", "/api/v1/sync/push", tok.AccessToken, map[string]any{"operations": []map[string]any{
		{"operation_id": uuid.Must(uuid.NewV7()), "client_seq": 1, "type": "create_project", "entity_id": projectID,
			"payload": map[string]any{"name": "Пасхалки"}}}}, nil); code != http.StatusOK {
		t.Fatalf("push: %d", code)
	}

	upload := func(name string, body []byte) *http.Response {
		req, _ := http.NewRequest("POST", srv.URL+"/api/v1/projects/"+projectID.String()+"/files", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		req.Header.Set("Content-Type", "text/markdown; charset=utf-8")
		req.Header.Set("X-File-Name", url.PathEscape(name))
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	resp := upload("ТЗ пасхалок.md", []byte("# ТЗ\nНайти 10 пасхалок."))
	var f struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: %d", resp.StatusCode)
	}
	_ = jsonDecode(resp.Body, &f)
	resp.Body.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/projects/"+projectID.String()+"/files/"+f.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	got, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(got.Body)
	got.Body.Close()
	if string(body) != "# ТЗ\nНайти 10 пасхалок." || got.Header.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.HasPrefix(got.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("download: %q %v", body, got.Header)
	}

	var pc struct {
		Description string `json:"description"`
		Files       []any  `json:"files"`
	}
	if code := call(t, srv, "PUT", "/api/v1/projects/"+projectID.String()+"/context", tok.AccessToken,
		map[string]string{"description": "Игра для детей"}, nil); code != http.StatusNoContent {
		t.Fatalf("set description: %d", code)
	}
	if code := call(t, srv, "GET", "/api/v1/projects/"+projectID.String()+"/context", tok.AccessToken, nil, &pc); code != http.StatusOK ||
		pc.Description != "Игра для детей" || len(pc.Files) != 1 {
		t.Fatalf("context: %d %+v", code, pc)
	}
	if resp := upload("", []byte("x")); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("name is required: %d", resp.StatusCode)
	}
}

func jsonDecode(r io.Reader, v any) error { return json.NewDecoder(r).Decode(v) }
