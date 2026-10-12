// aicrew-host — служба Execution Host AiCrew на домашнем ПК (docs/stage7-2-state-machine.md).
//
//	aicrew-host login              — войти на сервер PromptTree как устройство-хост
//	aicrew-host set-claude-token   — сохранить токен `claude setup-token` (зашифрован DPAPI)
//	aicrew-host add-repo …         — привязать локальный репозиторий к проекту PromptTree
//	aicrew-host run                — брать и выполнять задачи
//	aicrew-host promote -path …    — перенести integration-ветку в main (только человек)
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"golang.org/x/term"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/audit"
	"prompttree/aicrew/internal/config"
	"prompttree/aicrew/internal/gitx"
	"prompttree/aicrew/internal/host"
	"prompttree/aicrew/internal/manager"
	"prompttree/aicrew/internal/mcp"
	"prompttree/aicrew/internal/review"
	"prompttree/aicrew/internal/runas"
	"prompttree/aicrew/internal/runner"
	"prompttree/aicrew/internal/secrets"
	"prompttree/aicrew/internal/secretscan"
	"prompttree/aicrew/internal/skills"
	"prompttree/aicrew/internal/verify"
	"prompttree/aicrew/internal/vision"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	if err := dispatch(os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`aicrew-host — исполнитель задач AiCrew

  login              войти на сервер PromptTree (email и пароль владельца)
  set-claude-token   сохранить токен из "claude setup-token" (ввод скрыт; -clipboard — взять из буфера обмена)
  check-claude-token проверить сохранённый токен (сам токен не выводится)
  add-repo           привязать локальный репозиторий: -project <id> -path <папка> [-name] [-branch] [-profile файл.json]
  run                брать и выполнять задачи, сливать проверенные в integration-ветку
  set-worker         окружение windows_user: пароль ai_worker (-user, -roblox, -clipboard)
  set-junior         локальная модель Junior: -model qwen3-coder:30b -context 32768
  set-skills         скиллы репозитория: -path <папка> -required a,b -forbidden c
  set-profile        профиль проверки репозитория: -path <папка> -profile файл.json ({"template":"flutter"})
  promote            перенести integration в основную ветку после проверки: -path <папка>`)
}

type env struct {
	cfgPath string
	cfg     config.Config
	sec     secrets.Store
	client  *api.Client
}

func load() (*env, error) {
	p, err := config.Path()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(p)
	if err != nil {
		return nil, err
	}
	dir, _ := config.Dir()
	e := &env{cfgPath: p, cfg: cfg, sec: secrets.Store{Dir: filepath.Join(dir, "secrets")}}
	if cfg.ServerURL != "" {
		e.client = api.New(cfg.ServerURL, e.sec)
	}
	return e, nil
}

// stdin — один буферизованный читатель на весь процесс: несколько вопросов подряд из конвейера.
var stdin = bufio.NewReader(os.Stdin)

func readLine(prompt string) string {
	fmt.Print(prompt)
	s, _ := stdin.ReadString('\n')
	return strings.TrimSpace(s)
}

// readSecret читает секрет без эха; если ввод не терминал (скрипт), — одну строку из stdin.
func readSecret(prompt string) (string, error) {
	fmt.Print(prompt)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		s, err := stdin.ReadString('\n')
		fmt.Println()
		if err != nil && s == "" {
			return "", err
		}
		return strings.TrimSpace(s), nil
	}
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	return strings.TrimSpace(string(b)), err
}

