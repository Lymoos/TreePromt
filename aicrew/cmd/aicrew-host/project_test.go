package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"prompttree/aicrew/internal/api"
)

type memTokens map[string]string

func (m memTokens) Get(k string) (string, error) { return m[k], nil }
func (m memTokens) Set(k, v string) error        { m[k] = v; return nil }

func TestResolveProjectByNameAcrossPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("cursor") {
		case "0":
			w.Write([]byte(`{"projects":[{"id":"p1","name":"Полигон AiCrew"},{"id":"p2","name":"Старый"}],"cursor":5,"has_more":true}`))
		default: // второй страницей «Старый» удалён
			w.Write([]byte(`{"projects":[{"id":"p2","name":"Старый","deleted_at":"2026-10-12T10:00:00Z"}],"cursor":6,"has_more":false}`))
		}
	}))
	defer srv.Close()
	c := api.New(srv.URL, memTokens{})
	ctx := context.Background()

	if id, err := resolveProject(ctx, c, " полигон aicrew "); err != nil || id != "p1" {
		t.Fatalf("by name: %q %v", id, err)
	}
	if _, err := resolveProject(ctx, c, "Старый"); err == nil || !strings.Contains(err.Error(), "«Полигон AiCrew»") {
		t.Fatalf("deleted project must not match, and the error lists what exists: %v", err)
	}
	id := "6420c9e8-aa82-4b07-876c-fe5a610832d9"
	if got, err := resolveProject(ctx, c, id); err != nil || got != id {
		t.Fatalf("id passes through: %q %v", got, err)
	}
}
