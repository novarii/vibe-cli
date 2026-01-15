package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/docker/api/types"
	"github.com/moby/term"
)

// ExecInteractive runs an interactive command in a container with TTY support
func (c *Client) ExecInteractive(containerName string, cmd []string) (int, error) {
	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}

	execID, err := c.cli.ContainerExecCreate(c.ctx, containerName, execConfig)
	if err != nil {
		return -1, fmt.Errorf("failed to create exec: %w", err)
	}

	// Get terminal state
	fd := os.Stdin.Fd()
	oldState, err := term.SetRawTerminal(fd)
	if err != nil {
		return -1, fmt.Errorf("failed to set raw terminal: %w", err)
	}
	defer term.RestoreTerminal(fd, oldState)

	// Attach to exec
	resp, err := c.cli.ContainerExecAttach(c.ctx, execID.ID, types.ExecStartCheck{
		Tty: true,
	})
	if err != nil {
		return -1, fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Handle resize
	go c.handleResize(execID.ID)

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Stream I/O
	errChan := make(chan error, 2)

	// Copy stdin to container
	go func() {
		_, err := io.Copy(resp.Conn, os.Stdin)
		errChan <- err
	}()

	// Copy container output to stdout
	go func() {
		_, err := io.Copy(os.Stdout, resp.Reader)
		errChan <- err
	}()

	// Wait for either stream to finish or signal
	select {
	case <-sigChan:
		// User interrupted
	case <-errChan:
		// One of the streams finished
	}

	// Get exit code
	inspect, err := c.cli.ContainerExecInspect(c.ctx, execID.ID)
	if err != nil {
		return -1, nil // Don't fail, just return unknown exit code
	}

	return inspect.ExitCode, nil
}

// handleResize handles terminal resize events
func (c *Client) handleResize(execID string) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)
	defer signal.Stop(sigChan)

	for range sigChan {
		c.resizeExec(execID)
	}
}

// resizeExec resizes the exec TTY
func (c *Client) resizeExec(execID string) {
	fd := os.Stdout.Fd()
	ws, err := term.GetWinsize(fd)
	if err != nil {
		return
	}

	c.cli.ContainerExecResize(context.Background(), execID, types.ResizeOptions{
		Height: uint(ws.Height),
		Width:  uint(ws.Width),
	})
}

// ExecWithOutput runs a command and captures output while also displaying it
func (c *Client) ExecWithOutput(containerName string, cmd []string) (int, string, error) {
	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}

	execID, err := c.cli.ContainerExecCreate(c.ctx, containerName, execConfig)
	if err != nil {
		return -1, "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Get terminal state
	fd := os.Stdin.Fd()
	oldState, err := term.SetRawTerminal(fd)
	if err != nil {
		return -1, "", fmt.Errorf("failed to set raw terminal: %w", err)
	}
	defer term.RestoreTerminal(fd, oldState)

	// Attach to exec
	resp, err := c.cli.ContainerExecAttach(c.ctx, execID.ID, types.ExecStartCheck{
		Tty: true,
	})
	if err != nil {
		return -1, "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Handle resize
	go c.handleResize(execID.ID)

	// Capture output while also displaying it
	var output []byte
	outputWriter := &captureWriter{
		writer: os.Stdout,
		buffer: &output,
	}

	// Stream I/O
	errChan := make(chan error, 2)

	// Copy stdin to container
	go func() {
		_, err := io.Copy(resp.Conn, os.Stdin)
		errChan <- err
	}()

	// Copy container output to stdout and capture
	go func() {
		_, err := io.Copy(outputWriter, resp.Reader)
		errChan <- err
	}()

	// Wait for streams to finish
	<-errChan

	// Get exit code
	inspect, err := c.cli.ContainerExecInspect(c.ctx, execID.ID)
	if err != nil {
		return -1, string(output), nil
	}

	return inspect.ExitCode, string(output), nil
}

// captureWriter writes to both a writer and a buffer
type captureWriter struct {
	writer io.Writer
	buffer *[]byte
}

func (cw *captureWriter) Write(p []byte) (n int, err error) {
	*cw.buffer = append(*cw.buffer, p...)
	return cw.writer.Write(p)
}

// ExecInteractiveWithPrompt runs an interactive command with initial prompt piped to stdin
// This is like: cat prompt.md | claude --dangerously-skip-permissions
func (c *Client) ExecInteractiveWithPrompt(containerName string, cmd []string, initialInput string) (int, string, error) {
	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}

	execID, err := c.cli.ContainerExecCreate(c.ctx, containerName, execConfig)
	if err != nil {
		return -1, "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Get terminal state
	fd := os.Stdin.Fd()
	oldState, err := term.SetRawTerminal(fd)
	if err != nil {
		return -1, "", fmt.Errorf("failed to set raw terminal: %w", err)
	}
	defer term.RestoreTerminal(fd, oldState)

	// Attach to exec
	resp, err := c.cli.ContainerExecAttach(c.ctx, execID.ID, types.ExecStartCheck{
		Tty: true,
	})
	if err != nil {
		return -1, "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Handle resize
	go c.handleResize(execID.ID)

	// Capture output while displaying
	var output []byte
	outputWriter := &captureWriter{
		writer: os.Stdout,
		buffer: &output,
	}

	// Stream I/O
	errChan := make(chan error, 2)

	// First write the initial input, then copy from stdin
	go func() {
		// Write the prompt first
		_, err := resp.Conn.Write([]byte(initialInput + "\n"))
		if err != nil {
			errChan <- err
			return
		}
		// Then hand over to user's stdin
		_, err = io.Copy(resp.Conn, os.Stdin)
		errChan <- err
	}()

	// Copy container output to stdout and capture
	go func() {
		_, err := io.Copy(outputWriter, resp.Reader)
		errChan <- err
	}()

	// Wait for streams to finish
	<-errChan

	// Get exit code
	inspect, err := c.cli.ContainerExecInspect(c.ctx, execID.ID)
	if err != nil {
		return -1, string(output), nil
	}

	return inspect.ExitCode, string(output), nil
}
