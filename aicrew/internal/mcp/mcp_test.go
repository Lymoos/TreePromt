package mcp

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func realRegistry(t *testing.T) *Registry {
	_, file, _, _ := runtime.Caller(0)
	r, err := Load(filepath.Join(filepath.Dir(file), "..", "..", "mcp", "registry.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRealRegistryProfiles(t *testing.T) {
	r := realRegistry(t)
	if got := strings.Join(r.ProfileNames(), ","); got != "default,roblox,web" {
		t.Fatal(got)
	}
	d, err := r.Resolve("default", "container")
	if err != nil || d.ConfigJSON != nil || len(d.AllowedTools) != 0 {
		t.Fatalf("default must give nothing extra: %+v %v", d, err)
	}
	w, _ := r.Resolve("web", "container")
	if strings.Join(w.AllowedTools, ",") != "WebFetch" || w.ConfigJSON != nil {
		t.Fatalf("web: %+v", w)
	}
}

// MCP-scopes: профиль Roblox — только в windows_user и только перечисленные инструменты.
func TestRobloxProfileScopes(t *testing.T) {
	r := realRegistry(t)
	if _, err := r.Resolve("roblox", "container"); err == nil {
		t.Fatal("roblox profile must not resolve in a container")
	}
	res, err := r.Resolve("roblox", "windows_user")
	if err != nil {
		t.Fatal(err)
	}
	tools := strings.Join(res.AllowedTools, ",")
	if !strings.Contains(tools, "mcp__roblox_studio__execute_luau") || strings.Contains(tools, "insert_asset") ||
		strings.Contains(tools, "http_get") || strings.Contains(tools, "mcp__roblox_studio,") {
		t.Fatalf("tools: %s", tools)
	}
	if !strings.Contains(string(res.ConfigJSON), `mcp.bat`) {
		t.Fatalf("config: %s", res.ConfigJSON)
	}
}

func TestRegistryValidation(t *testing.T) {
	for name, body := range map[string]string{
		"unknown server": "profiles:\n  p:\n    servers: [ghost]\n",
		"bad builtin":    "profiles:\n  p:\n    builtin_tools: [Bash]\n",
		"bad type":       "servers:\n  s:\n    type: ftp\n",
		"unknown field":  "servers:\n  s:\n    type: stdio\n    command: x\n    password: hunter2\n",
		"stdio w/o cmd":  "servers:\n  s:\n    type: stdio\n",
	} {
		p := filepath.Join(t.TempDir(), "r.yaml")
		_ = os.WriteFile(p, []byte(body), 0o644)
		if _, err := Load(p); err == nil {
			t.Errorf("%s: must be rejected", name)
		}
	}
	r, _ := Load("")
	if _, err := r.Resolve("nope", "container"); err == nil {
		t.Fatal("unknown profile")
	}
}
