// Package api — клиент протокола aicrew-host ↔ сервер (docs/stage7-2-state-machine.md).
// Хост входит как устройство пользователя; refresh-токен хранится зашифрованным.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	ErrAuthRequired = errors.New("login required: run `aicrew-host login`")
	ErrNotOwner     = errors.New("task is no longer leased by this worker")
)

// TokenStore — где лежит refresh-токен (secrets.Store на хосте, память в тестах).
type TokenStore interface {
	Get(name string) (string, error)
	Set(name, value string) error
}

const refreshKey = "server_refresh_token"

type Client struct {
	Base   string
	HTTP   *http.Client
	Tokens TokenStore

	mu     sync.Mutex
	access string
}

func New(base string, tokens TokenStore) *Client {
	return &Client{Base: strings.TrimRight(base, "/"), HTTP: &http.Client{Timeout: 60 * time.Second}, Tokens: tokens}
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("HTTP %d %s: %s", e.Status, e.Code, e.Message) }

func (c *Client) do(ctx context.Context, method, path string, body, out any, auth bool) (int, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+"/api/v1"+path, rd)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth {
		c.mu.Lock()
		req.Header.Set("Authorization", "Bearer "+c.access)
		c.mu.Unlock()
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 400 {
		var e struct {
			Error struct{ Code, Message string } `json:"error"`
		}
		_ = json.Unmarshal(data, &e)
		return resp.StatusCode, &APIError{Status: resp.StatusCode, Code: e.Error.Code, Message: e.Error.Message}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

// authed выполняет запрос и один раз обновляет access-токен при 401.
func (c *Client) authed(ctx context.Context, method, path string, body, out any) (int, error) {
	code, err := c.do(ctx, method, path, body, out, true)
	var ae *APIError
	if errors.As(err, &ae) && ae.Status == http.StatusUnauthorized {
		if err := c.refresh(ctx); err != nil {
			return 0, err
		}
		return c.do(ctx, method, path, body, out, true)
	}
	return code, err
}

type tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (c *Client) store(t tokens) error {
	c.mu.Lock()
	c.access = t.AccessToken
	c.mu.Unlock()
	return c.Tokens.Set(refreshKey, t.RefreshToken)
}

func (c *Client) Login(ctx context.Context, email, password, deviceID, hostName string) error {
	var t tokens
	_, err := c.do(ctx, http.MethodPost, "/auth/login", map[string]any{
		"email": email, "password": password,
		"device": map[string]string{"id": deviceID, "name": hostName, "platform": "aicrew-host"},
	}, &t, false)
	if err != nil {
		return err
	}
	return c.store(t)
}

func (c *Client) refresh(ctx context.Context) error {
	rt, err := c.Tokens.Get(refreshKey)
	if err != nil || rt == "" {
		return ErrAuthRequired
	}
	var t tokens
	_, err = c.do(ctx, http.MethodPost, "/auth/refresh", map[string]string{"refresh_token": rt}, &t, false)
	var ae *APIError
	if errors.As(err, &ae) && ae.Status == http.StatusUnauthorized {
		return ErrAuthRequired
	}
	if err != nil {
		return err
	}
	return c.store(t)
}

// AccessToken — для WebSocket (первое сообщение auth).
func (c *Client) AccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	tok := c.access
	c.mu.Unlock()
	if tok != "" {
		return tok, nil
	}
	if err := c.refresh(ctx); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.access, nil
}

// ── протокол ──

type WorkerSpec struct {
	Kind  string `json:"kind"`
	Model string `json:"model"`
}

type Host struct {
	ID      string            `json:"id"`
	Workers map[string]string `json:"workers"`
}

func (c *Client) RegisterHost(ctx context.Context, name, os string, caps, envs []string, workers []WorkerSpec) (Host, error) {
	var h Host
	_, err := c.authed(ctx, http.MethodPost, "/exec/hosts", map[string]any{
		"name": name, "os": os, "capabilities": caps, "environments": envs, "workers": workers}, &h)
	return h, err
}

type Repository struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	LocalPath           string          `json:"local_path"`
	DefaultBranch       string          `json:"default_branch"`
	VerificationProfile json.RawMessage `json:"verification_profile"`
}

