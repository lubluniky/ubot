// Package sandbox provides a secure container-based execution environment.
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// Sandbox represents a secure container execution environment.
type Sandbox struct {
	config      SandboxConfig
	client      *client.Client
	containerID string
	running     bool
	mu          sync.RWMutex
}

// New creates a new Sandbox with the given configuration.
// The sandbox is not started until Start() is called.
func New(cfg SandboxConfig) (*Sandbox, error) {
	// Validate and apply defaults
	cfg.Validate()

	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Sandbox{
		config: cfg,
		client: cli,
	}, nil
}

// NewWithClient creates a new Sandbox with an existing Docker client.
// Useful for testing or when sharing a client across multiple sandboxes.
func NewWithClient(cfg SandboxConfig, cli *client.Client) (*Sandbox, error) {
	if cli == nil {
		return nil, fmt.Errorf("Docker client cannot be nil")
	}

	cfg.Validate()

	return &Sandbox{
		config: cfg,
		client: cli,
	}, nil
}

// Start creates and starts the sandbox container.
func (s *Sandbox) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("sandbox is already running")
	}

	// Pull the image if needed
	if err := s.ensureImage(ctx); err != nil {
		return fmt.Errorf("failed to ensure image: %w", err)
	}

	// Create container configuration
	containerCfg, hostCfg, networkCfg := s.buildContainerConfig()

	// Create the container
	resp, err := s.client.ContainerCreate(ctx, containerCfg, hostCfg, networkCfg, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	s.containerID = resp.ID

	// Start the container
	if err := s.client.ContainerStart(ctx, s.containerID, container.StartOptions{}); err != nil {
		// Clean up the created container
		_ = s.client.ContainerRemove(ctx, s.containerID, container.RemoveOptions{Force: true})
		s.containerID = ""
		return fmt.Errorf("failed to start container: %w", err)
	}

	s.running = true
	return nil
}

// ensureImage pulls the image if it doesn't exist locally.
func (s *Sandbox) ensureImage(ctx context.Context) error {
	// Check if image exists locally
	_, _, err := s.client.ImageInspectWithRaw(ctx, s.config.Image)
	if err == nil {
		return nil // Image exists
	}

	// Pull the image
	reader, err := s.client.ImagePull(ctx, s.config.Image, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", s.config.Image, err)
	}
	defer reader.Close()

	// Consume the reader to complete the pull
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", s.config.Image, err)
	}

	return nil
}

// buildContainerConfig creates the container, host, and network configurations.
func (s *Sandbox) buildContainerConfig() (*container.Config, *container.HostConfig, *network.NetworkingConfig) {
	// Container configuration
	containerCfg := &container.Config{
		Image:      s.config.Image,
		WorkingDir: s.config.WorkDir,
		User:       "nobody",
		Tty:        false,
		// Keep container running with a sleep command
		Cmd: []string{"sleep", "infinity"},
	}

	// Host configuration with security settings
	hostCfg := &container.HostConfig{
		// Read-only root filesystem for security
		ReadonlyRootfs: true,

		// Drop all capabilities
		CapDrop: []string{"ALL"},

		// Prevent privilege escalation
		SecurityOpt: []string{"no-new-privileges:true"},

		// Auto-remove container when stopped
		AutoRemove: true,

		// Resource limits
		Resources: container.Resources{
			// Memory limit in bytes
			Memory: s.config.MemoryMB * 1024 * 1024,
			// Memory + swap (same as memory to disable swap)
			MemorySwap: s.config.MemoryMB * 1024 * 1024,
			// CPU quota (100000 = 100% of one CPU)
			CPUQuota: int64(s.config.CPUPercent * 100000),
			CPUPeriod: 100000,
			// PID limit
			PidsLimit: &s.config.MaxProcesses,
		},

		// Tmpfs mounts for writable directories
		Tmpfs: map[string]string{
			"/tmp":          "rw,noexec,nosuid,size=64m",
			s.config.WorkDir: "rw,noexec,nosuid,size=64m",
		},
	}

	// Network mode
	if !s.config.NetworkEnabled {
		hostCfg.NetworkMode = "none"
	}

	// Use gVisor runtime if enabled
	if s.config.UseGVisor {
		hostCfg.Runtime = "runsc"
	}

	// Add mount paths
	for _, mp := range s.config.MountPaths {
		hostCfg.Mounts = append(hostCfg.Mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   mp.Source,
			Target:   mp.Target,
			ReadOnly: mp.ReadOnly,
		})
	}

	// Network configuration
	networkCfg := &network.NetworkingConfig{}

	return containerCfg, hostCfg, networkCfg
}

