package ai

import "strings"

const maxDiffLines = 2000

// LineDiff — построчная разница (LCS): «+ » добавлено, «- » удалено.
// Для очень длинных текстов вместо дельты возвращается пометка: модель всё равно получает весь исходник.
func LineDiff(before, after string) string {
	a := strings.Split(strings.ReplaceAll(before, "\r\n", "\n"), "\n")
	b := strings.Split(strings.ReplaceAll(after, "\r\n", "\n"), "\n")
	if len(a) > maxDiffLines || len(b) > maxDiffLines {
		return "(the notes changed substantially; use RAW_NOTES)"
	}
	// lcs[i][j] — длина общей подпоследовательности a[i:] и b[j:].
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var out []string
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			i, j = i+1, j+1
		case j < len(b) && (i == len(a) || lcs[i][j+1] >= lcs[i+1][j]):
			out = append(out, "+ "+b[j])
			j++
		default:
			out = append(out, "- "+a[i])
			i++
		}
	}
	return strings.Join(out, "\n")
}
