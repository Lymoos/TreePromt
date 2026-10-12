package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"prompttree/backend/internal/tree"
)

// Model — источник структурирования. Реализация — Gemini; в тестах — имитация.
type Model interface {
	Name() string
	Generate(ctx context.Context, system, user string) (tree.Generated, error)
}

// ErrModelAuth — ключ не подходит: повторять бессмысленно, нужен человек.
var ErrModelAuth = errors.New("Gemini rejected the API key")

type Gemini struct {
	mu      sync.Mutex
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
	retries int
	backoff time.Duration
}

func NewGemini(apiKey, model string) *Gemini {
	return &Gemini{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://generativelanguage.googleapis.com",
		http:    &http.Client{Timeout: 90 * time.Second},
		retries: 3,
		backoff: 2 * time.Second,
	}
}

func (g *Gemini) Name() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.model
}

// WithBaseURL подменяет адрес API (имитация в тестах).
func (g *Gemini) WithBaseURL(u string) *Gemini {
	if u != "" {
		g.baseURL = u
	}
	return g
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

// Generate вызывает generateContent с JSON-схемой ответа структурирования.
func (g *Gemini) Generate(ctx context.Context, system, user string) (tree.Generated, error) {
	text, err := g.GenerateJSON(ctx, system, user, responseSchema)
	if err != nil {
		return tree.Generated{}, err
	}
	var out tree.Generated
	if err := json.Unmarshal(text, &out); err != nil {
		return tree.Generated{}, fmt.Errorf("Gemini answer is not valid JSON: %w", err)
	}
	return out, nil
}

// GenerateJSON вызывает generateContent со схемой ответа и возвращает текст ответа (JSON).
// Ключ — только в заголовке, в URL и тексты ошибок он не попадает. Им же пользуется Logic QA (7.4).
func (g *Gemini) GenerateJSON(ctx context.Context, system, user string, schema map[string]any) ([]byte, error) {
	body, _ := json.Marshal(map[string]any{
		"systemInstruction": map[string]any{"parts": []any{map[string]any{"text": system}}},
		"contents":          []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": user}}}},
		"generationConfig": map[string]any{
			"temperature":      0.2,
			"responseMimeType": "application/json",
			"responseSchema":   schema,
		},
	})
	healed := false
retry:
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", g.baseURL, g.Name())

	var lastErr error
	for attempt := 0; attempt <= g.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(g.backoff * time.Duration(1<<(attempt-1))):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", g.apiKey)
		resp, err := g.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("Gemini is unreachable: %w", err)
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			return nil, ErrModelAuth
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("Gemini is busy (HTTP %d)", resp.StatusCode)
			continue
		case resp.StatusCode == http.StatusNotFound && !healed:
			// Google выводит модели из обращения (12.10.2026: gemini-2.5-flash — «no longer available to new users»).
			// Берём самую новую flash-модель, доступную этому ключу, и повторяем.
			if next, err := g.newestFlash(ctx); err == nil && next != g.Name() {
				g.mu.Lock()
				g.model = next
				g.mu.Unlock()
				healed = true
				goto retry
			}
			return nil, fmt.Errorf("Gemini model %s is not available (HTTP 404): set GEMINI_MODEL", g.Name())
		case resp.StatusCode != http.StatusOK:
			return nil, fmt.Errorf("Gemini returned HTTP %d", resp.StatusCode)
		}
		return parseGemini(data)
	}
	return nil, lastErr
}

func parseGemini(data []byte) ([]byte, error) {
	var r geminiResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("unreadable Gemini response: %w", err)
	}
	if r.PromptFeedback.BlockReason != "" {
		return nil, fmt.Errorf("Gemini blocked the request: %s", r.PromptFeedback.BlockReason)
	}
	if len(r.Candidates) == 0 {
		return nil, errors.New("Gemini returned no answer")
	}
	c := r.Candidates[0]
	var text strings.Builder
	for _, p := range c.Content.Parts {
		text.WriteString(p.Text)
	}
	if c.FinishReason != "" && c.FinishReason != "STOP" {
		return nil, fmt.Errorf("Gemini stopped early: %s", c.FinishReason)
	}
	return []byte(text.String()), nil
}

var flashName = regexp.MustCompile(`^models/gemini-(\d+)(?:\.(\d+))?-flash$`)

// newestFlash — самая новая модель вида gemini-X.Y-flash из списка, доступного ключу (generateContent).
func (g *Gemini) newestFlash(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL+"/v1beta/models?pageSize=1000", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("x-goog-api-key", g.apiKey)
	resp, err := g.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gemini models list: HTTP %d", resp.StatusCode)
	}
	var list struct {
		Models []struct {
			Name    string   `json:"name"`
			Methods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&list); err != nil {
		return "", err
	}
	type cand struct {
		name         string
		major, minor int
	}
	var cs []cand
	for _, m := range list.Models {
		p := flashName.FindStringSubmatch(m.Name)
		if p == nil || !contains(m.Methods, "generateContent") {
			continue
		}
		major, _ := strconv.Atoi(p[1])
		minor, _ := strconv.Atoi(p[2])
		cs = append(cs, cand{strings.TrimPrefix(m.Name, "models/"), major, minor})
	}
	if len(cs) == 0 {
		return "", errors.New("no gemini-*-flash model is available for this key")
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].major != cs[j].major {
			return cs[i].major > cs[j].major
		}
		return cs[i].minor > cs[j].minor
	})
	return cs[0].name, nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