func dispatch(cmd string, args []string) error {
	e, err := load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "login":
		fs := flag.NewFlagSet("login", flag.ExitOnError)
		server := fs.String("server", "", "адрес сервера PromptTree")
		_ = fs.Parse(args)
		if *server != "" {
			e.cfg.ServerURL = strings.TrimRight(*server, "/")
		}
		if e.cfg.ServerURL == "" {
			e.cfg.ServerURL = readLine("Адрес сервера PromptTree (https://…): ")
		}
		if e.cfg.DeviceID == "" {
			e.cfg.DeviceID = uuid.NewString()
		}
		email := readLine("Email: ")
		pass, err := readSecret("Пароль: ")
		if err != nil {
			return err
		}
		if err := api.New(e.cfg.ServerURL, e.sec).Login(ctx, email, pass, e.cfg.DeviceID, e.cfg.HostName); err != nil {
			return err
		}
		if err := config.Save(e.cfgPath, e.cfg); err != nil {
			return err
		}
		fmt.Println("Вход выполнен. Настройки:", e.cfgPath)
		return nil

	case "set-claude-token":
		fmt.Println(`Получите токен командой "claude setup-token" и вставьте его сюда. Он будет зашифрован ключом вашей учётной записи Windows.`)
		var raw string
		if len(args) > 0 && args[0] == "-clipboard" {
			// Буфер обмена читается напрямую, без конвейера PowerShell и без временных файлов.
			b, cerr := exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard -Raw").Output()
			raw, err = string(b), cerr
			switch {
			case err == nil && strings.TrimSpace(raw) == "":
				return errors.New("буфер обмена пуст: выделите токен и скопируйте его (в терминале — Ctrl+Shift+C или правый клик → Копировать; Ctrl+C там прерывает команду)")
			case err == nil && !strings.Contains(printable(raw), "sk-ant-"):
				return errors.New("в буфере обмена нет токена (sk-ant-…): скопируйте строку с токеном из вывода claude setup-token")
			}
		} else if term.IsTerminal(int(os.Stdin.Fd())) {
			raw, err = readSecret("Токен: ")
		} else {
			// Из конвейера (Get-Clipboard) — весь текст: токен мог перенестись на несколько строк.
			var b []byte
			b, err = io.ReadAll(stdin)
			raw = string(b)
		}
		if err != nil {
			return err
		}
		tok := extractToken(raw)
		if tok == "" {
			return errors.New("пустой токен")
		}
		if tok != raw {
			fmt.Printf("Убрано лишнее при вставке: %d символов.\n", len([]rune(raw))-len([]rune(tok)))
		}
		if err := e.sec.Set(secrets.ClaudeToken, tok); err != nil {
			return err
		}
		fmt.Println("Токен сохранён. Проверка: aicrew-host check-claude-token")
		return nil

	case "check-claude-token":
		// Сам токен не печатается: только формат, длина и ответ Claude из контейнера.
		raw, err := e.sec.Get(secrets.ClaudeToken)
		if err != nil {
			return fmt.Errorf("токен не найден: %w", err)
		}
		tok := cleanToken(raw)
		if tok != raw {
			fmt.Printf("В сохранённом токене было лишнее: %d символов — убрано.\n", len([]rune(raw))-len([]rune(tok)))
		}
		fmt.Printf("Длина: %d символов, формат setup-token (sk-ant-oat…): %v\n", len(tok), strings.HasPrefix(tok, "sk-ant-oat"))
		image := "aicrew-agent-flutter:local"
		if len(args) > 0 {
			image = args[0]
		}
		fmt.Println("Пробный запрос к Claude в контейнере…")
		cmd := exec.Command("docker", "run", "--rm", "-e", "CLAUDE_CODE_OAUTH_TOKEN", image,
			"claude", "-p", "Reply with exactly one word: ok")
		cmd.Env = append(os.Environ(), "CLAUDE_CODE_OAUTH_TOKEN="+tok)
		out, err := cmd.CombinedOutput()
		fmt.Println(strings.TrimSpace(string(out)))
		if err != nil {
			return fmt.Errorf("проверка не прошла: %w", err)
		}
		if tok != raw {
			if err := e.sec.Set(secrets.ClaudeToken, tok); err != nil {
				return err
			}
			fmt.Println("Очищенный токен сохранён.")
		}
		fmt.Println("Токен работает.")
		return nil

	case "add-repo":
		fs := flag.NewFlagSet("add-repo", flag.ExitOnError)
		project := fs.String("project", "", "проект PromptTree: имя или id")
		path := fs.String("path", "", "папка репозитория")
		name := fs.String("name", "", "название (по умолчанию — имя папки)")
		branch := fs.String("branch", "main", "основная ветка")
		profile := fs.String("profile", "", "JSON-файл профиля проверки")
		_ = fs.Parse(args)
		if e.client == nil {
			return api.ErrAuthRequired
		}
		abs, err := filepath.Abs(*path)
		if err != nil || *project == "" {
			return errors.New("нужны -project и -path")
		}
		if _, err := os.Stat(filepath.Join(abs, ".git")); err != nil {
			return fmt.Errorf("%s — не git-репозиторий", abs)
		}
		if *name == "" {
			*name = filepath.Base(abs)
		}
		var prof json.RawMessage
		if *profile != "" {
			b, err := os.ReadFile(*profile)
			if err != nil {
				return err
			}
			if _, err := verify.ParseProfile(b); err != nil {
				return err
			}
			prof = b
		}
		projectID, err := resolveProject(ctx, e.client, *project)
		if err != nil {
			return err
		}
		h, err := register(ctx, e)
		if err != nil {
			return err
		}
		r, err := e.client.AddRepository(ctx, projectID, h.ID, *name, abs, *branch, prof)
		if err != nil {
			return err
		}
		fmt.Println("Репозиторий привязан:", r.ID)
		return nil

	case "run":
		return runHost(ctx, e)

	case "set-junior":
		fs := flag.NewFlagSet("set-junior", flag.ExitOnError)
		model := fs.String("model", e.cfg.JuniorModel, "модель Ollama (уже скачанная: ollama pull …)")
		numCtx := fs.Int("context", 32768, "окно контекста, токенов")
		_ = fs.Parse(args)
		if !ollamaHas(ctx, e.cfg.OllamaURL, *model) {
			return fmt.Errorf("в Ollama нет модели %s — сначала: ollama pull %s", *model, *model)
		}
		e.cfg.JuniorModel, e.cfg.JuniorContext = *model, *numCtx
		if err := config.Save(e.cfgPath, e.cfg); err != nil {
			return err
		}
		fmt.Printf("Junior: %s, контекст %d. Перезапустите службу.\n", *model, *numCtx)
		return nil

	case "set-worker":
		// Окружение windows_user: пользователь ai_worker (создаёт владелец скриптом aicrew/setup).
		fs := flag.NewFlagSet("set-worker", flag.ExitOnError)
		user := fs.String("user", "ai_worker", "пользователь Windows для задач windows_user")
		clip := fs.Bool("clipboard", false, "взять пароль из буфера обмена")
		roblox := fs.Bool("roblox", false, "у пользователя установлен Roblox Studio с включённым MCP")
		_ = fs.Parse(args)
		var pass string
		if *clip {
			b, err := exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard -Raw").Output()
			if err != nil {
				return err
			}
			pass = strings.TrimRight(string(b), "\r\n")
		} else {
			var err error
			if pass, err = readSecret("Пароль " + *user + ": "); err != nil {
				return err
			}
		}
		if pass == "" {
			return errors.New("пустой пароль")
		}
		if err := e.sec.Set(secrets.WorkerPassword, pass); err != nil {
			return err
		}
		e.cfg.WorkerUser, e.cfg.RobloxStudio = *user, *roblox
		if err := config.Save(e.cfgPath, e.cfg); err != nil {
			return err
		}
		fmt.Printf("Окружение windows_user: %s, рабочая папка %s, Roblox Studio: %v. Пароль зашифрован DPAPI.\n",
			*user, e.cfg.WorkRoot, *roblox)
		return nil

	case "set-skills":
		fs := flag.NewFlagSet("set-skills", flag.ExitOnError)
		path := fs.String("path", "", "папка репозитория")
		required := fs.String("required", "", "обязательные скиллы через запятую")
		forbidden := fs.String("forbidden", "", "запрещённые скиллы через запятую")
		_ = fs.Parse(args)
		if e.client == nil {
			return api.ErrAuthRequired
		}
		reg, err := skills.Load(findNearExe(e.cfg.SkillsDir, "skills"))
		if err != nil {
			return err
		}
		split := func(v string) []string {
			var out []string
			for _, x := range strings.Split(v, ",") {
				if x = strings.TrimSpace(x); x != "" {
					if _, ok := reg.Get(x); !ok {
						fmt.Printf("Внимание: скилла %q нет в каталоге этого ПК.\n", x)
					}
					out = append(out, x)
				}
			}
			return out
		}
		repo, err := findRepo(ctx, e, *path)
		if err != nil {
			return err
		}
		req, forb := split(*required), split(*forbidden)
		if err := e.client.SetSkillPolicy(ctx, repo.ID, req, forb); err != nil {
			return err
		}
		fmt.Printf("Скиллы %s: обязательные %v, запрещённые %v.\n", repo.Name, req, forb)
		return nil

	case "set-profile":
		fs := flag.NewFlagSet("set-profile", flag.ExitOnError)
		path := fs.String("path", "", "папка репозитория")
		file := fs.String("profile", "", "JSON-файл профиля, например {\"template\":\"flutter\"}")
		_ = fs.Parse(args)
		if e.client == nil {
			return api.ErrAuthRequired
		}
		b, err := os.ReadFile(*file)
		if err != nil {
			return err
		}
		p, err := verify.ParseProfile(b)
		if err != nil {
			return err
		}
		repo, err := findRepo(ctx, e, *path)
		if err != nil {
			return err
		}
		if err := e.client.UpdateProfile(ctx, repo.ID, b); err != nil {
			return err
		}
		fmt.Printf("Профиль %s обновлён: %d шагов (попытка — %d, слияние — %d).\n", repo.Name, len(p.Steps),
			len(p.ForStage(verify.StageAttempt).Steps), len(p.ForStage(verify.StageMerge).Steps))
		return nil

	case "promote":
		fs := flag.NewFlagSet("promote", flag.ExitOnError)
		path := fs.String("path", "", "папка репозитория")
		_ = fs.Parse(args)
		return promote(ctx, e, *path)
	}
	usage()
	return fmt.Errorf("неизвестная команда %q", cmd)
}

