package host

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
)

// Junior работает только по контракту внутри плана Manager'а (решение 13.10.2026,
// docs/stage7-7-junior-contract.md): тесты пишет задача Claude и помечает пропущенными с меткой
// api.ContractMarker, Junior реализует код, снимает пропуск и не трогает ничего сверх контракта.

// juniorPrompt — условия контракта, добавляются к тексту задачи Junior.
func juniorPrompt(c *api.Contract) string {
	var b strings.Builder
	b.WriteString("\n\n---\nContract of this task (checked automatically):\n")
	b.WriteString("- Make these tests pass. They are currently skipped with the reason \"" + api.ContractMarker +
		"\": remove that skip, do not change what the tests check.\n")
	for _, t := range c.Tests {
		b.WriteString("    " + t + "\n")
	}
	b.WriteString("- You may change only these files (and the test files above):\n")
	for _, f := range c.Files {
		b.WriteString("    " + f + "\n")
	}
	b.WriteString("- Any change outside these files fails the task.\n")
	return b.String()
}

// checkJuniorContract — после работы Junior: изменены только файлы контракта, тесты-контракт без метки пропуска.
func checkJuniorContract(worktree string, changed []string, c *api.Contract) error {
	if c == nil {
		return &agent.Failure{Class: "AGENT_ERROR", Code: "no_contract", Message: "junior task has no contract"}
	}
	allowed := map[string]bool{}
	for _, p := range append(append([]string{}, c.Files...), c.Tests...) {
		allowed[path.Clean(filepath.ToSlash(p))] = true
	}
	var outside []string
	for _, p := range changed {
		if !allowed[path.Clean(filepath.ToSlash(p))] {
			outside = append(outside, p)
		}
	}
	if len(outside) > 0 {
		return &agent.Failure{Class: "AGENT_ERROR", Code: "contract_broken",
			Message: "junior changed files outside its contract: " + strings.Join(outside, ", ")}
	}
	for _, t := range c.Tests {
		b, err := os.ReadFile(filepath.Join(worktree, filepath.FromSlash(t)))
		if err != nil {
			return &agent.Failure{Class: "AGENT_ERROR", Code: "contract_unmet", Message: fmt.Sprintf("contract test %s: %v", t, err)}
		}
		if strings.Contains(string(b), api.ContractMarker) {
			return &agent.Failure{Class: "AGENT_ERROR", Code: "contract_unmet",
				Message: fmt.Sprintf("contract tests in %s are still skipped (%s)", t, api.ContractMarker)}
		}
	}
	return nil
}
