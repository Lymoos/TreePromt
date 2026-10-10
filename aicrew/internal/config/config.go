// Package config — настройки aicrew-host (без секретов: они в internal/secrets).
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	ServerURL    string   `json:"server_url"`
	HostName     string   `json:"host_name"`
	DeviceID     string   `json:"device_id"`
	Capabilities []string `json:"capabilities"` // flutter, go, web…
	Environments []string `json:"environments"` // container, windows_user
	// Образ контейнера исполнителя и проверок.
	AgentImage  string `json:"agent_image"`
	ClaudeModel string `json:"claude_model"`
	// Ревьюеры (7.4): Logic QA, пока у сервера нет Gemini, и Tech Lead.
	ReviewModel   string `json:"review_model"`
	TechLeadModel string `json:"tech_lead_model"`
	// Сколько задач одновременно (решение 7.1: одна).
	Concurrency int `json:"concurrency"`
}

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "AiCrew"), nil
}

func Path() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

func Defaults() Config {
	host, _ := os.Hostname()
	return Config{
		HostName:      host,
		Capabilities:  []string{"flutter", "go", "web"},
		Environments:  []string{"container"},
		AgentImage:    "aicrew-agent-flutter:local",
		ClaudeModel:   "sonnet",
		ReviewModel:   "haiku",
		TechLeadModel: "sonnet",
		Concurrency:   1,
	}
}

func Load(path string) (Config, error) {
	c := Defaults()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if c.Concurrency < 1 {
		c.Concurrency = 1
	}
	return c, nil
}

func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(path, b, 0o600)
}
