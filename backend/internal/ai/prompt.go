// Package ai — AI Structuring Engine (ТЗ п. 6, docs/stage6-ai-structuring.md):
// промпт, клиент Gemini, детерминированный валидатор и очередь запросов.
package ai

import (
	_ "embed"
	"encoding/json"
	"strings"

	"prompttree/backend/internal/tree"
)

//go:embed prompts/structure_v1.md
var systemPromptV1 string

// PromptVersion хранится в каждом запросе: по нему видно, каким промптом получен результат.
const PromptVersion = "structure_v1"

func SystemPrompt() string { return systemPromptV1 }

const (
	openTag  = "<<<USER_INPUT"
	closeTag = "USER_INPUT>>>"
)

// escapeData не даёт тексту пользователя закрыть блок данных раньше времени.
func escapeData(s string) string {
	s = strings.ReplaceAll(s, openTag, "<<< USER_INPUT")
	return strings.ReplaceAll(s, closeTag, "USER_INPUT >>>")
}

func block(name, body string) string {
	return name + ":\n" + openTag + "\n" + escapeData(body) + "\n" + closeTag + "\n\n"
}

// Input — всё, что модель получает о задаче.
type Input struct {
	Raw      string
	Previous *tree.StructuredDoc
}

// BuildUserMessage собирает сообщение: только данные в разделителях, без инструкций.
func BuildUserMessage(in Input) string {
	var b strings.Builder
	if prev := in.Previous; prev != nil && strings.TrimSpace(prev.FormattedText) != "" {
		b.WriteString(block("PREVIOUS_STRUCTURED", prev.FormattedText))
		if len(prev.HumanParagraphs) > 0 {
			js, _ := json.MarshalIndent(prev.HumanParagraphs, "", "  ")
			b.WriteString(block("HUMAN_PARAGRAPHS", string(js)))
		}
		if prev.SourceRaw != "" && prev.SourceRaw != in.Raw {
			b.WriteString(block("CHANGES_SINCE_PREVIOUS", LineDiff(prev.SourceRaw, in.Raw)))
		}
	}
	b.WriteString(block("RAW_NOTES", in.Raw))
	return b.String()
}

// responseSchema — JSON-схема ответа Gemini (формат OpenAPI-подмножества).
var responseSchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"role":           map[string]any{"type": "STRING"},
		"facts":          map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
		"constraints":    map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
		"open_questions": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
		"formatted_text": map[string]any{"type": "STRING"},
	},
	"required":         []string{"role", "facts", "constraints", "open_questions", "formatted_text"},
	"propertyOrdering": []string{"role", "facts", "constraints", "open_questions", "formatted_text"},
}
