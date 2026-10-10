package verify

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// TestReport — test_report.json (ТЗ п. 9): сколько тестов, какие упали и почему.
// Собирается кодом из машинного вывода, а не из рассказа модели.
type TestReport struct {
	Total    int           `json:"total"`
	Passed   int           `json:"passed"`
	Failed   int           `json:"failed"`
	Skipped  int           `json:"skipped"`
	Failures []TestFailure `json:"failures,omitempty"`
}

type TestFailure struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

const maxFailures = 20

func (r *TestReport) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "tests: %d passed, %d failed, %d skipped", r.Passed, r.Failed, r.Skipped)
	for _, f := range r.Failures {
		fmt.Fprintf(&b, "\nFAILED %s\n%s", f.Name, indent(clip(f.Message, 1200)))
	}
	return b.String()
}

func (r *TestReport) addFailure(name, msg string) {
	if len(r.Failures) < maxFailures {
		r.Failures = append(r.Failures, TestFailure{Name: name, Message: strings.TrimSpace(msg)})
	}
}

// ParseFlutterMachine разбирает `flutter test --machine` (JSON-события по строке).
// nil — если событий нет (тесты не запустились: ошибка компиляции видна в выводе шага).
func ParseFlutterMachine(out string) *TestReport {
	type test struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	var ev struct {
		Type   string `json:"type"`
		Test   test   `json:"test"`
		TestID int    `json:"testID"`
		Result string `json:"result"`
		Hidden bool   `json:"hidden"`
		Skip   bool   `json:"skipped"`
		Error  string `json:"error"`
		Stack  string `json:"stackTrace"`
	}
	names := map[int]string{}
	errs := map[int]*strings.Builder{}
	var rep TestReport
	seen := false
	var failedIDs []int
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		ev.Type, ev.Error, ev.Stack, ev.Result, ev.Hidden, ev.Skip = "", "", "", "", false, false
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "testStart":
			seen = true
			names[ev.Test.ID] = ev.Test.Name
		case "error":
			b := errs[ev.TestID]
			if b == nil {
				b = &strings.Builder{}
				errs[ev.TestID] = b
			}
			b.WriteString(ev.Error + "\n" + firstLines(ev.Stack, 6) + "\n")
		case "testDone":
			if ev.Hidden { // служебные «loading …» и setUpAll
				if ev.Result != "success" {
					failedIDs = append(failedIDs, ev.TestID)
					rep.Failed++
					rep.Total++
				}
				continue
			}
			rep.Total++
			switch {
			case ev.Skip:
				rep.Skipped++
			case ev.Result == "success":
				rep.Passed++
			default:
				rep.Failed++
				failedIDs = append(failedIDs, ev.TestID)
			}
		}
	}
	if !seen {
		return nil
	}
	for _, id := range failedIDs {
		msg := ""
		if b := errs[id]; b != nil {
			msg = b.String()
		}
		rep.addFailure(names[id], msg)
	}
	return &rep
}

// ParseGoJSON разбирает `go test -json`.
func ParseGoJSON(out string) *TestReport {
	var ev struct {
		Action  string `json:"Action"`
		Package string `json:"Package"`
		Test    string `json:"Test"`
		Output  string `json:"Output"`
	}
	output := map[string]*strings.Builder{}
	var rep TestReport
	var failed []string
	seen := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		ev.Action, ev.Package, ev.Test, ev.Output = "", "", "", ""
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		seen = true
		key := ev.Package + "." + ev.Test
		switch ev.Action {
		case "output":
			if ev.Test != "" {
				b := output[key]
				if b == nil {
					b = &strings.Builder{}
					output[key] = b
				}
				if b.Len() < 4000 {
					b.WriteString(ev.Output)
				}
			}
		case "pass", "fail", "skip":
			if ev.Test == "" {
				if ev.Action == "fail" && !hasTestsOf(failed, ev.Package) {
					// Пакет упал без упавших тестов — ошибка сборки пакета.
					failed = append(failed, ev.Package+".")
					rep.Failed++
					rep.Total++
				}
				continue
			}
			if strings.Contains(ev.Test, "/") {
				continue // подтесты считаются в родителе
			}
			rep.Total++
			switch ev.Action {
			case "pass":
				rep.Passed++
			case "skip":
				rep.Skipped++
			default:
				rep.Failed++
				failed = append(failed, key)
			}
		}
	}
	if !seen {
		return nil
	}
	sort.Strings(failed)
	for _, key := range failed {
		msg := ""
		if b := output[key]; b != nil {
			msg = b.String()
		}
		rep.addFailure(strings.TrimSuffix(key, "."), msg)
	}
	return &rep
}

func hasTestsOf(failed []string, pkg string) bool {
	for _, f := range failed {
		if strings.HasPrefix(f, pkg+".") {
			return true
		}
	}
	return false
}

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func indent(s string) string {
	return "  " + strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n  ")
}
