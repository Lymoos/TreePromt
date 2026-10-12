// Package skills — реестр скиллов (ТЗ п. 10.2, docs/stage7-5-manager-skills-mcp.md, раздел 2).
// Скилл — каталог <имя>/SKILL.md с метаданными в заголовке (YAML между ---) и ресурсами.
// Скилл — недоверенный контент уровня кода: только из каталога, версия фиксируется хэшем.
package skills

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	When        string   `yaml:"when" json:"when,omitempty"`
	Roles       []string `yaml:"roles" json:"roles,omitempty"`
	Projects    []string `yaml:"projects" json:"projects,omitempty"`
	Version     string   `yaml:"version" json:"version,omitempty"`
	// License — для скиллов из чужих каталогов (условия — в LICENSE.txt рядом).
	License string `yaml:"license" json:"license,omitempty"`

	Dir  string `yaml:"-" json:"-"`
	Hash string `yaml:"-" json:"hash"`
}

type Registry struct {
	byName map[string]Skill
}

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Load читает каталог скиллов. Ошибка в любом скилле — ошибка загрузки: битый скилл не подключается молча.
func Load(dir string) (*Registry, error) {
	r := &Registry{byName: map[string]Skill{}}
	if dir == "" {
		return r, nil
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sd := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(filepath.Join(sd, "SKILL.md"))
		if err != nil {
			return nil, fmt.Errorf("skill %s: %w", e.Name(), err)
		}
		s, err := parseHeader(raw)
		if err != nil {
			return nil, fmt.Errorf("skill %s: %w", e.Name(), err)
		}
		if s.Name != e.Name() || !nameRe.MatchString(s.Name) {
			return nil, fmt.Errorf("skill %s: name must match the folder and be lowercase-kebab", e.Name())
		}
		if strings.TrimSpace(s.Description) == "" {
			return nil, fmt.Errorf("skill %s: description is required", s.Name)
		}
		s.Dir = sd
		if s.Hash, err = hashDir(sd); err != nil {
			return nil, err
		}
		r.byName[s.Name] = s
	}
	return r, nil
}

func parseHeader(raw []byte) (Skill, error) {
	var s Skill
	raw = bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(raw, []byte("---\n")) {
		return s, fmt.Errorf("SKILL.md must start with a --- header")
	}
	end := bytes.Index(raw[4:], []byte("\n---"))
	if end < 0 {
		return s, fmt.Errorf("SKILL.md header is not closed")
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw[4 : 4+end]))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return s, fmt.Errorf("SKILL.md header: %w", err)
	}
	return s, nil
}

// hashDir — версия скилла: sha256 по путям и содержимому всех файлов.
func hashDir(dir string) (string, error) {
	h := sha256.New()
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return err
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		rel, _ := filepath.Rel(dir, f)
		fmt.Fprintf(h, "%s\x00%d\x00", filepath.ToSlash(rel), len(b))
		h.Write(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")))
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

func (r *Registry) Names() []string {
	var out []string
	for n := range r.byName {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func (r *Registry) All() []Skill {
	var out []Skill
	for _, n := range r.Names() {
		out = append(out, r.byName[n])
	}
	return out
}

func (r *Registry) Get(name string) (Skill, bool) {
	s, ok := r.byName[name]
	return s, ok
}

// Attached — что подключено к попытке (в факты и аудит).
type Attached struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
}

// Materialize копирует выбранные скиллы в dir/<имя> — этот каталог монтируется исполнителю
// только для чтения. Неизвестный скилл — ошибка: подключается только каталог.
func (r *Registry) Materialize(names []string, dir string) ([]Attached, error) {
	var out []Attached
	for _, n := range names {
		s, ok := r.byName[n]
		if !ok {
			return nil, fmt.Errorf("skill %q is not in the registry", n)
		}
		if err := copyDir(s.Dir, filepath.Join(dir, n)); err != nil {
			return nil, err
		}
		out = append(out, Attached{Name: n, Hash: s.Hash})
	}
	return out, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}
