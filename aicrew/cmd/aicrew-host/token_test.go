package main

import "testing"

func TestExtractToken(t *testing.T) {
	want := "sk-ant-oat01-AbC_d-123xyzQAA"
	cases := map[string]string{
		"одна строка":           want + "\r\n",
		"перенос":               "sk-ant-oat01-AbC_d-\r\n123xyzQAA\r\n",
		"с текстом вокруг":      "Your OAuth token (valid for 1 year):\r\n\x1b[33m" + want + "\x1b[0m\r\n\r\nStore this token securely.\r\n",
		"перенос и текст после": "  sk-ant-oat01-AbC_d-123\r\nxyzQAA\r\nStore this token securely.",
		"следы вставки":         "\x1b[200~" + want + "\x1b[201~",
	}
	for name, in := range cases {
		if got := extractToken(in); got != want {
			t.Errorf("%s: got %q", name, got)
		}
	}
}
