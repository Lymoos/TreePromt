package secretscan

import (
	"strings"
	"testing"
)

// Тестовые «секреты» собираются из частей, чтобы сам файл теста не выглядел как утечка.
func j(parts ...string) string { return strings.Join(parts, "") }

func TestFormats(t *testing.T) {
	s := New(nil)
	cases := map[string]string{
		"anthropic_key":   "key := \"" + j("sk-ant-", "oat01-", "Abcdefghijklmnopqrstuvwx_12") + "\"",
		"openai_key":      "OPENAI=" + j("sk-", "proj-", "A1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6Q7"),
		"google_api_key":  "const k = '" + j("AIza", "SyA1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6Q") + "'",
		"github_token":    j("ghp_", "A1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6Q7r8"),
		"aws_access_key":  "aws " + j("AKIA", "ABCDEFGHIJKLMNOP"),
		"private_key":     j("-----BEGIN ", "RSA PRIVATE KEY-----"),
		"jwt":             j("eyJ", "hbGciOiJIUzI1NiJ9", ".eyJ", "zdWIiOiIxMjM0NTY3ODkwIn0", ".dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"),
		"url_password":    "DATABASE_URL=postgres://app:" + j("Kq9", "!x2Lm") + "@db:5432/prod",
		"assigned_secret": `client_secret = "` + j("Zp4", "Qr8Wm2Xv7Lk3Nt9Bh6") + `"`,
		"slack_token":     j("xoxb-", "1234567890-abcdefghij"),
	}
	for kind, text := range cases {
		f := s.Scan(text)
		if len(f) == 0 || f[0].Kind != kind {
			t.Errorf("%s: got %+v", kind, f)
		}
	}
}

func TestPlaceholdersAndWordsAreNotSecrets(t *testing.T) {
	s := New(nil)
	for _, text := range []string{
		`password = "changeme_please_now"`,
		`API_KEY=${API_KEY_FROM_ENV}`,
		`token: <your-token-here-xxxxxxxx>`,
		`postgres://postgres:pt_test_only@127.0.0.1:55432/devlocal`,
		`password = passwordpasswordpassword`, // низкая энтропия
		`final tokenizer = Tokenizer(); // token = tokenizer.next()`,
		`secret: process.env.SECRET_VALUE_FROM_CI`,
	} {
		if f := s.Scan(text); len(f) != 0 {
			t.Errorf("false positive %q: %+v", text, f)
		}
	}
}

func TestExactHostSecretsAndRedaction(t *testing.T) {
	pw := j("Mo", "nday-", "Coffee-42")
	s := New(map[string]string{pw: "worker_password", "short": "x"})
	text := "logged in with " + pw + " twice: " + pw
	if f := s.Scan(text); len(f) != 2 || f[0].Kind != "worker_password" {
		t.Fatalf("%+v", f)
	}
	red := s.Redact(text)
	if strings.Contains(red, pw) || strings.Count(red, "[REDACTED:worker_password]") != 2 {
		t.Fatal(red)
	}
	if f := s.Scan("short short"); len(f) != 0 {
		t.Fatal("too short values are not tracked")
	}
}

func TestScanDiffReportsFileAndLine(t *testing.T) {
	diff := "diff --git a/lib/a.dart b/lib/a.dart\n+++ b/lib/a.dart\n@@ -0,0 +10,3 @@\n+void main() {}\n+const key = '" +
		j("AIza", "SyA1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6Q") + "';\n+// ok\n"
	f := New(nil).ScanDiff(diff)
	if len(f) != 1 || f[0].File != "lib/a.dart" || f[0].Line != 11 || f[0].Kind != "google_api_key" {
		t.Fatalf("%+v", f)
	}
}
