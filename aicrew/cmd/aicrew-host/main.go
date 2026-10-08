// aicrew-host — служба Execution Host AiCrew на домашнем ПК (docs/stage7-2-state-machine.md).
//
//	aicrew-host login              — войти на сервер PromptTree как устройство-хост
//	aicrew-host set-claude-token   — сохранить токен `claude setup-token` (зашифрован DPAPI)
//	aicrew-host add-repo …         — привязать локальный репозиторий к проекту PromptTree
//	aicrew-host run                — брать и выполнять задачи
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"golang.org/x/term"

	"prompttree/aicrew/internal/agent"
	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/config"
	"prompttree/aicrew/internal/gitx"
	"prompttree/aicrew/internal/host"
	"prompttree/aicrew/internal/runner"
	"prompttree/aicrew/internal/secrets"
	"prompttree/aicrew/internal/verify"
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
  set-claude-token   сохранить токен из "claude setup-token" (ввод скрыт)
  add-repo           привязать локальный репозиторий: -project <id> -path <папка> [-name] [-branch] [-profile файл.json]
  run                брать и выполнять задачи`)
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

func readLine(prompt string) string {
	fmt.Print(prompt)
	s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(s)
}

func readSecret(prompt string) (string, error) {
	fmt.Print(prompt)
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
		tok, err := readSecret("Токен: ")
		if err != nil {
			return err
		}
		if tok == "" {
			return errors.New("пустой токен")
		}
		if err := e.sec.Set(secrets.ClaudeToken, tok); err != nil {
			return err
		}
		fmt.Println("Токен сохранён.")
		return nil

	case "add-repo":
		fs := flag.NewFlagSet("add-repo", flag.ExitOnError)
		project := fs.String("project", "", "id проекта PromptTree")
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
		h, err := register(ctx, e)
		if err != nil {
			return err
		}
		r, err := e.client.AddRepository(ctx, *project, h.ID, *name, abs, *branch, prof)
		if err != nil {
			return err
		}
		fmt.Println("Репозиторий привязан:", r.ID)
		return nil

	case "run":
		return runHost(ctx, e)
	}
	usage()
	return fmt.Errorf("неизвестная команда %q", cmd)
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
	workerID := h.Workers["claude_code/"+e.cfg.ClaudeModel]
	run := runner.New("git", "docker")
	g := gitx.Git{R: run}
	hst := &host.Host{
		API: e.client, Git: g, WorkerID: workerID, Log: log,
		PollEvery: 10 * time.Second, HeartbeatEvery: 30 * time.Second,
		Agent: &agent.ClaudeCode{R: run, Image: e.cfg.AgentImage, Model: e.cfg.ClaudeModel,
			Token: func() (string, error) { return e.sec.Get(secrets.ClaudeToken) }},
		Verifier: verify.Verifier{R: run, DefaultImage: e.cfg.AgentImage},
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
	log.Info("aicrew-host started", "host", h.ID, "worker", workerID, "repositories", len(paths), "orphans_removed", len(removed))
	return hst.Run(ctx)
}
