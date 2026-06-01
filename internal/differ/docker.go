package differ

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultImage = "postgres:17.10-alpine3.23"

// Container represents a running ephemeral Docker container.
type Container struct {
	ID string
}

// StartContainer starts a PostgreSQL container and waits until it is ready
// to accept connections. The caller must call StopContainer when done.
func StartContainer(ctx context.Context) (*Container, error) {
	out, err := hostCmdOutput(ctx, "docker", "run", "-d", "--rm",
		"-e", "POSTGRES_HOST_AUTH_METHOD=trust",
		defaultImage,
	)
	if err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}
	id := strings.TrimSpace(string(out))
	c := &Container{ID: id}
	if err := waitReady(ctx, id); err != nil {
		_ = StopContainer(id)
		return nil, err
	}
	return c, nil
}

// StopContainer force-removes the container.
func StopContainer(containerID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := hostCmdOutput(ctx, "docker", "rm", "-f", containerID)
	return err
}

// waitReady polls pg_isready inside the container until postgres accepts
// connections or the 60-second deadline is reached.
// Each probe carries its own 5-second timeout so a hung docker-exec call
// cannot block the loop forever.
func waitReady(ctx context.Context, containerID string) error {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := ExecOutput(probeCtx, containerID, "pg_isready", "-U", "postgres")
		cancel()
		if err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("container %s: postgres not ready within 60s", containerID)
}

// ExecOutput runs a command inside the container and returns its stdout.
// stderr is captured separately and included in any error.
func ExecOutput(ctx context.Context, containerID string, args ...string) ([]byte, error) {
	fullArgs := append([]string{"exec", containerID}, args...)
	cmd := exec.CommandContext(ctx, "docker", fullArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker exec %v: %w\n%s", args, err, stderr.String())
	}
	return out, nil
}

// CopyToContainer copies localPath on the host into containerPath inside the container.
func CopyToContainer(ctx context.Context, containerID, localPath, containerPath string) error {
	_, err := hostCmdOutput(ctx, "docker", "cp", localPath, containerID+":"+containerPath)
	return err
}

// hostCmdOutput runs a command on the host and returns its combined output.
func hostCmdOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", name, err, out)
	}
	return out, nil
}
