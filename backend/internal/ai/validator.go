package ai

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"prompttree/backend/internal/tree"
)

// Детерминированный валидатор (ТЗ п. 6.5): проверяет ответ модели относительно
// входа, а не доверяет промпту. Замечание не отклоняет результат молча —
// он становится предложением, и решает пользователь.

const (
	FindingNewNumber          = "new_number"
	FindingNewTerm            = "new_term"
	FindingHumanLost          = "human_paragraph_lost"
	FindingInjectionEcho      = "injection_echo"
	FindingTooLong            = "too_long"
	FindingEmpty              = "empty_result"
	maxFindingsPerKind        = 10
	lengthFactor, lengthSlack = 3, 3000
)

var (
	// Число — только отдельно стоящее: «2» в «OAuth2» относится к термину, его проверяет latinRe.
	numberRe   = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_])(\d+(?:[.,]\d+)*)`)
	listMarker = regexp.MustCompile(`(?m)^\s*\d+[.)]\s`)
	latinRe    = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*(?:[+#.\-'/][A-Za-z0-9]+)*[+#]*`)
	// Фрагменты, которых не может быть в честном ответе: эхо разделителей и системного промпта.
	injectionMarkers = []string{
		"<<<user_input", "user_input>>>", "ignore previous instructions", "ignore all previous",
		"zero hallucinations", "you are a technical editor", "игнорируй предыдущие", "system prompt",
	}
)

// Validate возвращает замечания к ответу модели.
func Validate(in Input, g tree.Generated) []tree.Finding {
	var corpus strings.Builder
	corpus.WriteString(in.Raw)
	if p := in.Previous; p != nil {
		corpus.WriteString("\n" + p.FormattedText)
		for _, h := range p.HumanParagraphs {
			corpus.WriteString("\n" + h)
		}
	}
	input := strings.ToLower(corpus.String())

	// Всё, кроме роли: роль — разрешённое служебное поле.
	parts := append(append(append([]string{}, g.Facts...), g.Constraints...), g.OpenQuestions...)
	parts = append(parts, g.FormattedText)
	output := strings.Join(parts, "\n")

	var findings []tree.Finding
	add := func(code, format string, args ...any) {
		n := 0
		for _, f := range findings {
			if f.Code == code {
				n++
			}
		}
		if n < maxFindingsPerKind {
			findings = append(findings, tree.Finding{Code: code, Detail: fmt.Sprintf(format, args...)})
		}
	}

	if strings.TrimSpace(g.FormattedText) == "" {
		add(FindingEmpty, "модель вернула пустой текст")
	}

	seen := map[string]bool{}
	for _, m := range numberRe.FindAllStringSubmatch(listMarker.ReplaceAllString(output, ""), -1) {
		num := m[1]
		if !seen[num] && !strings.Contains(input, num) {
			add(FindingNewNumber, "число «%s» отсутствует в исходнике", num)
		}
		seen[num] = true
	}

	cyrillicInput := mostlyCyrillic(in.Raw)
	for _, loc := range latinRe.FindAllStringIndex(output, -1) {
		term := output[loc[0]:loc[1]]
		key := strings.ToLower(term)
		if seen[key] || len(term) < 2 {
			continue
		}
		// В русском тексте любое латинское слово — кандидат в «додуманную» технологию.
		// В английском проверяем только то, что похоже на название: CamelCase, цифры, символы,
		// аббревиатуры и слова с заглавной буквы не в начале предложения.
		if !cyrillicInput && !looksLikeName(term) &&
			!(unicode.IsUpper(rune(term[0])) && !sentenceStart(output, loc[0])) {
			continue
		}
		seen[key] = true
		if !strings.Contains(input, key) {
			add(FindingNewTerm, "«%s» нет в исходнике", term)
		}
	}

	if p := in.Previous; p != nil {
		for _, h := range p.HumanParagraphs {
			if !strings.Contains(g.FormattedText, h) {
				add(FindingHumanLost, "потерян абзац, написанный вручную: «%s»", truncate(h, 80))
			}
		}
	}

	lowerOut := strings.ToLower(output + "\n" + g.Role)
	for _, m := range injectionMarkers {
		if strings.Contains(lowerOut, m) && !strings.Contains(input, m) {
			add(FindingInjectionEcho, "в ответе есть служебный фрагмент «%s»", m)
		}
	}

	if len(g.FormattedText) > lengthFactor*len(in.Raw)+lengthSlack {
		add(FindingTooLong, "ответ намного длиннее исходника (%d против %d символов)", len(g.FormattedText), len(in.Raw))
	}
	return findings
}

// mostlyCyrillic: заметная доля кириллицы — значит, латиница в тексте это названия, а не язык заметки.
func mostlyCyrillic(s string) bool {
	var cyr, lat int
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Cyrillic, r):
			cyr++
		case r < 128 && unicode.IsLetter(r):
			lat++
		}
	}
	return cyr > 0 && cyr*10 >= (cyr+lat)*3
}

// sentenceStart: слово стоит в начале предложения, строки или пункта списка.
func sentenceStart(text string, at int) bool {
	for i := at; i > 0; {
		r, size := utf8.DecodeLastRuneInString(text[:i])
		i -= size
		switch {
		case r == ' ' || r == '\t':
			continue
		case strings.ContainsRune(".!?:\n#*->(\"«•—", r):
			return true
		default:
			return false
		}
	}
	return true
}

func looksLikeName(t string) bool {
	upper, lowerAfterFirst, digitOrSym := 0, false, false
	for i, r := range t {
		switch {
		case unicode.IsUpper(r):
			upper++
			if i > 0 {
				lowerAfterFirst = true // CamelCase или аббревиатура
			}
		case unicode.IsDigit(r) || strings.ContainsRune("+#./-", r):
			digitOrSym = true
		}
	}
	return lowerAfterFirst || digitOrSym || (upper == len(t) && len(t) >= 2)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
