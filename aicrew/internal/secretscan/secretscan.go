// Package secretscan — сканер секретов (ТЗ п. 12; docs/stage7-6-security-audit.md, раздел 1):
// перед автокоммитом, перед отправкой текста моделям и перед сохранением фактов и логов.
// Сначала точные значения секретов этого хоста, затем известные форматы, затем присваивания
// password=/token=/secret= с высокоэнтропийным значением.
package secretscan

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

type Finding struct {
	Kind string `json:"kind"`
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
}

type pattern struct {
	kind string
	re   *regexp.Regexp
	// group — номер группы со значением (0 — всё совпадение).
	group int
}

var patterns = []pattern{
	{"private_key", regexp.MustCompile(`-----BEGIN (?:[A-Z]+ )*PRIVATE KEY-----`), 0},
	{"anthropic_key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_\-]{20,}`), 0},
	{"openai_key", regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9_\-]{32,}`), 0},
	{"google_api_key", regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`), 0},
	{"github_token", regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36}\b|\bgithub_pat_[A-Za-z0-9_]{50,}`), 0},
	{"slack_token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9\-]{10,}`), 0},
	{"aws_access_key", regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), 0},
	{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}`), 0},
	{"url_password", regexp.MustCompile(`\b[a-z][a-z0-9+.\-]*://[^:\s/@]+:([^@\s/]{6,})@`), 1},
	{"assigned_secret", regexp.MustCompile(`(?i)\b(?:password|passwd|pwd|secret|token|api[_\-]?key|access[_\-]?key|client[_\-]?secret)\b["']?\s*[:=]\s*["'` + "`" + `]?([^\s"'` + "`" + `,;)]{12,})`), 1},
}

// placeholders — значения-заглушки в примерах и тестах не считаются секретами.
var placeholders = []string{"example", "changeme", "change_me", "placeholder", "dummy", "xxxx", "your", "redacted",
	"test", "fake", "sample", "<", "${", "{{", "process.env", "os.getenv", "getenv", "env:"}

type Scanner struct {
	exact map[string]string // значение → вид
}

// New — exact: точные значения секретов хоста (токен Claude, пароль ai_worker…). Короткие не берутся.
func New(exact map[string]string) *Scanner {
	s := &Scanner{exact: map[string]string{}}
	for v, kind := range exact {
		if len(strings.TrimSpace(v)) >= 8 {
			s.exact[v] = kind
		}
	}
	return s
}

func isPlaceholder(v string) bool {
	low := strings.ToLower(v)
	for _, p := range placeholders {
		if strings.Contains(low, p) {
			return true
		}
	}
	return false
}

// entropy — бит на символ (Шеннон).
func entropy(s string) float64 {
	if s == "" {
		return 0
	}
	freq := map[rune]float64{}
	for _, r := range s {
		freq[r]++
	}
	n := float64(len([]rune(s)))
	var h float64
	for _, c := range freq {
		p := c / n
		h -= p * math.Log2(p)
	}
	return h
}

type match struct {
	kind       string
	start, end int
}

func (s *Scanner) matches(text string) []match {
	var out []match
	for v, kind := range s.exact {
		for i := strings.Index(text, v); i >= 0; {
			out = append(out, match{kind, i, i + len(v)})
			j := strings.Index(text[i+len(v):], v)
			if j < 0 {
				break
			}
			i += len(v) + j
		}
	}
	for _, p := range patterns {
		for _, m := range p.re.FindAllStringSubmatchIndex(text, -1) {
			st, en := m[0], m[1]
			if p.group > 0 {
				st, en = m[2*p.group], m[2*p.group+1]
			}
			v := text[st:en]
			if isPlaceholder(v) {
				continue
			}
			if p.kind == "assigned_secret" && entropy(v) < 3.5 {
				continue // «password = something_simple» — не секрет, а слово
			}
			out = append(out, match{p.kind, st, en})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out
}

// Scan — найденные секреты в тексте (без самих значений: в отчёт они не попадают).
func (s *Scanner) Scan(text string) []Finding {
	var out []Finding
	for _, m := range s.matches(text) {
		out = append(out, Finding{Kind: m.kind, Line: strings.Count(text[:m.start], "\n") + 1})
	}
	return out
}

// Redact заменяет найденное на [REDACTED:вид].
func (s *Scanner) Redact(text string) string {
	ms := s.matches(text)
	if len(ms) == 0 {
		return text
	}
	var b strings.Builder
	last := 0
	for _, m := range ms {
		if m.start < last {
			continue // пересечение — уже заменено
		}
		b.WriteString(text[last:m.start])
		b.WriteString("[REDACTED:" + m.kind + "]")
		last = m.end
	}
	b.WriteString(text[last:])
	return b.String()
}

// ScanDiff — секреты в добавленных строках unified diff (git diff -U0): файл и номер строки.
func (s *Scanner) ScanDiff(diff string) []Finding {
	var out []Finding
	file, line := "", 0
	hunk := regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)`)
	for _, l := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(l, "+++ "):
			file = strings.TrimPrefix(strings.TrimPrefix(l, "+++ "), "b/")
		case strings.HasPrefix(l, "@@"):
			if m := hunk.FindStringSubmatch(l); m != nil {
				line = atoi(m[1])
			}
		case strings.HasPrefix(l, "+"):
			for _, f := range s.Scan(l[1:]) {
				out = append(out, Finding{Kind: f.Kind, File: file, Line: line})
			}
			line++
		case strings.HasPrefix(l, " "):
			line++
		}
	}
	return out
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
