package contract

import (
	"strings"
	"testing"
)

const flutterContract = `
version: 1
modules:
  ui:     { paths: ["lib/ui/**", "lib/main.dart"] }
  domain: { paths: ["lib/domain/**"] }
  data:   { paths: ["lib/data/**"] }
allow:
  ui: [domain]
  data: [domain]
forbidden_imports: ["dart:mirrors", "package:http"]
protected_paths: [pubspec.yaml, "**/migrations/**", architecture.yaml]
readonly_paths: [".github/**"]
`

func files(m map[string]string) func(string) ([]byte, bool) {
	return func(p string) ([]byte, bool) {
		s, ok := m[p]
		return []byte(s), ok
	}
}

func TestFlutterContract(t *testing.T) {
	c, err := Parse([]byte(flutterContract))
	if err != nil {
		t.Fatal(err)
	}
	repo := map[string]string{
		"pubspec.yaml": "name: todo\n",
		"lib/ui/list.dart": `import 'package:flutter/material.dart';
import 'package:todo/domain/todo.dart';
import '../data/store.dart';          // ui → data запрещено
import 'package:http/http.dart' as h;  // запрещённый пакет
`,
		"lib/domain/todo.dart": "import 'dart:mirrors';\n",
		"lib/data/store.dart":  "import 'package:todo/domain/todo.dart';\n",
		"lib/main.dart":        "import 'ui/list.dart';\n",
	}
	changed := []string{"lib/ui/list.dart", "lib/domain/todo.dart", "lib/data/store.dart", "lib/main.dart",
		"pubspec.yaml", "db/migrations/001.sql", ".github/workflows/ci.yml"}
	res := c.Check(changed, files(repo), DetectProject(files(repo)))

	want := map[string]bool{
		"lib/ui/list.dart forbidden_dependency":  true,
		"lib/ui/list.dart forbidden_import":      true,
		"lib/domain/todo.dart forbidden_import":  true,
		".github/workflows/ci.yml readonly_path": true,
	}
	if len(res.Violations) != len(want) {
		t.Fatalf("violations:\n%s", res)
	}
	for _, v := range res.Violations {
		if !want[v.File+" "+v.Rule] {
			t.Errorf("unexpected %+v", v)
		}
	}
	if strings.Join(res.ApprovalRequired, ",") != "pubspec.yaml,db/migrations/001.sql" {
		t.Fatalf("approval: %v", res.ApprovalRequired)
	}
}

func TestGoContract(t *testing.T) {
	c, err := Parse([]byte(`
version: 1
modules:
  api:  { paths: ["internal/httpapi/**"] }
  core: { paths: ["internal/core/**"] }
allow:
  api: [core]
forbidden_imports: ["unsafe", "github.com/evil"]
`))
	if err != nil {
		t.Fatal(err)
	}
	repo := map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.23\n",
		"internal/core/user.go": `package core
import (
	"fmt"
	"example.com/app/internal/httpapi"
	"github.com/evil/pkg"
)`,
		"internal/httpapi/h.go": `package httpapi
import "example.com/app/internal/core"`,
	}
	res := c.Check([]string{"internal/core/user.go", "internal/httpapi/h.go"}, files(repo), DetectProject(files(repo)))
	if len(res.Violations) != 2 {
		t.Fatalf("violations:\n%s", res)
	}
	if !strings.Contains(res.String(), "core must not depend on api") || !strings.Contains(res.String(), "github.com/evil/pkg") {
		t.Fatal(res.String())
	}
}

func TestDeletedFilesOnlyCheckPaths(t *testing.T) {
	c, _ := Parse([]byte(flutterContract))
	res := c.Check([]string{"lib/ui/gone.dart", ".github/x.yml"}, files(map[string]string{}), Project{})
	if len(res.Violations) != 1 || res.Violations[0].Rule != "readonly_path" {
		t.Fatalf("%s", res)
	}
}

func TestStrictParsing(t *testing.T) {
	for _, bad := range []string{
		"version: 2\n",
		"version: 1\nmodulez: {}\n",
		"version: 1\nmodules: {a: {paths: [x]}}\nallow: {a: [b]}\n",
		"version: 1\nmodules: {a: {}}\n",
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("must reject:\n%s", bad)
		}
	}
}

func TestGlob(t *testing.T) {
	cases := []struct {
		g, p string
		ok   bool
	}{
		{"lib/ui/**", "lib/ui/a/b.dart", true},
		{"lib/ui/**", "lib/uix/a.dart", false},
		{"**/migrations/**", "migrations/1.sql", true},
		{"**/migrations/**", "db/migrations/1.sql", true},
		{"*.yaml", "pubspec.yaml", true},
		{"*.yaml", "a/pubspec.yaml", false},
	}
	for _, c := range cases {
		if Glob(c.g).MatchString(c.p) != c.ok {
			t.Errorf("%s ~ %s: want %v", c.g, c.p, c.ok)
		}
	}
}
