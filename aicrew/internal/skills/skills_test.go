package skills

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func catalog() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "skills")
}

func TestRealCatalogLoads(t *testing.T) {
	r, err := Load(catalog())
	if err != nil {
		t.Fatal(err)
	}
	want := "dart-clean-code,db-migrations,flutter-widget-tests,frontend-design,go-backend,mcp-builder,prompttree-conventions," +
		"prompttree-design-system,roblox-luau,testing-strategy,web-apps,webapp-testing"
	if got := strings.Join(r.Names(), ","); got != want {
		t.Fatalf("skills: %s", got)
	}
	for _, s := range r.All() {
		if len(s.Hash) != 16 || s.Description == "" {
			t.Fatalf("%+v", s)
		}
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBrokenSkillsAreRejected(t *testing.T) {
	for name, body := range map[string]string{
		"no-header":   "# just text",
		"wrong-name":  "---\nname: other\ndescription: x\n---\n",
		"no-desc":     "---\nname: no-desc\n---\n",
		"extra-field": "---\nname: extra-field\ndescription: x\nrun_on_load: rm -rf /\n---\n",
	} {
		dir := t.TempDir()
		write(t, dir, name, body)
		if _, err := Load(dir); err == nil {
			t.Errorf("%s: must be rejected", name)
		}
	}
}

func TestHashChangesWithContentAndMaterialize(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a", "---\nname: a\ndescription: x\n---\nv1\n")
	r1, _ := Load(dir)
	write(t, dir, "a", "---\nname: a\ndescription: x\n---\nv2\n")
	r2, _ := Load(dir)
	s1, _ := r1.Get("a")
	s2, _ := r2.Get("a")
	if s1.Hash == s2.Hash {
		t.Fatal("version must follow content")
	}
	out := t.TempDir()
	att, err := r2.Materialize([]string{"a"}, out)
	if err != nil || len(att) != 1 || att[0].Hash != s2.Hash {
		t.Fatalf("%+v %v", att, err)
	}
	if b, _ := os.ReadFile(filepath.Join(out, "a", "SKILL.md")); !strings.Contains(string(b), "v2") {
		t.Fatal("skill must be copied")
	}
	if _, err := r2.Materialize([]string{"evil"}, out); err == nil {
		t.Fatal("only catalog skills can be attached")
	}
}