// Stop stops the sandbox container.
func (s *Sandbox) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil // Already stopped
	}

	// Stop the container with a timeout
	timeout := 10
	if err := s.client.ContainerStop(ctx, s.containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		// Force remove if stop fails
		_ = s.client.ContainerRemove(ctx, s.containerID, container.RemoveOptions{Force: true})
	}

	s.running = false
	s.containerID = ""
	return nil
}

// Execute runs a command inside the sandbox and returns the output.
func (s *Sandbox) Execute(ctx context.Context, cmd []string) (stdout, stderr string, exitCode int, err error) {
	s.mu.RLock()
	if !s.running {
		s.mu.RUnlock()
		return "", "", -1, fmt.Errorf("sandbox is not running")
	}
	containerID := s.containerID
	s.mu.RUnlock()

	// Guard check on the command
	if len(cmd) > 0 {
		fullCmd := ""
		for _, c := range cmd {
			fullCmd += c + " "
		}
		if reason := GuardCommand(fullCmd); reason != "" {
			return "", "", -1, fmt.Errorf("command guard: %s", reason)
		}
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	// Create exec configuration
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   s.config.WorkDir,
		User:         "nobody",
	}

	// Create the exec instance
	execResp, err := s.client.ContainerExecCreate(execCtx, containerID, execConfig)
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to the exec instance
	attachResp, err := s.client.ContainerExecAttach(execCtx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	// Read stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	outputDone := make(chan error, 1)

	go func() {
		_, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, attachResp.Reader)
		outputDone <- err
	}()

	// Wait for output or timeout
	select {
	case err := <-outputDone:
		if err != nil {
			return stdoutBuf.String(), stderrBuf.String(), -1, fmt.Errorf("failed to read output: %w", err)
		}
	case <-execCtx.Done():
		return stdoutBuf.String(), stderrBuf.String(), -1, fmt.Errorf("command timed out after %v", s.config.Timeout)
	}

	// Get the exit code
	inspectResp, err := s.client.ContainerExecInspect(execCtx, execResp.ID)
	if err != nil {
		return stdoutBuf.String(), stderrBuf.String(), -1, fmt.Errorf("failed to inspect exec: %w", err)
	}

	return stdoutBuf.String(), stderrBuf.String(), inspectResp.ExitCode, nil
}

// ExecuteShell runs a shell command inside the sandbox.
// This is a convenience method that wraps the command in a shell invocation.
func (s *Sandbox) ExecuteShell(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	return s.Execute(ctx, []string{"sh", "-c", command})
}

// IsRunning returns true if the sandbox container is running.
func (s *Sandbox) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// ContainerID returns the ID of the running container, or empty string if not running.
func (s *Sandbox) ContainerID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.containerID
}

// Close stops the sandbox and releases resources.
func (s *Sandbox) Close() error {
	// Stop the container if running
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Stop(ctx); err != nil {
		// Log but don't fail on stop errors
		_ = err
	}

	// Close the Docker client
	if s.client != nil {
		if err := s.client.Close(); err != nil {
			return fmt.Errorf("failed to close Docker client: %w", err)
		}
	}

	return nil
}

// Config returns a copy of the sandbox configuration.
func (s *Sandbox) Config() SandboxConfig {
	return s.config
}

// Reset stops and restarts the sandbox with a fresh container.
func (s *Sandbox) Reset(ctx context.Context) error {
	if err := s.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop sandbox: %w", err)
	}
	return s.Start(ctx)
}

// CopyToContainer copies data from a reader to a path inside the container.
func (s *Sandbox) CopyToContainer(ctx context.Context, dstPath string, content io.Reader) error {
	s.mu.RLock()
	if !s.running {
		s.mu.RUnlock()
		return fmt.Errorf("sandbox is not running")
	}
	containerID := s.containerID
	s.mu.RUnlock()

	return s.client.CopyToContainer(ctx, containerID, dstPath, content, container.CopyToContainerOptions{})
}

// Ping checks if the Docker daemon is accessible.
func (s *Sandbox) Ping(ctx context.Context) error {
	_, err := s.client.Ping(ctx)
	return err
}
