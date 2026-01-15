package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
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

// ExecStreaming runs a command and streams output through a processor function
// Used for YOLO mode to format stream-json output in real-time
func (c *Client) ExecStreaming(containerName string, cmd []string, processor func(line string)) (int, string, error) {
	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false, // No TTY for non-interactive streaming
	}

	execID, err := c.cli.ContainerExecCreate(c.ctx, containerName, execConfig)
	if err != nil {
		return -1, "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to exec
	resp, err := c.cli.ContainerExecAttach(c.ctx, execID.ID, types.ExecStartCheck{
		Tty: false,
	})
	if err != nil {
		return -1, "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Read output with demuxing (stdout/stderr are multiplexed without TTY)
	var output []byte
	done := make(chan struct{})

	// Create a pipe to handle demuxed output
	pr, pw := io.Pipe()

	// Demux in a goroutine
	go func() {
		// StdCopy demultiplexes the stream
		_, err := stdcopy.StdCopy(pw, pw, resp.Reader)
		if err != nil && err != io.EOF {
			// Ignore errors, just close
		}
		pw.Close()
	}()

	// Read line by line and process
	go func() {
		scanner := bufio.NewScanner(pr)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024) // Allow large lines

		for scanner.Scan() {
			line := scanner.Text()
			output = append(output, line...)
			output = append(output, '\n')
			processor(line)
		}
		close(done)
	}()

	// Wait for completion or interrupt
	select {
	case <-sigChan:
		// User interrupted - kill the claude process inside container
		resp.Close()
		// Kill claude processes with stream-json output (YOLO mode)
		killCmd := types.ExecConfig{
			Cmd: []string{"pkill", "-f", "stream-json"},
		}
		if killExec, err := c.cli.ContainerExecCreate(c.ctx, containerName, killCmd); err == nil {
			c.cli.ContainerExecStart(c.ctx, killExec.ID, types.ExecStartCheck{})
		}
		return 130, string(output), nil
	case <-done:
		// Normal completion
	}

	// Get exit code
	inspect, err := c.cli.ContainerExecInspect(c.ctx, execID.ID)
	if err != nil {
		return -1, string(output), nil
	}

	return inspect.ExitCode, string(output), nil
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
