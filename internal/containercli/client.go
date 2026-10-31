package containercli

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner is the narrow boundary between the TUI and the container executable.
type Runner interface {
	Run(context.Context, ...string) ([]byte, error)
}

// CLI executes Apple's container command without a shell.
type CLI struct{}

func (CLI) Run(ctx context.Context, args ...string) ([]byte, error) {
	output, err := exec.CommandContext(ctx, "container", args...).CombinedOutput()
	if err == nil {
		return output, nil
	}

	message := strings.TrimSpace(string(output))
	if message != "" {
		return output, fmt.Errorf("container %s: %s", strings.Join(args, " "), message)
	}
	return output, fmt.Errorf("container %s: %w", strings.Join(args, " "), err)
}

// Run executes a command that may take several minutes, such as pulling an image.
func Run(runner Runner, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	output, err := runner.Run(ctx, args...)
	return strings.TrimSpace(string(output)), err
}
