package host

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
)

// Junior работает только по контракту внутри плана Manager'а (решение 13.10.2026,
// docs/stage7-7-junior-contract.md): тесты пишет задача Claude или Codex и помечает пропущенными меткой
// api.ContractMarker. Перед стартом Junior служба сама снимает метку (замер 18.10.2026: маленькие модели
// её не замечают), Junior только пишет код в файлах контракта; тесты он менять не может.

// skipMarker — пропуск теста-контракта: Dart `, skip: 'aicrew-contract'`, Go `t.Skip("aicrew-contract")`.
var skipMarker = regexp.MustCompile(`(?m)\s*,\s*skip:\s*['"]` + api.ContractMarker + `['"]|^[ \t]*t\.Skip\("` + api.ContractMarker + `"\)[ \t]*\r?\n?`)

// unskipContract снимает метку пропуска в тестах контракта и возвращает их новый текст: после работы Junior
// тесты должны остаться ровно такими.
func unskipContract(worktree string, c *api.Contract) (map[string]string, error) {
	tests := map[string]string{}
	for _, t := range c.Tests {
		p := filepath.Join(worktree, filepath.FromSlash(t))
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, &agent.Failure{Class: "AGENT_ERROR", Code: "contract_missing", Message: fmt.Sprintf("contract test %s: %v", t, err)}
		}
		text := skipMarker.ReplaceAllString(string(b), "")
		if text == string(b) {
			return nil, &agent.Failure{Class: "AGENT_ERROR", Code: "contract_missing",
				Message: fmt.Sprintf("contract test %s has no %q skip: the contract task must write and skip the tests", t, api.ContractMarker)}
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			return nil, err
		}
		tests[path.Clean(filepath.ToSlash(t))] = text
	}
	return tests, nil
}

// juniorPrompt — условия контракта, добавляются к тексту задачи Junior.
func juniorPrompt(c *api.Contract) string {
	var b strings.Builder
	b.WriteString("\n\n---\nContract of this task (checked automatically):\n")
	b.WriteString("- These tests define the work. They are enabled and currently fail; make them pass. Do not change them:\n")
	for _, t := range c.Tests {
		b.WriteString("    " + t + "\n")
	}
	b.WriteString("- Change only these files:\n")
	for _, f := range c.Files {
		b.WriteString("    " + f + "\n")
	}
	b.WriteString("- Any other change, including to the tests, fails the task.\n")
	return b.String()
}

// generatedByTools — файлы, которые пересоздают сами инструменты при сборке и тестах (не работа модели).
func generatedByTools(p string) bool {
	base := path.Base(p)
	return strings.HasPrefix(base, "generated_plugin_registrant.") || base == "generated_plugins.cmake"
}

// checkJuniorContract — после работы Junior: изменены только файлы контракта, тесты — ровно те, что выдала служба.
func checkJuniorContract(worktree string, changed []string, c *api.Contract, tests map[string]string) error {
	if c == nil {
		return &agent.Failure{Class: "AGENT_ERROR", Code: "no_contract", Message: "junior task has no contract"}
	}
	allowed := map[string]bool{}
	for _, p := range append(append([]string{}, c.Files...), c.Tests...) {
		allowed[path.Clean(filepath.ToSlash(p))] = true
	}
	var outside []string
	for _, p := range changed {
		p = path.Clean(filepath.ToSlash(p))
		if !allowed[p] && !generatedByTools(p) {
			outside = append(outside, p)
		}
	}
	if len(outside) > 0 {
		return &agent.Failure{Class: "AGENT_ERROR", Code: "contract_broken",
			Message: "junior changed files outside its contract: " + strings.Join(outside, ", ")}
	}
	for t, want := range tests {
		b, err := os.ReadFile(filepath.Join(worktree, filepath.FromSlash(t)))
		if err != nil || string(b) != want {
			return &agent.Failure{Class: "AGENT_ERROR", Code: "contract_broken",
				Message: fmt.Sprintf("junior changed the contract tests in %s", t)}
		}
	}
	return nil
}
