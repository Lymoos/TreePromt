// Package config reads server settings from environment variables.
package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL       string
	JWTSecret         []byte
	ListenAddr        string
	AllowRegistration bool     // если false, регистрация возможна только пока нет ни одного пользователя
	AllowedOrigins    []string // для CORS и WebSocket (веб-клиент)
	TrustProxy        bool     // брать IP клиента из X-Forwarded-For (за Caddy)
}

func FromEnv() (Config, error) {
	c := Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		JWTSecret:         []byte(os.Getenv("JWT_SECRET")),
		ListenAddr:        envOr("LISTEN_ADDR", ":8080"),
		AllowRegistration: os.Getenv("ALLOW_REGISTRATION") == "true",
		TrustProxy:        os.Getenv("TRUST_PROXY") == "true",
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
