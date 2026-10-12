// Package mcp — реестр MCP-серверов и профилей задач (ТЗ п. 11, docs/stage7-5-manager-skills-mcp.md, раздел 3).
// Агент получает конфиг только серверов своего профиля, а не произвольный доступ.
package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

type Server struct {
	Description string            `yaml:"description"`
	Type        string            `yaml:"type"` // stdio | http
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args"`
	URL         string            `yaml:"url"`
	Env         map[string]string `yaml:"env"` // только несекретные значения; секреты — этап 7.6
	Tools       []string          `yaml:"tools"`
	Scopes      Scopes            `yaml:"scopes"`
}

type Scopes struct {
	Network bool     `yaml:"network"`
	Paths   []string `yaml:"paths"`
}

type Profile struct {
	Description  string   `yaml:"description"`
	Servers      []string `yaml:"servers"`
	BuiltinTools []string `yaml:"builtin_tools"` // инструменты Claude Code, например WebFetch
	Environments []string `yaml:"environments"`  // где профиль допустим; пусто — везде
}

type Registry struct {
	Servers  map[string]Server  `yaml:"servers"`
	Profiles map[string]Profile `yaml:"profiles"`
}

// allowedBuiltin — встроенные инструменты, которые профиль может добавить сверх базовых.
var allowedBuiltin = map[string]bool{"WebFetch": true, "WebSearch": true}

func Load(path string) (*Registry, error) {
	r := &Registry{Servers: map[string]Server{}, Profiles: map[string]Profile{}}
	if path == "" {
		r.Profiles["default"] = Profile{Description: "no MCP servers"}
		return r, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(r); err != nil {
		return nil, fmt.Errorf("mcp registry: %w", err)
	}
	if r.Servers == nil {
		r.Servers = map[string]Server{}
	}
	if _, ok := r.Profiles["default"]; !ok {
		r.Profiles["default"] = Profile{Description: "no MCP servers"}
	}
	for name, s := range r.Servers {
		switch s.Type {
		case "stdio":
			if s.Command == "" {
				return nil, fmt.Errorf("mcp server %s: command is required", name)
			}
		case "http":
			if s.URL == "" {
				return nil, fmt.Errorf("mcp server %s: url is required", name)
			}
		default:
			return nil, fmt.Errorf("mcp server %s: type must be stdio or http", name)
		}
	}
	for name, p := range r.Profiles {
		for _, s := range p.Servers {
			if _, ok := r.Servers[s]; !ok {
				return nil, fmt.Errorf("mcp profile %s: unknown server %q", name, s)
			}
		}
		for _, t := range p.BuiltinTools {
			if !allowedBuiltin[t] {
				return nil, fmt.Errorf("mcp profile %s: builtin tool %q is not allowed", name, t)
			}
		}
	}
	return r, nil
}

func (r *Registry) ProfileNames() []string {
	var out []string
	for n := range r.Profiles {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Resolved — что получает исполнитель: конфиг серверов и разрешённые инструменты.
type Resolved struct {
	Profile      string   `json:"profile"`
	ConfigJSON   []byte   `json:"-"`
	AllowedTools []string `json:"allowed_tools"`
	Servers      []string `json:"servers"`
}

// Resolve — профиль задачи в конфиг Claude Code (--mcp-config). Неизвестный профиль или
// профиль не для этого окружения — ошибка.
func (r *Registry) Resolve(profile, env string) (Resolved, error) {
	if profile == "" {
		profile = "default"
	}
	p, ok := r.Profiles[profile]
	if !ok {
		return Resolved{}, fmt.Errorf("mcp profile %q is not in the registry", profile)
	}
	if len(p.Environments) > 0 && !contains(p.Environments, env) {
		return Resolved{}, fmt.Errorf("mcp profile %q is not allowed in environment %q", profile, env)
	}
	res := Resolved{Profile: profile, AllowedTools: append([]string{}, p.BuiltinTools...)}
	if len(p.Servers) == 0 {
		return res, nil
	}
	servers := map[string]any{}
	for _, name := range p.Servers {
		s := r.Servers[name]
		switch s.Type {
		case "stdio":
			servers[name] = map[string]any{"type": "stdio", "command": s.Command, "args": nonNil(s.Args), "env": s.Env}
		case "http":
			servers[name] = map[string]any{"type": "http", "url": s.URL}
		}
		// Только инструменты этого сервера: mcp__<сервер>__<инструмент> или весь сервер.
		if len(s.Tools) == 0 {
			res.AllowedTools = append(res.AllowedTools, "mcp__"+name)
		}
		for _, t := range s.Tools {
			res.AllowedTools = append(res.AllowedTools, "mcp__"+name+"__"+t)
		}
		res.Servers = append(res.Servers, name)
	}
	res.ConfigJSON, _ = json.Marshal(map[string]any{"mcpServers": servers})
	return res, nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}
