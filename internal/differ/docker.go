package differ

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const defaultImage = "postgres:17.10-alpine3.23"

// Container represents a running ephemeral PostgreSQL Docker container.
type Container struct {
	ID  string
	cli *dockerclient.Client
}

// StartContainer creates a Docker client, ensures the image is present,
// starts a PostgreSQL container with a built-in health check, and waits
// until postgres is ready. The caller must call Stop when done.
func StartContainer(ctx context.Context) (*Container, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}

	if err := ensureImage(ctx, cli, defaultImage); err != nil {
		cli.Close()
		return nil, err
	}

	cfg := &container.Config{
		Image: defaultImage,
		Env:   []string{"POSTGRES_HOST_AUTH_METHOD=trust"},
		Healthcheck: &container.HealthConfig{
			Test:     []string{"CMD", "pg_isready", "-U", "postgres"},
			Interval: time.Second,
			Retries:  60,
		},
	}
	resp, err := cli.ContainerCreate(ctx, cfg, nil, nil, nil, "")
	if err != nil {
		cli.Close()
		return nil, fmt.Errorf("create container: %w", err)
	}

	c := &Container{ID: resp.ID, cli: cli}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = c.Stop()
		return nil, fmt.Errorf("start container: %w", err)
	}

	readyCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := c.waitReady(readyCtx); err != nil {
		_ = c.Stop()
		return nil, err
	}
	return c, nil
}

// Stop force-removes the container and closes the Docker client.
func (c *Container) Stop() error {
	defer c.cli.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return c.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
}

// waitReady polls ContainerInspect until the built-in health check reports healthy.
func (c *Container) waitReady(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("container %s: postgres not ready: %w", c.ID[:12], ctx.Err())
		default:
		}

		info, err := c.cli.ContainerInspect(ctx, c.ID)
		if err != nil {
			return fmt.Errorf("inspect container: %w", err)
		}
		if info.State != nil && info.State.Health != nil {
			switch info.State.Health.Status {
			case container.Healthy:
				return nil
			case container.Unhealthy:
				return fmt.Errorf("container %s became unhealthy", c.ID[:12])
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// execOutput runs a command inside the container and returns its stdout.
// stderr is captured and included in any error.
func (c *Container) execOutput(ctx context.Context, args ...string) ([]byte, error) {
	exec, err := c.cli.ContainerExecCreate(ctx, c.ID, container.ExecOptions{
		Cmd:          args,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("exec create %v: %w", args, err)
	}

	resp, err := c.cli.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("exec attach %v: %w", args, err)
	}
	defer resp.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, resp.Reader); err != nil {
		return nil, fmt.Errorf("exec read %v: %w", args, err)
	}

	info, err := c.cli.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return nil, fmt.Errorf("exec inspect %v: %w", args, err)
	}
	if info.ExitCode != 0 {
		return nil, fmt.Errorf("exec %v: exit %d\n%s", args, info.ExitCode, stderr.String())
	}
	return stdout.Bytes(), nil
}

// copyFile copies a local file into the container at containerPath (Linux path).
func (c *Container) copyFile(ctx context.Context, localPath, containerPath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name: path.Base(containerPath),
		Mode: 0644,
		Size: int64(len(data)),
	}); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return c.cli.CopyToContainer(ctx, c.ID, path.Dir(containerPath), &buf, container.CopyToContainerOptions{})
}

// ensureImage pulls the image only if it is not already present locally.
func ensureImage(ctx context.Context, cli *dockerclient.Client, ref string) error {
	_, _, err := cli.ImageInspectWithRaw(ctx, ref)
	if err == nil {
		return nil
	}
	if !dockerclient.IsErrNotFound(err) {
		return fmt.Errorf("inspect image %s: %w", ref, err)
	}
	rc, err := cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull %s: %w", ref, err)
	}
	defer rc.Close()
	_, err = io.Copy(io.Discard, rc)
	return err
}
