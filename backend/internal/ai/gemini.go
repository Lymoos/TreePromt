package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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

func (g *Gemini) Name() string { return g.model }

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

// Generate вызывает generateContent с JSON-схемой ответа. Ключ — только в заголовке,
// в URL и тексты ошибок он не попадает.
func (g *Gemini) Generate(ctx context.Context, system, user string) (tree.Generated, error) {
	body, _ := json.Marshal(map[string]any{
		"systemInstruction": map[string]any{"parts": []any{map[string]any{"text": system}}},
		"contents":          []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": user}}}},
		"generationConfig": map[string]any{
			"temperature":      0.2,
			"responseMimeType": "application/json",
			"responseSchema":   responseSchema,
		},
	})
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", g.baseURL, g.model)

	var lastErr error
	for attempt := 0; attempt <= g.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return tree.Generated{}, ctx.Err()
			case <-time.After(g.backoff * time.Duration(1<<(attempt-1))):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return tree.Generated{}, err
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
			return tree.Generated{}, ErrModelAuth
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("Gemini is busy (HTTP %d)", resp.StatusCode)
			continue
		case resp.StatusCode != http.StatusOK:
			return tree.Generated{}, fmt.Errorf("Gemini returned HTTP %d", resp.StatusCode)
		}
		return parseGemini(data)
	}
	return tree.Generated{}, lastErr
}

func parseGemini(data []byte) (tree.Generated, error) {
	var r geminiResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return tree.Generated{}, fmt.Errorf("unreadable Gemini response: %w", err)
	}
	if r.PromptFeedback.BlockReason != "" {
		return tree.Generated{}, fmt.Errorf("Gemini blocked the request: %s", r.PromptFeedback.BlockReason)
	}
	if len(r.Candidates) == 0 {
		return tree.Generated{}, errors.New("Gemini returned no answer")
	}
	c := r.Candidates[0]
	var text strings.Builder
	for _, p := range c.Content.Parts {
		text.WriteString(p.Text)
	}
	if c.FinishReason != "" && c.FinishReason != "STOP" {
		return tree.Generated{}, fmt.Errorf("Gemini stopped early: %s", c.FinishReason)
	}
	var g tree.Generated
	if err := json.Unmarshal([]byte(text.String()), &g); err != nil {
		return tree.Generated{}, fmt.Errorf("Gemini answer is not valid JSON: %w", err)
	}
	return g, nil
}
