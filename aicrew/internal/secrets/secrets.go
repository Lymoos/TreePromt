// Package secrets хранит токены aicrew-host (refresh-токен сервера, токен подписки Claude Code)
// зашифрованными: на Windows — DPAPI текущего пользователя (решение 7.1, п. 2). Ни в репозитории,
// ни в переменных окружения хоста секретов нет.
package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
)

const (
	RefreshToken = "server_refresh_token"
	ClaudeToken  = "claude_code_oauth_token"
)

var ErrMissing = errors.New("secret is not set")

var nameRe = regexp.MustCompile(`^[a-z0-9_]+$`)

type Store struct {
	Dir string
}

func (s Store) path(name string) (string, error) {
	if !nameRe.MatchString(name) {
		return "", errors.New("invalid secret name")
	}
	return filepath.Join(s.Dir, name+".bin"), nil
}

func (s Store) Set(name, value string) error {
	p, err := s.path(name)
	if err != nil {
		return err
	}
	enc, err := protect([]byte(value))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, enc, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (s Store) Get(name string) (string, error) {
	p, err := s.path(name)
	if err != nil {
		return "", err
	}
	enc, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrMissing
	}
	if err != nil {
		return "", err
	}
	dec, err := unprotect(enc)
	if err != nil {
		return "", err
	}
	return string(dec), nil
}

func (s Store) Delete(name string) error {
	p, err := s.path(name)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
