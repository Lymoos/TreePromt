//go:build windows

package runas

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"prompttree/aicrew/internal/runner"
)

var procCreateProcessWithLogonW = windows.NewLazySystemDLL("advapi32.dll").NewProc("CreateProcessWithLogonW")

const logonWithProfile = 0x00000001

// Run запускает cmd от имени ai_worker. Вход и вывод — через файлы в WorkRoot\.runs (ai_worker
// имеет права только там): так не нужна оболочка, а длинный текст задачи не попадает в командную строку.
// Процесс и все его потомки — в Job-объекте: таймаут или отмена убивают всё дерево.
func (r Runner) Run(ctx context.Context, cmd runner.Command) (runner.Result, error) {
	var res runner.Result
	exe := cmd.Exe
	if !filepath.IsAbs(exe) || !r.Allowed[strings.ToLower(exe)] {
		return res, fmt.Errorf("runas: %s is not allowed for %s", exe, r.User)
	}
	pass, err := r.Password()
	if err != nil || pass == "" {
		return res, errors.New("runas: password of " + r.User + " is not set: run `aicrew-host set-worker-password`")
	}
	runDir, err := os.MkdirTemp(filepath.Join(r.WorkRoot, ".runs"), "run-")
	if err != nil {
		return res, err
	}
	defer os.RemoveAll(runDir)
	inPath, outPath, errPath := filepath.Join(runDir, "in.txt"), filepath.Join(runDir, "out.txt"), filepath.Join(runDir, "err.txt")
	if err := os.WriteFile(inPath, []byte(cmd.Stdin), 0o644); err != nil {
		return res, err
	}
	in, err := inheritable(inPath, windows.GENERIC_READ, windows.OPEN_EXISTING)
	if err != nil {
		return res, err
	}
	defer windows.CloseHandle(in)
	out, err := inheritable(outPath, windows.GENERIC_WRITE, windows.CREATE_ALWAYS)
	if err != nil {
		return res, err
	}
	defer windows.CloseHandle(out)
	errh, err := inheritable(errPath, windows.GENERIC_WRITE, windows.CREATE_ALWAYS)
	if err != nil {
		return res, err
	}
	defer windows.CloseHandle(errh)

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return res, err
	}
	defer windows.CloseHandle(job)
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return res, err
	}

	si := windows.StartupInfo{Flags: windows.STARTF_USESTDHANDLES, StdInput: in, StdOutput: out, StdErr: errh}
	si.Cb = uint32(unsafe.Sizeof(si))
	var pi windows.ProcessInformation
	envBlock := envBlock(Env(r.User, cmd.Env))
	cmdLine, _ := windows.UTF16PtrFromString(CommandLine(exe, cmd.Args))
	user, _ := windows.UTF16PtrFromString(r.User)
	domain, _ := windows.UTF16PtrFromString(".")
	pw, _ := windows.UTF16PtrFromString(pass)
	dir, _ := windows.UTF16PtrFromString(cmd.Dir)
	start := time.Now()
	ok, _, callErr := procCreateProcessWithLogonW.Call(
		uintptr(unsafe.Pointer(user)), uintptr(unsafe.Pointer(domain)), uintptr(unsafe.Pointer(pw)), logonWithProfile,
		0, uintptr(unsafe.Pointer(cmdLine)),
		windows.CREATE_UNICODE_ENVIRONMENT|windows.CREATE_SUSPENDED|windows.CREATE_NO_WINDOW,
		uintptr(unsafe.Pointer(&envBlock[0])), uintptr(unsafe.Pointer(dir)),
		uintptr(unsafe.Pointer(&si)), uintptr(unsafe.Pointer(&pi)))
	if ok == 0 {
		return res, fmt.Errorf("runas %s: %w", r.User, callErr)
	}
	defer windows.CloseHandle(pi.Process)
	defer windows.CloseHandle(pi.Thread)
	if err := windows.AssignProcessToJobObject(job, pi.Process); err != nil {
		_ = windows.TerminateProcess(pi.Process, 1)
		return res, err
	}
	if _, err := windows.ResumeThread(pi.Thread); err != nil {
		_ = windows.TerminateJobObject(job, 1)
		return res, err
	}

	done := make(chan struct{})
	go func() {
		_, _ = windows.WaitForSingleObject(pi.Process, windows.INFINITE)
		close(done)
	}()
	var timeout <-chan time.Time
	if cmd.Timeout > 0 {
		t := time.NewTimer(cmd.Timeout)
		defer t.Stop()
		timeout = t.C
	}
	select {
	case <-done:
	case <-ctx.Done():
		_ = windows.TerminateJobObject(job, 1)
		<-done
	case <-timeout:
		res.TimedOut = true
		_ = windows.TerminateJobObject(job, 1)
		<-done
	}
	var code uint32
	_ = windows.GetExitCodeProcess(pi.Process, &code)
	res.ExitCode, res.Duration = int(code), time.Since(start)
	windows.CloseHandle(out)
	windows.CloseHandle(errh)
	stdout, _ := os.ReadFile(outPath)
	stderr, _ := os.ReadFile(errPath)
	res.Stdout, res.Stderr = string(stdout), string(stderr)
	if cmd.Stdout != nil {
		sc := bufio.NewScanner(strings.NewReader(res.Stdout))
		sc.Buffer(make([]byte, 1<<20), 16<<20)
		for sc.Scan() {
			cmd.Stdout(sc.Text())
		}
	}
	if ctx.Err() != nil {
		return res, ctx.Err()
	}
	return res, nil
}

func inheritable(path string, access uint32, disposition uint32) (windows.Handle, error) {
	p, _ := windows.UTF16PtrFromString(path)
	sa := windows.SecurityAttributes{InheritHandle: 1}
	sa.Length = uint32(unsafe.Sizeof(sa))
	return windows.CreateFile(p, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, &sa, disposition,
		windows.FILE_ATTRIBUTE_NORMAL, 0)
}

// envBlock — блок окружения UTF-16: NAME=value\0…\0\0, отсортированный без учёта регистра.
func envBlock(env []string) []uint16 {
	sortEnv(env)
	var b []uint16
	for _, kv := range env {
		u, _ := windows.UTF16FromString(kv)
		b = append(b, u...)
	}
	return append(b, 0)
}

func sortEnv(env []string) {
	for i := 1; i < len(env); i++ {
		for j := i; j > 0 && strings.ToUpper(env[j]) < strings.ToUpper(env[j-1]); j-- {
			env[j], env[j-1] = env[j-1], env[j]
		}
	}
}
