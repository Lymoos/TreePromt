package runas

import (
	"context"
	"os"
	"strings"
	"testing"

	"prompttree/aicrew/internal/runner"
)

func TestCommandLineQuoting(t *testing.T) {
	got := CommandLine(`C:\Program Files\claude.exe`, []string{"-p", "--mcp-config", `{"mcpServers":{"a":{"args":["/c","x y"]}}}`, `C:\dir\`, ""})
	want := `"C:\Program Files\claude.exe" -p --mcp-config "{\"mcpServers\":{\"a\":{\"args\":[\"/c\",\"x y\"]}}}" C:\dir\ ""`
	if got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	if q := quote(`a b\`); q != `"a b\\"` {
		t.Fatal(q)
	}
}

func TestEnvHasWorkerProfileAndNoOwnerPaths(t *testing.T) {
	t.Setenv("USERPROFILE", `C:\Users\Owner`)
	t.Setenv("PATH", `C:\Windows\System32;C:\Users\Owner\go\bin;C:\Program Files\Git\cmd`)
	t.Setenv("SystemDrive", "C:")
	env := strings.Join(Env("ai_worker", []string{"CLAUDE_CODE_OAUTH_TOKEN=secret"}), "\n")
	for _, want := range []string{`USERPROFILE=C:\Users\ai_worker`, `APPDATA=C:\Users\ai_worker\AppData\Roaming`, "CLAUDE_CODE_OAUTH_TOKEN=secret", `C:\Program Files\Git\cmd`} {
		if !strings.Contains(env, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(env, `C:\Users\Owner`) {
		t.Fatal("owner's profile must not leak into ai_worker's environment")
	}
}

func TestOnlyAllowedProgramsRun(t *testing.T) {
	r := Runner{User: "ai_worker", Allowed: map[string]bool{`c:\tools\selene.exe`: true}, Password: func() (string, error) { return "", nil }}
	for _, exe := range []string{"cmd.exe", `C:\Windows\System32\cmd.exe`, `C:\tools\bash.exe`} {
		if _, err := r.Run(context.Background(), runner.Command{Exe: exe}); err == nil || !strings.Contains(err.Error(), "not allowed") {
			if os.PathSeparator == '\\' {
				t.Errorf("%s: %v", exe, err)
			}
		}
	}
}
