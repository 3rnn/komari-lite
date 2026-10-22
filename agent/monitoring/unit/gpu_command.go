package monitoring

import (
	"context"
	"errors"
	"os/exec"
	"time"
)

const gpuCommandTimeout = 2 * time.Second
const gpuOutputLimit = 4 * 1024 * 1024

type gpuOutput struct{ data []byte }

func (b *gpuOutput) Write(p []byte) (int, error) {
	if len(p) > gpuOutputLimit-len(b.data) {
		return 0, errors.New("GPU command output exceeds limit")
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

func runGPUCommand(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gpuCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	// A descendant retaining stdout/stderr must not hold collection open.
	cmd.WaitDelay = 100 * time.Millisecond
	out := &gpuOutput{}
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.data, nil
}