type PreviousAttempt struct {
	Attempt      int             `json:"attempt"`
	Status       string          `json:"status"`
	FailureClass *string         `json:"failure_class"`
	FailureCode  *string         `json:"failure_code"`
	Facts        json.RawMessage `json:"facts"`
}

type Task struct {
	ID               string            `json:"id"`
	Attempt          int               `json:"attempt"`
	Title            string            `json:"title"`
	Prompt           string            `json:"prompt"`
	ExecutionEnv     string            `json:"execution_env"`
	AgentRuntimeSec  int               `json:"agent_runtime_sec"`
	VerifyRuntimeSec int               `json:"verification_runtime_sec"`
	Repository       Repository        `json:"repository"`
	PreviousAttempts []PreviousAttempt `json:"previous_attempts"`
}

// Claim — nil, если работы нет.
func (c *Client) Claim(ctx context.Context, workerID string) (*Task, error) {
	var t Task
	code, err := c.authed(ctx, http.MethodPost, "/exec/claim", map[string]string{"worker_id": workerID}, &t)
	if err != nil || code == http.StatusNoContent {
		return nil, err
	}
	return &t, nil
}

// Heartbeat продлевает lease; true — пользователь просит отмену.
func (c *Client) Heartbeat(ctx context.Context, taskID, workerID string, attempt int) (bool, error) {
	var out struct {
		Cancel bool `json:"cancel_requested"`
	}
	_, err := c.authed(ctx, http.MethodPost, "/exec/tasks/"+taskID+"/heartbeat", map[string]any{"worker_id": workerID, "attempt": attempt}, &out)
	return out.Cancel, ownerErr(err)
}

type Facts struct {
	BaseCommit    string `json:"base_commit,omitempty"`
	ResultCommit  string `json:"result_commit,omitempty"`
	FailureCode   string `json:"failure_code,omitempty"`
	FailureClass  string `json:"failure_class,omitempty"`
	RetryAfterSec int    `json:"retry_after_sec,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Details       any    `json:"details,omitempty"`
}

func (c *Client) Transition(ctx context.Context, taskID, workerID string, attempt int, to string, f Facts) (string, error) {
	var out struct {
		Status string `json:"status"`
	}
	_, err := c.authed(ctx, http.MethodPost, "/exec/tasks/"+taskID+"/transition",
		map[string]any{"worker_id": workerID, "attempt": attempt, "to": to, "facts": f}, &out)
	return out.Status, ownerErr(err)
}

func (c *Client) KeepWorktrees(ctx context.Context) (map[string]bool, error) {
	var out struct {
		IDs []string `json:"task_ids"`
	}
	_, err := c.authed(ctx, http.MethodGet, "/exec/keep-worktrees", nil, &out)
	m := map[string]bool{}
	for _, id := range out.IDs {
		m[id] = true
	}
	return m, err
}

func ownerErr(err error) error {
	var ae *APIError
	if errors.As(err, &ae) && ae.Code == "not_owner" {
		return ErrNotOwner
	}
	return err
}

// AddRepository регистрирует локальный репозиторий этого хоста для проекта PromptTree.
func (c *Client) AddRepository(ctx context.Context, projectID, hostID, name, localPath, branch string, profile json.RawMessage) (Repository, error) {
	var r Repository
	if len(profile) == 0 {
		profile = json.RawMessage(`{}`)
	}
	_, err := c.authed(ctx, http.MethodPost, "/exec/repositories", map[string]any{
		"project_id": projectID, "host_id": hostID, "name": name, "local_path": localPath,
		"default_branch": branch, "verification_profile": profile}, &r)
	return r, err
}

func (c *Client) ListRepositories(ctx context.Context) ([]struct {
	Repository
	HostID string `json:"host_id"`
}, error) {
	var out struct {
		Repos []struct {
			Repository
			HostID string `json:"host_id"`
		} `json:"repositories"`
	}
	_, err := c.authed(ctx, http.MethodGet, "/exec/repositories", nil, &out)
	return out.Repos, err
}
