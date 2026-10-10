// Package contract — валидатор архитектурного контракта architecture.yaml (ТЗ п. 9.2,
// docs/stage7-4-checks.md, раздел 3). Нарушения ловит код, а не LLM.
package contract

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const FileName = "architecture.yaml"

type Module struct {
	Paths []string `yaml:"paths"`
}

type Contract struct {
	Version          int                 `yaml:"version"`
	Modules          map[string]Module   `yaml:"modules"`
	Allow            map[string][]string `yaml:"allow"`
	ForbiddenImports []string            `yaml:"forbidden_imports"`
	ProtectedPaths   []string            `yaml:"protected_paths"`
	ReadonlyPaths    []string            `yaml:"readonly_paths"`

	globs map[string][]*regexp.Regexp
}

// Parse читает контракт строго: неизвестные поля и ссылки на несуществующие модули — ошибка.
func Parse(data []byte) (*Contract, error) {
	var c Contract
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	if c.Version != 1 {
		return nil, fmt.Errorf("%s: unsupported version %d", FileName, c.Version)
	}
	c.globs = map[string][]*regexp.Regexp{}
	for name, m := range c.Modules {
		if len(m.Paths) == 0 {
			return nil, fmt.Errorf("%s: module %q has no paths", FileName, name)
		}
		for _, p := range m.Paths {
			c.globs[name] = append(c.globs[name], Glob(p))
		}
	}
	for from, tos := range c.Allow {
		if _, ok := c.Modules[from]; !ok {
			return nil, fmt.Errorf("%s: allow: unknown module %q", FileName, from)
		}
		for _, to := range tos {
			if _, ok := c.Modules[to]; !ok {
				return nil, fmt.Errorf("%s: allow.%s: unknown module %q", FileName, from, to)
			}
		}
	}
	return &c, nil
}

// Glob переводит шаблон пути (**, *, ?) в регулярное выражение; пути — через «/».
func Glob(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; {
		case strings.HasPrefix(pattern[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(pattern[i:], "**"):
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

func matchAny(patterns []string, p string) bool {
	for _, g := range patterns {
		if Glob(g).MatchString(p) {
			return true
		}
	}
	return false
}

// ModuleOf — модуль файла; пусто, если файл не относится ни к одному модулю.
func (c *Contract) ModuleOf(file string) string {
	var names []string
	for name, gs := range c.globs {
		for _, g := range gs {
			if g.MatchString(file) {
				names = append(names, name)
				break
			}
		}
	}
	sort.Strings(names) // детерминизм при пересечении шаблонов
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func (c *Contract) allowed(from, to string) bool {
	if from == to {
		return true
	}
	for _, m := range c.Allow[from] {
		if m == to {
			return true
		}
	}
	return false
}

func (c *Contract) forbidden(imp string) bool {
	for _, f := range c.ForbiddenImports {
		if imp == f || strings.HasPrefix(imp, strings.TrimSuffix(f, "/")+"/") {
			return true
		}
	}
	return false
}

type Violation struct {
	File   string `json:"file"`
	Rule   string `json:"rule"` // readonly_path | forbidden_import | forbidden_dependency
	Detail string `json:"detail"`
}

type Result struct {
	Present          bool        `json:"present"`
	Violations       []Violation `json:"violations,omitempty"`
	ApprovalRequired []string    `json:"approval_required,omitempty"`
}

func (r Result) String() string {
	var lines []string
	for _, v := range r.Violations {
		lines = append(lines, fmt.Sprintf("%s: %s — %s", v.File, v.Rule, v.Detail))
	}
	return strings.Join(lines, "\n")
}

// Project — то, что нужно знать о проекте, чтобы отличить свои импорты от чужих.
type Project struct {
	DartPackage string // name из pubspec.yaml
	GoModule    string // module из go.mod
}

// DetectProject читает pubspec.yaml и go.mod.
func DetectProject(read func(string) ([]byte, bool)) Project {
	var p Project
	if b, ok := read("pubspec.yaml"); ok {
		if m := regexp.MustCompile(`(?m)^name:\s*([A-Za-z0-9_]+)`).FindSubmatch(b); m != nil {
			p.DartPackage = string(m[1])
		}
	}
	if b, ok := read("go.mod"); ok {
		if m := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindSubmatch(b); m != nil {
			p.GoModule = string(m[1])
		}
	}
	return p
}

// Check проверяет изменённые файлы. read возвращает содержимое файла результата (false — файл удалён).
func (c *Contract) Check(changed []string, read func(string) ([]byte, bool), proj Project) Result {
	res := Result{Present: true}
	for _, f := range changed {
		f = path.Clean(strings.ReplaceAll(f, `\`, "/"))
		if matchAny(c.ReadonlyPaths, f) {
			res.Violations = append(res.Violations, Violation{File: f, Rule: "readonly_path", Detail: "agents must not modify this path"})
			continue
		}
		if matchAny(c.ProtectedPaths, f) {
			res.ApprovalRequired = append(res.ApprovalRequired, f)
		}
		data, ok := read(f)
		if !ok {
			continue
		}
		var imports []imported
		switch {
		case strings.HasSuffix(f, ".dart"):
			imports = dartImports(f, data, proj.DartPackage)
		case strings.HasSuffix(f, ".go"):
			imports = goImports(f, data, proj.GoModule)
		default:
			continue
		}
		from := c.ModuleOf(f)
		for _, imp := range imports {
			if imp.external != "" {
				if c.forbidden(imp.external) {
					res.Violations = append(res.Violations, Violation{File: f, Rule: "forbidden_import", Detail: imp.external})
				}
				continue
			}
			to := c.ModuleOf(imp.local)
			if from != "" && to != "" && !c.allowed(from, to) {
				res.Violations = append(res.Violations, Violation{File: f, Rule: "forbidden_dependency",
					Detail: fmt.Sprintf("module %s must not depend on %s (%s)", from, to, imp.local)})
			}
		}
	}
	return res
}

// imported — либо путь внутри репозитория (local), либо внешний пакет (external).
type imported struct{ local, external string }

var dartImportRe = regexp.MustCompile(`(?m)^\s*(?:import|export|part)\s+['"]([^'"]+)['"]`)

func dartImports(file string, src []byte, pkg string) []imported {
	var out []imported
	for _, m := range dartImportRe.FindAllSubmatch(src, -1) {
		spec := string(m[1])
		switch {
		case strings.HasPrefix(spec, "dart:"):
			out = append(out, imported{external: spec})
		case pkg != "" && strings.HasPrefix(spec, "package:"+pkg+"/"):
			out = append(out, imported{local: "lib/" + strings.TrimPrefix(spec, "package:"+pkg+"/")})
		case strings.HasPrefix(spec, "package:"):
			out = append(out, imported{external: spec})
		default:
			out = append(out, imported{local: path.Clean(path.Join(path.Dir(file), spec))})
		}
	}
	return out
}

func goImports(file string, src []byte, module string) []imported {
	f, err := parser.ParseFile(token.NewFileSet(), file, src, parser.ImportsOnly)
	if err != nil {
		return nil // синтаксис — дело компилятора (шаг 1)
	}
	var out []imported
	for _, is := range f.Imports {
		imp, _ := strconv.Unquote(is.Path.Value)
		switch {
		case module != "" && imp == module:
			out = append(out, imported{local: "_"})
		case module != "" && strings.HasPrefix(imp, module+"/"):
			// Импорт пакета = каталог; модуль определяется по условному файлу внутри.
			out = append(out, imported{local: strings.TrimPrefix(imp, module+"/") + "/_"})
		default:
			out = append(out, imported{external: imp})
		}
	}
	return out
}
