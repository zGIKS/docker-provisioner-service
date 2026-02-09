package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Runtime struct {
	bin     string
	timeout time.Duration
}

func NewRuntime(bin string, timeout time.Duration) *Runtime {
	return &Runtime{bin: bin, timeout: timeout}
}

func (r *Runtime) Run(parentCtx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parentCtx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.bin, args...)
	output, err := cmd.CombinedOutput()
	out := strings.TrimSpace(string(output))

	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("docker %v timeout after %s", args, r.timeout)
	}
	if err != nil {
		if out == "" {
			return "", fmt.Errorf("docker %v failed: %w", args, err)
		}
		return "", fmt.Errorf("docker %v failed: %s", args, out)
	}

	return out, nil
}
