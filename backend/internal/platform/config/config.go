// Package config reads server settings from environment variables.
package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL       string
	JWTSecret         []byte
	ListenAddr        string
	AllowRegistration bool     // если false, регистрация возможна только пока нет ни одного пользователя
	AllowedOrigins    []string // для CORS и WebSocket (веб-клиент)
	TrustProxy        bool     // брать IP клиента из X-Forwarded-For (за Caddy)

	// Этап 6. Без ключа структурирование выключено, остальное работает.
	GeminiAPIKey string
	GeminiModel  string
	AIDailyLimit int
	// Только для тестов: адрес имитации API Gemini.
	GeminiBaseURL string
}

func FromEnv() (Config, error) {
	c := Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		JWTSecret:         []byte(os.Getenv("JWT_SECRET")),
		ListenAddr:        envOr("LISTEN_ADDR", ":8080"),
		AllowRegistration: os.Getenv("ALLOW_REGISTRATION") == "true",
		TrustProxy:        os.Getenv("TRUST_PROXY") == "true",
		GeminiAPIKey:      strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		GeminiModel:       envOr("GEMINI_MODEL", "gemini-2.5-flash"),
		AIDailyLimit:      200,
		GeminiBaseURL:     os.Getenv("GEMINI_BASE_URL"),
	}
	if v := os.Getenv("AI_DAILY_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return c, errors.New("AI_DAILY_LIMIT must be a non-negative integer")
		}
		c.AIDailyLimit = n
	}
	for _, o := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, o)
		}
	}
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 32 {
		return c, errors.New("JWT_SECRET must be at least 32 bytes")
	}
	return c, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
