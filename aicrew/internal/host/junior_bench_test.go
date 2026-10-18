package host

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/runner"
)

// Сравнение локальных моделей на задачах-контрактах Junior (по желанию, не в CI):
//
//	AICREW_BENCH_REPO=<репозиторий с контрактами> AICREW_BENCH_MODELS=qwen3:8b,qwen2.5-coder:7b \
//	go test -run JuniorBench -v -timeout 3h ./internal/host
//
// Каждый контракт — в свежей копии репозитория, до двух попыток (вторая знает, почему не прошла первая),
// проверка как у службы: только файлы контракта, без метки пропуска, analyze и тесты контракта зелёные.
func TestJuniorBench(t *testing.T) {
	repo := os.Getenv("AICREW_BENCH_REPO")
	if repo == "" {
		t.Skip("AICREW_BENCH_REPO is not set")
	}
	models := strings.Split(os.Getenv("AICREW_BENCH_MODELS"), ",")
	contracts := []struct {
		name, prompt string
		c            api.Contract
	}{
		{"filter", "Реализуй filterByText в lib/logic/filter.dart.", api.Contract{Files: []string{"lib/logic/filter.dart"}, Tests: []string{"test/logic/filter_test.dart"}}},
		{"sort", "Реализуй sortActiveFirst в lib/logic/sort.dart.", api.Contract{Files: []string{"lib/logic/sort.dart"}, Tests: []string{"test/logic/sort_test.dart"}}},
		{"json", "Реализуй todoToJson и todoFromJson в lib/logic/json.dart.", api.Contract{Files: []string{"lib/logic/json.dart"}, Tests: []string{"test/logic/json_test.dart"}}},
		{"stats", "Реализуй countTodos и геттеры TodoStats в lib/logic/stats.dart.", api.Contract{Files: []string{"lib/logic/stats.dart"}, Tests: []string{"test/logic/stats_test.dart"}}},
	}
	run := runner.New("docker", "git")
	var report []string
	for _, model := range models {
		passed, firstTry, total := 0, 0, time.Duration(0)
		for _, ct := range contracts {
			dir := filepath.Join(t.TempDir(), "w")
			if out, err := exec.Command("git", "clone", "-q", "--local", repo, dir).CombinedOutput(); err != nil {
				t.Fatalf("clone: %v %s", err, out)
			}
			l := &agent.Local{R: run, Image: "aicrew-agent-flutter:local", BaseURL: "http://127.0.0.1:11434", Model: model, NumCtx: 16384}
			var previous []string
			result := "FAILED"
			start := time.Now()
			turns := 0
			for attempt := 1; attempt <= 2; attempt++ {
				res, err := l.Run(context.Background(), agent.Request{TaskID: "bench-" + ct.name, Attempt: attempt, Workdir: dir,
					Prompt: ct.prompt + juniorPrompt(&ct.c), Timeout: 20 * time.Minute, MaxTurns: 40, Previous: previous,
					Checks: []string{"flutter analyze lib/logic test/logic", "flutter test " + ct.c.Tests[0]}})
				turns += res.Usage.Turns
				why := ""
				if err != nil {
					why = err.Error()
				} else if err := checkJuniorContract(dir, changedFiles(t, dir), &ct.c); err != nil {
					why = err.Error()
				} else if out, ok := verifyContract(dir, ct.c.Tests[0]); !ok {
					why = "checks failed: " + out
				}
				if why == "" {
					result = fmt.Sprintf("passed on attempt %d", attempt)
					passed++
					if attempt == 1 {
						firstTry++
					}
					break
				}
				t.Logf("%s / %s attempt %d: %s", model, ct.name, attempt, firstLine(why))
				previous = append(previous, why)
			}
			took := time.Since(start).Round(time.Second)
			total += took
			report = append(report, fmt.Sprintf("%-18s %-7s %-20s turns %-3d %s", model, ct.name, result, turns, took))
		}
		report = append(report, fmt.Sprintf("%-18s TOTAL   %d/%d passed, %d on first try, %s", model, passed, len(contracts), firstTry, total))
	}
	t.Log("\n" + strings.Join(report, "\n"))
}

func changedFiles(t *testing.T, dir string) []string {
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain", "-uall").Output()
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if len(l) > 3 {
			files = append(files, strings.TrimSpace(l[3:]))
		}
	}
	return files
}

// verifyContract — то же, что проверка службы для этой части: analyze и тесты контракта в чистом контейнере.
func verifyContract(dir, test string) (string, bool) {
	cmd := exec.Command("docker", "run", "--rm", "-v", dir+":/work", "-w", "/work", "aicrew-agent-flutter:local", "bash", "-c",
		"flutter pub get >/dev/null 2>&1 && flutter analyze lib/logic test/logic && flutter test "+test)
	out, err := cmd.CombinedOutput()
	s := string(out)
	if len(s) > 600 {
		s = s[len(s)-600:]
	}
	return s, err == nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
