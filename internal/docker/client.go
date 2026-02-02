package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// Client wraps the Docker client
type Client struct {
	cli *client.Client
	ctx context.Context
}

// splitVolume splits a volume string like "source:target:options"
// handling absolute paths that start with /
func splitVolume(vol string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(vol); i++ {
		if vol[i] == ':' {
			parts = append(parts, vol[start:i])
			start = i + 1
		}
	}
	if start < len(vol) {
		parts = append(parts, vol[start:])
	}
	return parts
}

// NewClient creates a new Docker client
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Client{
		cli: cli,
		ctx: context.Background(),
	}, nil
}

// Close closes the Docker client
func (c *Client) Close() error {
	return c.cli.Close()
}

// ContainerExists checks if a container exists (running or stopped)
func (c *Client) ContainerExists(name string) bool {
	filterArgs := filters.NewArgs()
	filterArgs.Add("name", "^/"+name+"$")

	containers, err := c.cli.ContainerList(c.ctx, types.ContainerListOptions{
		All:     true,
		Filters: filterArgs,
	})

	return err == nil && len(containers) > 0
}

// ContainerRunning checks if a container is currently running
func (c *Client) ContainerRunning(name string) bool {
	filterArgs := filters.NewArgs()
	filterArgs.Add("name", "^/"+name+"$")
	filterArgs.Add("status", "running")

	containers, err := c.cli.ContainerList(c.ctx, types.ContainerListOptions{
		Filters: filterArgs,
	})

	return err == nil && len(containers) > 0
}

// GetContainerStatus returns the status of a container
func (c *Client) GetContainerStatus(name string) (string, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("name", "^/"+name+"$")

	containers, err := c.cli.ContainerList(c.ctx, types.ContainerListOptions{
		All:     true,
		Filters: filterArgs,
	})

	if err != nil {
		return "", err
	}

	if len(containers) == 0 {
		return "not found", nil
	}

	return containers[0].State, nil
}

// CreateContainer creates a new container
func (c *Client) CreateContainer(name, image, workdir string, volumes []string, envVars []string, networkName string) error {
	// Convert volumes to mounts
	var mounts []mount.Mount
	for _, vol := range volumes {
		// Parse "source:target" or "source:target:options" format
		parts := splitVolume(vol)
		if len(parts) < 2 {
			continue
		}

		source := parts[0]
		target := parts[1]
		readOnly := false

		if len(parts) >= 3 && parts[2] == "ro" {
			readOnly = true
		}

		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   source,
			Target:   target,
			ReadOnly: readOnly,
		})
	}

	config := &container.Config{
		Image:      image,
		WorkingDir: workdir,
		Tty:        true,
		OpenStdin:  true,
		Env:        envVars,
	}

	hostConfig := &container.HostConfig{
		Mounts: mounts,
	}

	// Set up network config if specified
	var networkConfig *networktypes.NetworkingConfig
	if networkName != "" {
		networkConfig = &networktypes.NetworkingConfig{
			EndpointsConfig: map[string]*networktypes.EndpointSettings{
				networkName: {},
			},
		}
	}

	_, err := c.cli.ContainerCreate(c.ctx, config, hostConfig, networkConfig, nil, name)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	return nil
}

// StartContainer starts a stopped container
func (c *Client) StartContainer(name string) error {
	if err := c.cli.ContainerStart(c.ctx, name, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	return nil
}

// StopContainer stops a running container
func (c *Client) StopContainer(name string) error {
	if err := c.cli.ContainerStop(c.ctx, name, container.StopOptions{}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	return nil
}

// RemoveContainer removes a container
func (c *Client) RemoveContainer(name string) error {
	if err := c.cli.ContainerRemove(c.ctx, name, types.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}
	return nil
}

// ExecNonInteractive runs a command in a container and returns the output
func (c *Client) ExecNonInteractive(containerName string, cmd []string) (int, string, error) {
	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := c.cli.ContainerExecCreate(c.ctx, containerName, execConfig)
	if err != nil {
		return -1, "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := c.cli.ContainerExecAttach(c.ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return -1, "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Read output
	output, err := io.ReadAll(resp.Reader)
	if err != nil {
		return -1, "", fmt.Errorf("failed to read exec output: %w", err)
	}

	// Get exit code
	inspect, err := c.cli.ContainerExecInspect(c.ctx, execID.ID)
	if err != nil {
		return -1, string(output), fmt.Errorf("failed to inspect exec: %w", err)
	}

	return inspect.ExitCode, string(output), nil
}

// PullImage pulls a Docker image
func (c *Client) PullImage(image string) error {
	reader, err := c.cli.ImagePull(c.ctx, image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// Copy output to stdout to show progress
	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, reader)
	if err != nil {
		// Fallback to regular copy
		_, err = io.Copy(os.Stdout, reader)
	}

	return err
}

// ImageExists checks if an image exists locally
func (c *Client) ImageExists(image string) bool {
	_, _, err := c.cli.ImageInspectWithRaw(c.ctx, image)
	return err == nil
}

// CopyFileToContainer copies a file from host to container
func (c *Client) CopyFileToContainer(containerName, srcPath, dstPath string) error {
	// Read source file
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Get file info for permissions
	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	// Create tar archive with the file
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	hdr := &tar.Header{
		Name: filepath.Base(dstPath),
		Mode: int64(info.Mode()),
		Size: int64(len(content)),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("failed to write tar content: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Copy to container
	err = c.cli.CopyToContainer(c.ctx, containerName, filepath.Dir(dstPath), &buf, types.CopyToContainerOptions{})
	if err != nil {
		return fmt.Errorf("failed to copy to container: %w", err)
	}

	return nil
}
