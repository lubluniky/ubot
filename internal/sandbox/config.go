// Package sandbox provides a secure container-based execution environment.
package sandbox

import "time"

// Default configuration values.
const (
	DefaultImage        = "alpine:latest"
	DefaultMemoryMB     = 128
	DefaultCPUPercent   = 0.5
	DefaultMaxProcesses = 50
	DefaultTimeout      = 30 * time.Second
	DefaultWorkDir      = "/workspace"
)

// SandboxConfig holds configuration for the sandbox environment.
type SandboxConfig struct {
	// Image is the container image to use.
	// Default: alpine:latest
	Image string

	// MemoryMB is the memory limit in megabytes.
	// Default: 128
	MemoryMB int64

	// CPUPercent is the CPU limit as a fraction (0.0-1.0).
	// Default: 0.5 (50% of one CPU)
	CPUPercent float64

	// MaxProcesses is the maximum number of PIDs allowed in the container.
	// Default: 50
	MaxProcesses int64

	// NetworkEnabled allows network access if true.
	// Default: false (isolated)
	NetworkEnabled bool

	// UseGVisor enables gVisor runtime (runsc) if available.
	// Provides additional kernel-level isolation.
	// Default: false
	UseGVisor bool

	// WorkDir is the working directory inside the container.
	// Default: /workspace
	WorkDir string

	// Timeout is the maximum duration for command execution.
	// Default: 30s
	Timeout time.Duration

	// MountPaths specifies paths to mount into the container.
	MountPaths []MountPath
}

// MountPath defines a bind mount configuration.
type MountPath struct {
	// Source is the path on the host.
	Source string

	// Target is the path inside the container.
	Target string

	// ReadOnly makes the mount read-only if true.
	ReadOnly bool
}

// DefaultConfig returns a SandboxConfig with sensible defaults.
func DefaultConfig() SandboxConfig {
	return SandboxConfig{
		Image:          DefaultImage,
		MemoryMB:       DefaultMemoryMB,
		CPUPercent:     DefaultCPUPercent,
		MaxProcesses:   DefaultMaxProcesses,
		NetworkEnabled: false,
		UseGVisor:      false,
		WorkDir:        DefaultWorkDir,
		Timeout:        DefaultTimeout,
		MountPaths:     nil,
	}
}

// WithImage returns a copy of the config with the specified image.
func (c SandboxConfig) WithImage(image string) SandboxConfig {
	c.Image = image
	return c
}

// WithMemoryMB returns a copy of the config with the specified memory limit.
func (c SandboxConfig) WithMemoryMB(mb int64) SandboxConfig {
	c.MemoryMB = mb
	return c
}

// WithCPUPercent returns a copy of the config with the specified CPU limit.
func (c SandboxConfig) WithCPUPercent(pct float64) SandboxConfig {
	c.CPUPercent = pct
	return c
}

// WithMaxProcesses returns a copy of the config with the specified PID limit.
func (c SandboxConfig) WithMaxProcesses(max int64) SandboxConfig {
	c.MaxProcesses = max
	return c
}

// WithNetwork returns a copy of the config with network enabled or disabled.
func (c SandboxConfig) WithNetwork(enabled bool) SandboxConfig {
	c.NetworkEnabled = enabled
	return c
}

// WithGVisor returns a copy of the config with gVisor enabled or disabled.
func (c SandboxConfig) WithGVisor(enabled bool) SandboxConfig {
	c.UseGVisor = enabled
	return c
}

// WithWorkDir returns a copy of the config with the specified working directory.
func (c SandboxConfig) WithWorkDir(dir string) SandboxConfig {
	c.WorkDir = dir
	return c
}

// WithTimeout returns a copy of the config with the specified timeout.
func (c SandboxConfig) WithTimeout(timeout time.Duration) SandboxConfig {
	c.Timeout = timeout
	return c
}

// WithMountPaths returns a copy of the config with the specified mount paths.
func (c SandboxConfig) WithMountPaths(paths []MountPath) SandboxConfig {
	c.MountPaths = paths
	return c
}

// AddMountPath returns a copy of the config with an additional mount path.
func (c SandboxConfig) AddMountPath(source, target string, readOnly bool) SandboxConfig {
	c.MountPaths = append(c.MountPaths, MountPath{
		Source:   source,
		Target:   target,
		ReadOnly: readOnly,
	})
	return c
}

// Validate checks if the configuration is valid and applies defaults.
func (c *SandboxConfig) Validate() {
	if c.Image == "" {
		c.Image = DefaultImage
	}
	if c.MemoryMB <= 0 {
		c.MemoryMB = DefaultMemoryMB
	}
	if c.CPUPercent <= 0 || c.CPUPercent > 1.0 {
		c.CPUPercent = DefaultCPUPercent
	}
	if c.MaxProcesses <= 0 {
		c.MaxProcesses = DefaultMaxProcesses
	}
	if c.WorkDir == "" {
		c.WorkDir = DefaultWorkDir
	}
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
}
