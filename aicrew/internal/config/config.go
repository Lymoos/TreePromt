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
	// 7.5: Manager, реестры, локальные модели (Ollama; модели лежат на D:, настройка самой Ollama).
	ManagerModel       string `json:"manager_model"`
	SkillsDir          string `json:"skills_dir"`           // каталог скиллов; пусто — рядом с программой
	MCPRegistry        string `json:"mcp_registry"`         // реестр MCP; пусто — рядом с программой
	OllamaURL          string `json:"ollama_url"`           // для хоста
	OllamaURLContainer string `json:"ollama_url_container"` // для контейнера исполнителя
	JuniorModel        string `json:"junior_model"`
	// Окно контекста Junior: системный промпт Claude Code ~18k токенов, а Ollama по умолчанию даёт 4k.
	// Служба создаёт в Ollama вариант модели с этим num_ctx (см. ensureContextModel).
	JuniorContext int `json:"junior_context"`
	// Окружение windows_user (7.5b): пользователь Windows без прав администратора и его рабочая папка.
	WorkerUser   string            `json:"worker_user"`   // ai_worker; пусто — окружение выключено
	WorkRoot     string            `json:"work_root"`     // D:\aicrew-work — репозитории Roblox лежат здесь
	WorkerClaude string            `json:"worker_claude"` // claude.exe в профиле ai_worker
	WorkerTools  map[string]string `json:"worker_tools"`  // инструменты проверок: selene, stylua
	RobloxStudio bool              `json:"roblox_studio"` // у ai_worker установлен Roblox Studio с включённым MCP
	VisionModel  string            `json:"vision_model"`
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
		HostName:           host,
		Capabilities:       []string{"flutter", "go", "web"},
		Environments:       []string{"container"},
		AgentImage:         "aicrew-agent-flutter:local",
		ClaudeModel:        "sonnet",
		ReviewModel:        "haiku",
		TechLeadModel:      "sonnet",
		ManagerModel:       "haiku",
		OllamaURL:          "http://127.0.0.1:11434",
		OllamaURLContainer: "http://host.docker.internal:11434",
		JuniorModel:        "qwen3-coder:30b", // qwen2.5-coder писал вызовы инструментов текстом
		JuniorContext:      32768,
		VisionModel:        "qwen2.5vl:7b",
		WorkRoot:           `D:\aicrew-work`,
		WorkerClaude:       `C:\Users\ai_worker\.local\bin\claude.exe`,
		WorkerTools: map[string]string{
			"selene": `C:\Users\ai_worker\.rokit\bin\selene.exe`,
			"stylua": `C:\Users\ai_worker\.rokit\bin\stylua.exe`,
		},
		Concurrency: 1,
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
