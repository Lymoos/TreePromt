//go:build !windows

package runas

import (
	"context"
	"errors"

	"prompttree/aicrew/internal/runner"
)

func (r Runner) Run(context.Context, runner.Command) (runner.Result, error) {
	return runner.Result{}, errors.New("windows_user environment is available only on Windows")
}
