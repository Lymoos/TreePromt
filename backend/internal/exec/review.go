package exec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Ревью (ТЗ п. 9.3–9.5, docs/stage7-4-checks.md): модель только предлагает вердикт,
// статус двигает сервер, и только после проверки вердикта кодом.

const (
	VerdictApprove = "approve"
	VerdictReject  = "reject"
)

type ReviewIssue struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Severity string `json:"severity"` // blocker | major | minor
	Message  string `json:"message"`
}

type ReviewVerdict struct {
	Verdict       string        `json:"verdict"`
	RejectionType string        `json:"rejection_type,omitempty"` // code_bug | environment | qa_error | outdated_contract | contract
	Issues        []ReviewIssue `json:"issues"`
	Summary       string        `json:"summary"`
	Reviewer      string        `json:"reviewer,omitempty"` // logic_qa:<модель>, tech_lead:<модель>
}

var rejectionTypes = map[string]bool{"code_bug": true, "environment": true, "qa_error": true, "outdated_contract": true, "contract": true}
var severities = map[string]bool{"blocker": true, "major": true, "minor": true}

// Validate — ответ модели проверяется кодом: неизвестные значения и пустые отказы не принимаются.
func (v *ReviewVerdict) Validate() error {
	switch v.Verdict {
	case VerdictApprove:
		v.RejectionType = ""
	case VerdictReject:
		if !rejectionTypes[v.RejectionType] {
			return fmt.Errorf("rejection_type %q is not allowed", v.RejectionType)
		}
		if len(v.Issues) == 0 {
			return errors.New("reject without issues")
		}
	default:
		return fmt.Errorf("verdict %q is not allowed", v.Verdict)
	}
	if len(v.Issues) > 30 {
		v.Issues = v.Issues[:30]
	}
	for i := range v.Issues {
		is := &v.Issues[i]
		if !severities[is.Severity] {
			return fmt.Errorf("issue %d: severity %q is not allowed", i, is.Severity)
		}
		if strings.TrimSpace(is.Message) == "" {
			return fmt.Errorf("issue %d: empty message", i)
		}
		is.Message = clip(is.Message, 2000)
		is.File = clip(is.File, 300)
	}
	v.Summary = clip(strings.TrimSpace(v.Summary), 4000)
	return nil
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// ReviewSchema — JSON-схема ответа ревьюера для моделей со структурированным выводом (Gemini).
var ReviewSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"verdict":        map[string]any{"type": "string", "enum": []string{VerdictApprove, VerdictReject}},
		"rejection_type": map[string]any{"type": "string", "enum": []string{"code_bug", "environment", "qa_error", "outdated_contract"}},
		"issues": map[string]any{"type": "array", "items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file":     map[string]any{"type": "string"},
				"line":     map[string]any{"type": "integer"},
				"severity": map[string]any{"type": "string", "enum": []string{"blocker", "major", "minor"}},
				"message":  map[string]any{"type": "string"},
			},
			"required": []string{"file", "severity", "message"},
		}},
		"summary": map[string]any{"type": "string"},
	},
	"required": []string{"verdict", "issues", "summary"},
}

// ReviewModel — модель Logic QA на сервере (Gemini Flash; ключ только на VPS, решение 7.1).
type ReviewModel interface {
	Name() string
	GenerateJSON(ctx context.Context, system, user string, schema map[string]any) ([]byte, error)
}

// ErrReviewerNotConfigured — у сервера нет модели ревью; хост использует запасного ревьюера (решение 7.4/1).
var ErrReviewerNotConfigured = errors.New("logic QA model is not configured on the server")

// SetReviewer подключает модель Logic QA.
func (s *Service) SetReviewer(m ReviewModel) { s.reviewer = m }

// Review — Logic QA через модель сервера. Хост присылает промпт с фактами; сервер вызывает
// модель и возвращает проверенный вердикт. Статус здесь не меняется: его меняет Transition.
func (s *Service) Review(ctx context.Context, id Identity, taskID, workerID uuid.UUID, attempt int, system, user string) (ReviewVerdict, error) {
	if s.reviewer == nil {
		return ReviewVerdict{}, ErrReviewerNotConfigured
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ReviewVerdict{}, err
	}
	if _, err := workerOf(ctx, tx, id, workerID); err != nil {
		tx.Rollback(ctx)
		return ReviewVerdict{}, err
	}
	t, err := lockTask(ctx, tx, taskID)
	if err != nil {
		tx.Rollback(ctx)
		return ReviewVerdict{}, err
	}
	tx.Rollback(ctx) // только проверка прав; модель вызывается вне транзакции
	if !t.ownedBy(workerID, attempt) || t.status != StatusLLMReview {
		return ReviewVerdict{}, ErrNotOwner
	}
	raw, err := s.reviewer.GenerateJSON(ctx, system, user, ReviewSchema)
	if err != nil {
		return ReviewVerdict{}, err
	}
	var v ReviewVerdict
	if err := json.Unmarshal(raw, &v); err != nil {
		return ReviewVerdict{}, fmt.Errorf("%w: reviewer answer is not valid JSON", ErrInvalid)
	}
	if err := v.Validate(); err != nil {
		return ReviewVerdict{}, fmt.Errorf("%w: reviewer answer: %v", ErrInvalid, err)
	}
	v.Reviewer = "logic_qa:" + s.reviewer.Name()
	return v, nil
}
