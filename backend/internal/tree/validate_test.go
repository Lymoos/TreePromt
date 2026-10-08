package tree

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidName(t *testing.T) {
	for _, ok := range []string{"Заметка", "AI 🤖 задача", "a", strings.Repeat("я", MaxNameRunes)} {
		if err := validName(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "   ", "a\nb", "tab\there", strings.Repeat("я", MaxNameRunes+1), "\xff"} {
		if err := validName(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestValidSortKey(t *testing.T) {
	if err := validSortKey("a0V"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "a b", "ключ", strings.Repeat("a", MaxSortKeyLen+1)} {
		if err := validSortKey(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestValidContent(t *testing.T) {
	if err := validContent(strings.Repeat("Привет 👋 ", 50_000)); err != nil {
		t.Fatal(err)
	}
	if err := validContent(strings.Repeat("x", MaxContentBytes+1)); err == nil {
		t.Fatal("oversized content accepted")
	}
	if err := validContent("\xff\xfe"); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	_, err := decode[struct {
		Name string `json:"name"`
	}](json.RawMessage(`{"name":"x","extra":1}`))
	if err == nil {
		t.Fatal("unknown field accepted")
	}
}

func TestSanitizePayloadHidesText(t *testing.T) {
	out := sanitizePayload(json.RawMessage(`{"content":"секретная мысль","name":"Имя"}`))
	s := string(out)
	if strings.Contains(s, "секретная") {
		t.Fatalf("text leaked into operation log: %s", s)
	}
	if !strings.Contains(s, "sha256") || !strings.Contains(s, "Имя") {
		t.Fatalf("unexpected payload: %s", s)
	}
}