// resolveProject — id проекта по имени (как в дереве) или сам id, если передан id.
func resolveProject(ctx context.Context, c *api.Client, nameOrID string) (string, error) {
	if _, err := uuid.Parse(nameOrID); err == nil {
		return nameOrID, nil
	}
	ps, err := c.Projects(ctx)
	if err != nil {
		return "", err
	}
	var found []api.Project
	for _, p := range ps {
		if strings.EqualFold(strings.TrimSpace(p.Name), strings.TrimSpace(nameOrID)) {
			found = append(found, p)
		}
	}
	switch len(found) {
	case 1:
		return found[0].ID, nil
	case 0:
		var names []string
		for _, p := range ps {
			names = append(names, "«"+p.Name+"»")
		}
		return "", fmt.Errorf("проекта «%s» нет; есть: %s", nameOrID, strings.Join(names, ", "))
	}
	return "", fmt.Errorf("проектов «%s» несколько — укажите id: %s, %s…", nameOrID, found[0].ID, found[1].ID)
}

func register(ctx context.Context, e *env) (api.Host, error) {
	return e.client.RegisterHost(ctx, e.cfg.HostName, runtime.GOOS, e.cfg.Capabilities, e.cfg.Environments,
		[]api.WorkerSpec{{Kind: "claude_code", Model: e.cfg.ClaudeModel}})
}

