// Package runas — окружение windows_user (решение 7.1, п. 1; docs/stage7-5-manager-skills-mcp.md, раздел 5):
// команды задачи выполняются от отдельного пользователя Windows ai_worker — без прав администратора
// и без доступа к папкам владельца. Реализует runner.Runner, поэтому исполнитель и проверки
// работают поверх него так же, как поверх docker.
package runas

import (
	"os"
	"path/filepath"
	"strings"

	"prompttree/aicrew/internal/runner"
)

// Runner запускает программы от имени User. Password выдаётся только на время запуска (DPAPI).
type Runner struct {
	User     string
	Password func() (string, error)
	// WorkRoot — общая рабочая папка (D:\aicrew-work): ai_worker имеет права только на неё.
	WorkRoot string
	// Allowed — какие программы можно запускать (полные пути); остальное запрещено (ТЗ п. 12: неизвестное — DENY).
	Allowed map[string]bool
}

var _ runner.Runner = Runner{}

// Profile — каталоги профиля ai_worker: окружение процесса собирается из них, а не из профиля владельца.
func Profile(user string) map[string]string {
	home := filepath.Join(os.Getenv("SystemDrive")+`\`, "Users", user)
	return map[string]string{
		"USERNAME":     user,
		"USERPROFILE":  home,
		"HOMEDRIVE":    os.Getenv("SystemDrive"),
		"HOMEPATH":     strings.TrimPrefix(home, os.Getenv("SystemDrive")),
		"APPDATA":      filepath.Join(home, "AppData", "Roaming"),
		"LOCALAPPDATA": filepath.Join(home, "AppData", "Local"),
		"TEMP":         filepath.Join(home, "AppData", "Local", "Temp"),
		"TMP":          filepath.Join(home, "AppData", "Local", "Temp"),
	}
}

// systemVars — переменные, которые можно взять из окружения службы: они не указывают на профиль владельца.
var systemVars = []string{"SystemRoot", "SystemDrive", "windir", "ComSpec", "PATHEXT", "ProgramFiles", "ProgramFiles(x86)",
	"ProgramW6432", "ProgramData", "CommonProgramFiles", "CommonProgramFiles(x86)", "CommonProgramW6432",
	"NUMBER_OF_PROCESSORS", "PROCESSOR_ARCHITECTURE", "OS", "PUBLIC", "ALLUSERSPROFILE"}

// Env — окружение процесса ai_worker: системные переменные, профиль ai_worker, PATH без каталогов
// владельца и дополнительные переменные задачи (токен — здесь, не в командной строке).
func Env(user string, extra []string) []string {
	owner := os.Getenv("USERPROFILE")
	var env []string
	for _, k := range systemVars {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	prof := Profile(user)
	for k, v := range prof {
		env = append(env, k+"="+v)
	}
	var path []string
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p == "" || (owner != "" && strings.HasPrefix(strings.ToLower(p), strings.ToLower(owner))) {
			continue // каталоги из профиля владельца ai_worker не нужны и не доступны
		}
		path = append(path, p)
	}
	path = append(path, filepath.Join(prof["LOCALAPPDATA"], "Programs"), filepath.Join(prof["USERPROFILE"], ".local", "bin"),
		filepath.Join(prof["USERPROFILE"], ".rokit", "bin"), filepath.Join(prof["APPDATA"], "npm"))
	env = append(env, "PATH="+strings.Join(path, string(os.PathListSeparator)))
	return append(env, extra...)
}

// CommandLine — argv в командную строку Windows по правилам CommandLineToArgvW (без cmd.exe).
func CommandLine(exe string, args []string) string {
	parts := []string{quote(exe)}
	for _, a := range args {
		parts = append(parts, quote(a))
	}
	return strings.Join(parts, " ")
}

func quote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n\v\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for _, c := range s {
		switch c {
		case '\\':
			slashes++
			continue
		case '"':
			b.WriteString(strings.Repeat(`\`, slashes*2+1))
			b.WriteRune(c)
		default:
			b.WriteString(strings.Repeat(`\`, slashes))
			b.WriteRune(c)
		}
		slashes = 0
	}
	b.WriteString(strings.Repeat(`\`, slashes*2))
	b.WriteByte('"')
	return b.String()
}
