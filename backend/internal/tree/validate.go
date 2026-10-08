package tree

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxNameRunes        = 200
	MaxDescriptionRunes = 10_000
	MaxSortKeyLen       = 128
	MaxContentBytes     = 2 << 20 // 2 МиБ текста на заметку
	MaxOpsPerPush       = 500
)

// rejectErr — отказ в операции по бизнес-правилу (а не сбой сервера).
type rejectErr struct{ OpError }

func (e *rejectErr) Error() string { return e.Code + ": " + e.Message }

func reject(code, format string, args ...any) error {
	return &rejectErr{OpError{Code: code, Message: fmt.Sprintf(format, args...)}}
}

const (
	CodeInvalidPayload = "invalid_payload"
	CodeNotFound       = "not_found"
	CodeForbidden      = "forbidden"
	CodeAlreadyExists  = "already_exists"
	CodeInvalidParent  = "invalid_parent"
	CodeCycle          = "cycle"
	CodeKindChange     = "kind_change_not_allowed"
	CodeUnknownOp      = "unknown_operation"
	CodeTooLarge       = "content_too_large"
	CodeUnsupported    = "unsupported"
)

func decode[T any](raw json.RawMessage) (T, error) {
	var v T
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, reject(CodeInvalidPayload, "payload: %v", err)
	}
	return v, nil
}

func validName(s string) error {
	if !utf8.ValidString(s) {
		return reject(CodeInvalidPayload, "name is not valid UTF-8")
	}
	if strings.TrimSpace(s) == "" {
		return reject(CodeInvalidPayload, "name is empty")
	}
	if utf8.RuneCountInString(s) > MaxNameRunes {
		return reject(CodeInvalidPayload, "name is longer than %d characters", MaxNameRunes)
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return reject(CodeInvalidPayload, "name contains control characters")
		}
	}
	return nil
}

func validDescription(s string) error {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > MaxDescriptionRunes {
		return reject(CodeInvalidPayload, "description is invalid or longer than %d characters", MaxDescriptionRunes)
	}
	return nil
}

// sort_key — дробный ключ порядка, который генерирует клиент; сервер только проверяет формат.
func validSortKey(s string) error {
	if s == "" || len(s) > MaxSortKeyLen {
		return reject(CodeInvalidPayload, "sort_key must be 1..%d characters", MaxSortKeyLen)
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7e {
			return reject(CodeInvalidPayload, "sort_key must be printable ASCII")
		}
	}
	return nil
}

func validKind(k string) error {
	switch k {
	case KindFolder, KindRawNote, KindAITask:
		return nil
	}
	return reject(CodeInvalidPayload, "unknown kind %q", k)
}

func validContent(s string) error {
	if len(s) > MaxContentBytes {
		return reject(CodeTooLarge, "content is larger than %d bytes", MaxContentBytes)
	}
	if !utf8.ValidString(s) {
		return reject(CodeInvalidPayload, "content is not valid UTF-8")
	}
	return nil
}

// sanitizePayload replaces note text with its hash and length before the payload
// is stored in the operation log: the log is for idempotency and audit, not content.
func sanitizePayload(raw json.RawMessage) json.RawMessage {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return json.RawMessage(`{}`)
	}
	for _, k := range []string{"content", "raw_content"} {
		if s, ok := m[k].(string); ok {
			sum := sha256.Sum256([]byte(s))
			m[k] = map[string]any{"sha256": hex.EncodeToString(sum[:]), "bytes": len(s)}
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return out
}