func runHost(ctx context.Context, e *env) error {
	if e.client == nil {
		return api.ErrAuthRequired
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	h, err := register(ctx, e)
	if err != nil {
		return err
	}
	scanner := hostScanner(e)
	cfgDir, _ := config.Dir()
	sink := &audit.Sink{Dir: filepath.Join(cfgDir, "audit"), Scanner: scanner}
	go sink.Run(ctx, e.client, 15*time.Second) // аудит досылается пачками; без сети ждёт на диске
	run := audit.Runner{Inner: runner.New("git", "docker"), Sink: sink}
	g := gitx.Git{R: run}
	claude := &agent.ClaudeCode{R: run, Image: e.cfg.AgentImage, Model: e.cfg.ClaudeModel,
		Token: func() (string, error) {
			t, err := e.sec.Get(secrets.ClaudeToken)
			return cleanToken(t), err
		}}

	// 7.5: реестры скиллов и MCP, Junior на Ollama, Manager.
	skillReg, err := skills.Load(findNearExe(e.cfg.SkillsDir, "skills"))
	if err != nil {
		return err
	}
	mcpReg, err := mcp.Load(findNearExe(e.cfg.MCPRegistry, filepath.Join("mcp", "registry.yaml")))
	if err != nil {
		return err
	}
	executors := []manager.Option{{Name: "claude", Description: "Claude Code (Sonnet): any coding task, required for complex or architectural work"}}
	var junior agent.Provider
	if ollamaHas(ctx, e.cfg.OllamaURL, e.cfg.JuniorModel) {
		// Свой исполнитель AiCrew: разговор с моделью ведёт служба через API Ollama, а не Claude Code.
		junior = &agent.Local{R: run, Image: e.cfg.AgentImage, BaseURL: e.cfg.OllamaURL, Model: e.cfg.JuniorModel,
			NumCtx: e.cfg.JuniorContext, Log: log}
		executors = append(executors, manager.Option{Name: "junior", Description: "local small model (" + e.cfg.JuniorModel +
			"): only mechanical work with an exact spec — renames, simple tests, docs, small edits"})
	}
	var visionQA review.Reviewer
	if ollamaHas(ctx, e.cfg.OllamaURL, e.cfg.VisionModel) {
		visionQA = vision.Vision{R: run, BuildImage: e.cfg.AgentImage, Image: "aicrew-vision:local",
			OllamaURL: e.cfg.OllamaURL, Model: e.cfg.VisionModel}
	}
	envs, caps := append([]string{}, e.cfg.Environments...), append([]string{}, e.cfg.Capabilities...)
	var windowsEnv *host.WindowsEnv
	if e.cfg.WorkerUser != "" {
		if pw, err := e.sec.Get(secrets.WorkerPassword); err == nil && pw != "" {
			allowed := map[string]bool{strings.ToLower(e.cfg.WorkerClaude): true}
			for _, p := range e.cfg.WorkerTools {
				allowed[strings.ToLower(p)] = true
			}
			ra := audit.Runner{Sink: sink, Inner: runas.Runner{User: e.cfg.WorkerUser, WorkRoot: e.cfg.WorkRoot, Allowed: allowed,
				Password: func() (string, error) { return e.sec.Get(secrets.WorkerPassword) }}}
			windowsEnv = &host.WindowsEnv{WorkRoot: e.cfg.WorkRoot,
				Agent:    &agent.ClaudeCode{R: ra, Model: e.cfg.ClaudeModel, NativeExe: e.cfg.WorkerClaude, Token: claude.Token},
				Verifier: verify.Native{R: ra, Tools: e.cfg.WorkerTools}}
			envs = append(envs, "windows_user")
			if e.cfg.RobloxStudio {
				caps = append(caps, "roblox_studio")
			}
		}
	}
	reg := api.Registries{Skills: skillReg.Names(), MCPProfiles: mcpReg.ProfileNames()}
	for _, x := range executors {
		reg.Executors = append(reg.Executors, x.Name)
	}
	h, err = e.client.RegisterHost(ctx, e.cfg.HostName, runtime.GOOS, caps, envs,
		[]api.WorkerSpec{{Kind: "claude_code", Model: e.cfg.ClaudeModel}}, reg)
	if err != nil {
		return err
	}
	workerID := h.Workers["claude_code/"+e.cfg.ClaudeModel]
	skillOpts := map[string]string{}
	for _, sk := range skillReg.All() {
		skillOpts[sk.Name] = sk.Description
	}
	mcpOpts := map[string]string{}
	for name, p := range mcpReg.Profiles {
		if len(p.Environments) == 0 || containsAny(p.Environments, envs) {
			mcpOpts[name] = p.Description
		}
	}
	hst := &host.Host{
		API: e.client, Git: g, WorkerID: workerID, Log: log,
		PollEvery: 10 * time.Second, HeartbeatEvery: 30 * time.Second,
		Agent:    claude,
		Verifier: verify.Verifier{R: run, DefaultImage: e.cfg.AgentImage},
		Reviewer: reviewers{
			// Logic QA: Gemini на сервере; пока ключа нет — Claude Haiku на ПК (решение 7.4/1).
			logic: review.Chain{Primary: review.Server{API: e.client, WorkerID: workerID},
				Fallback: review.Claude{Agent: claude, Model: e.cfg.ReviewModel}},
			techLead: review.Claude{Agent: claude, Model: e.cfg.TechLeadModel},
			vision:   visionQA,
		},
		Junior:  junior,
		Skills:  skillReg,
		MCP:     mcpReg,
		Planner: manager.Manager{Agent: claude, Model: e.cfg.ManagerModel},
		Windows: windowsEnv,
		Scanner: scanner,
		Audit:   sink,
		Catalog: manager.Catalog{Capabilities: caps, Environments: envs,
			Executors: executors, Skills: manager.Options(skillOpts), MCPProfiles: manager.Options(mcpOpts)},
	}

	// При старте — уборка осиротевших worktree (ТЗ п. 8.2).
	repos, err := e.client.ListRepositories(ctx)
	if err != nil {
		return err
	}
	var paths []string
	for _, r := range repos {
		if r.HostID == h.ID {
			paths = append(paths, r.LocalPath)
		}
	}
	removed, err := hst.CleanupOrphans(ctx, paths)
	if err != nil {
		return err
	}
	log.Info("aicrew-host started", "host", h.ID, "worker", workerID, "repositories", len(paths), "orphans_removed", len(removed),
		"skills", len(reg.Skills), "mcp_profiles", reg.MCPProfiles, "executors", reg.Executors, "vision_qa", visionQA != nil, "environments", envs)
	return hst.Run(ctx)
}

// promote переносит integration-ветку в основную (решение 7.3: это делает только человек).
// Перед переносом integration ещё раз проходит проверку; main двигается только перемоткой.
func promote(ctx context.Context, e *env, path string) error {
	if e.client == nil {
		return api.ErrAuthRequired
	}
	repo, err := findRepo(ctx, e, path)
	if err != nil {
		return err
	}
	abs := repo.LocalPath
	run := runner.New("git", "docker")
	g := gitx.Git{R: run}
	integ := repo.Integration()
	head, err := g.RevParse(ctx, abs, integ)
	if err != nil {
		return fmt.Errorf("ветки %s нет: AiCrew ещё ничего не слил", integ)
	}
	if g.IsAncestor(ctx, abs, head, repo.DefaultBranch) {
		fmt.Printf("%s уже содержит всё из %s — переносить нечего.\n", repo.DefaultBranch, integ)
		return nil
	}

	profile, err := verify.ParseProfile(repo.VerificationProfile)
	if err != nil {
		return err
	}
	fmt.Printf("Проверка %s (%s)…\n", integ, head[:12])
	tmp, err := g.AddTempWorktree(ctx, abs, "promote-check", head)
	if err != nil {
		return err
	}
	defer g.RemoveTempWorktree(context.WithoutCancel(ctx), abs, "promote-check")
	rep, err := verify.Verifier{R: run, DefaultImage: e.cfg.AgentImage}.Run(ctx, profile.ForStage(verify.StageMerge), tmp,
		"aicrew-verify-promote", 20*time.Minute)
	if err != nil {
		return err
	}
	for _, s := range rep.Steps {
		fmt.Printf("  %s: exit %d (%d с)\n", s.Name, s.ExitCode, s.Seconds)
	}
	if !rep.Passed {
		// Integration красная — инцидент: AiCrew на паузе по этому репозиторию до «Возобновить» (7.6).
		last := rep.Steps[len(rep.Steps)-1]
		if err := e.client.ReportIntegrationRed(ctx, repo.ID, fmt.Sprintf("step %q exited with %d", last.Name, last.ExitCode)); err != nil {
			fmt.Println("Не удалось сообщить об инциденте:", err)
		}
		return fmt.Errorf("%s не прошла проверку — %s не тронута, AiCrew по репозиторию на паузе", integ, repo.DefaultBranch)
	}
	from, to, err := g.FastForward(ctx, abs, repo.DefaultBranch, head)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %s → %s\n", repo.DefaultBranch, from[:12], to[:12])
	return nil
}

var (
	tokenRe     = regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]+`)
	tokenTailRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// extractToken находит токен в многострочном тексте: строку с «sk-ant-» плюс следующие
// строки, если они целиком из символов токена (перенос длинной строки в терминале).
func extractToken(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		l = printable(l)
		loc := tokenRe.FindStringIndex(l)
		if loc == nil {
			continue
		}
		tok := l[loc[0]:loc[1]]
		if strings.TrimSpace(l[loc[1]:]) == "" {
			for _, next := range lines[i+1:] {
				next = strings.TrimSpace(printable(next))
				if !tokenTailRe.MatchString(next) {
					break
				}
				tok += next
			}
		}
		return tok
	}
	return cleanToken(s)
}

// printable убирает управляющие и не-ASCII символы, пробелы оставляет.
func printable(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r < 127 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// cleanToken достаёт сам токен из вставленного текста: убирает невидимые символы, переносы
// строк и всё вокруг токена (escape-последовательности вставки, соседний текст терминала).
func cleanToken(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r > 32 && r < 127 {
			b.WriteRune(r)
		}
	}
	t := b.String()
	if m := tokenRe.FindString(t); m != "" {
		return m
	}
	return t
}

// findRepo — привязанный к AiCrew репозиторий по папке.
func findRepo(ctx context.Context, e *env, path string) (*api.Repository, error) {
	abs, err := filepath.Abs(path)
	if err != nil || path == "" {
		return nil, errors.New("нужен -path <папка репозитория>")
	}
	repos, err := e.client.ListRepositories(ctx)
	if err != nil {
		return nil, err
	}
	for i := range repos {
		if strings.EqualFold(filepath.Clean(repos[i].LocalPath), filepath.Clean(abs)) {
			return &repos[i].Repository, nil
		}
	}
	return nil, fmt.Errorf("%s не привязан к AiCrew (add-repo)", abs)
}

// reviewers — Logic QA для всех задач, Tech Lead — для задач с пометкой (решение 7.4/4).
type reviewers struct {
	logic, techLead, vision review.Reviewer
}

func (r reviewers) Review(ctx context.Context, role string, t *api.Task, workdir string, f review.Facts) (api.Verdict, error) {
	switch role {
	case review.RoleTechLead:
		return r.techLead.Review(ctx, role, t, workdir, f)
	case vision.Role:
		if r.vision == nil {
			return api.Verdict{Verdict: "approve", Issues: []api.Issue{}, Summary: "vision QA skipped: no local vision model on this host",
				Reviewer: vision.Role + ":skipped"}, nil
		}
		return r.vision.Review(ctx, role, t, workdir, f)
	}
	return r.logic.Review(ctx, role, t, workdir, f)
}

// findNearExe — путь из настроек или каталог рядом с программой (…/bin/aicrew-host.exe → …/aicrew/<rel>).
func findNearExe(configured, rel string) string {
	if configured != "" {
		return configured
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	for _, cand := range []string{filepath.Join(dir, rel), filepath.Join(dir, "..", rel), filepath.Join(dir, "..", "..", "aicrew", rel)} {
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return ""
}

// ollamaHas — Ollama отвечает и модель скачана.
func ollamaHas(ctx context.Context, base, model string) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/api/tags", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if json.NewDecoder(resp.Body).Decode(&tags) != nil {
		return false
	}
	for _, m := range tags.Models {
		if m.Name == model {
			return true
		}
	}
	return false
}

func containsAny(xs, ys []string) bool {
	for _, x := range xs {
		for _, y := range ys {
			if x == y {
				return true
			}
		}
	}
	return false
}

// hostScanner — сканер секретов с точными значениями секретов этого хоста (самая надёжная проверка).
func hostScanner(e *env) *secretscan.Scanner {
	exact := map[string]string{}
	for name, kind := range map[string]string{secrets.ClaudeToken: "claude_token", secrets.WorkerPassword: "worker_password",
		secrets.RefreshToken: "server_refresh_token"} {
		if v, err := e.sec.Get(name); err == nil && v != "" {
			exact[v] = kind
			if name == secrets.ClaudeToken {
				exact[cleanToken(v)] = kind
			}
		}
	}
	return secretscan.New(exact)
}
