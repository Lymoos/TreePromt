//go:build !windows

package secrets

// На других ОС (CI, Linux-хост в будущем) — файл с правами 0600. Для продакшен-хоста
// на Linux сюда нужно подключить keyring; домашний хост по решению 7.1 — Windows.
func protect(data []byte) ([]byte, error)   { return data, nil }
func unprotect(data []byte) ([]byte, error) { return data, nil }
