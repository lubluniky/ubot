package sandbox

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Image != DefaultImage {
		t.Errorf("Image = %q, want %q", cfg.Image, DefaultImage)
	}
	if cfg.MemoryMB != DefaultMemoryMB {
		t.Errorf("MemoryMB = %d, want %d", cfg.MemoryMB, DefaultMemoryMB)
	}
	if cfg.CPUPercent != DefaultCPUPercent {
		t.Errorf("CPUPercent = %f, want %f", cfg.CPUPercent, DefaultCPUPercent)
	}
	if cfg.MaxProcesses != DefaultMaxProcesses {
		t.Errorf("MaxProcesses = %d, want %d", cfg.MaxProcesses, DefaultMaxProcesses)
	}
	if cfg.NetworkEnabled != false {
		t.Error("NetworkEnabled should be false by default")
	}
	if cfg.UseGVisor != false {
		t.Error("UseGVisor should be false by default")
	}
	if cfg.WorkDir != DefaultWorkDir {
		t.Errorf("WorkDir = %q, want %q", cfg.WorkDir, DefaultWorkDir)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, DefaultTimeout)
	}
	if cfg.MountPaths != nil {
		t.Error("MountPaths should be nil by default")
	}
}

func TestConfigWithMethods(t *testing.T) {
	cfg := DefaultConfig().
		WithImage("python:3.11-alpine").
		WithMemoryMB(256).
		WithCPUPercent(0.75).
		WithMaxProcesses(100).
		WithNetwork(true).
		WithGVisor(true).
		WithWorkDir("/app").
		WithTimeout(60 * time.Second)

	if cfg.Image != "python:3.11-alpine" {
		t.Errorf("Image = %q, want %q", cfg.Image, "python:3.11-alpine")
	}
	if cfg.MemoryMB != 256 {
		t.Errorf("MemoryMB = %d, want %d", cfg.MemoryMB, 256)
	}
	if cfg.CPUPercent != 0.75 {
		t.Errorf("CPUPercent = %f, want %f", cfg.CPUPercent, 0.75)
	}
	if cfg.MaxProcesses != 100 {
		t.Errorf("MaxProcesses = %d, want %d", cfg.MaxProcesses, 100)
	}
	if !cfg.NetworkEnabled {
		t.Error("NetworkEnabled should be true")
	}
	if !cfg.UseGVisor {
		t.Error("UseGVisor should be true")
	}
	if cfg.WorkDir != "/app" {
		t.Errorf("WorkDir = %q, want %q", cfg.WorkDir, "/app")
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 60*time.Second)
	}
}

func TestConfigAddMountPath(t *testing.T) {
	cfg := DefaultConfig().
		AddMountPath("/host/path1", "/container/path1", true).
		AddMountPath("/host/path2", "/container/path2", false)

	if len(cfg.MountPaths) != 2 {
		t.Fatalf("MountPaths length = %d, want %d", len(cfg.MountPaths), 2)
	}

	mp1 := cfg.MountPaths[0]
	if mp1.Source != "/host/path1" || mp1.Target != "/container/path1" || !mp1.ReadOnly {
		t.Errorf("MountPath[0] = %+v, want {/host/path1, /container/path1, true}", mp1)
	}

	mp2 := cfg.MountPaths[1]
	if mp2.Source != "/host/path2" || mp2.Target != "/container/path2" || mp2.ReadOnly {
		t.Errorf("MountPath[1] = %+v, want {/host/path2, /container/path2, false}", mp2)
	}
}

func TestConfigWithMountPaths(t *testing.T) {
	paths := []MountPath{
		{Source: "/src1", Target: "/dst1", ReadOnly: true},
		{Source: "/src2", Target: "/dst2", ReadOnly: false},
	}

	cfg := DefaultConfig().WithMountPaths(paths)

	if len(cfg.MountPaths) != 2 {
		t.Fatalf("MountPaths length = %d, want %d", len(cfg.MountPaths), 2)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		cfg      SandboxConfig
		expected SandboxConfig
	}{
		{
			name: "empty config gets defaults",
			cfg:  SandboxConfig{},
			expected: SandboxConfig{
				Image:        DefaultImage,
				MemoryMB:     DefaultMemoryMB,
				CPUPercent:   DefaultCPUPercent,
				MaxProcesses: DefaultMaxProcesses,
				WorkDir:      DefaultWorkDir,
				Timeout:      DefaultTimeout,
			},
		},
		{
			name: "negative values get defaults",
			cfg: SandboxConfig{
				MemoryMB:     -100,
				CPUPercent:   -0.5,
				MaxProcesses: -10,
				Timeout:      -time.Second,
			},
			expected: SandboxConfig{
				Image:        DefaultImage,
				MemoryMB:     DefaultMemoryMB,
				CPUPercent:   DefaultCPUPercent,
				MaxProcesses: DefaultMaxProcesses,
				WorkDir:      DefaultWorkDir,
				Timeout:      DefaultTimeout,
			},
		},
		{
			name: "CPU percent over 1.0 gets default",
			cfg: SandboxConfig{
				CPUPercent: 1.5,
			},
			expected: SandboxConfig{
				Image:        DefaultImage,
				MemoryMB:     DefaultMemoryMB,
				CPUPercent:   DefaultCPUPercent,
				MaxProcesses: DefaultMaxProcesses,
				WorkDir:      DefaultWorkDir,
				Timeout:      DefaultTimeout,
			},
		},
		{
			name: "valid values are preserved",
			cfg: SandboxConfig{
				Image:        "custom:image",
				MemoryMB:     512,
				CPUPercent:   0.8,
				MaxProcesses: 200,
				WorkDir:      "/custom",
				Timeout:      2 * time.Minute,
			},
			expected: SandboxConfig{
				Image:        "custom:image",
				MemoryMB:     512,
				CPUPercent:   0.8,
				MaxProcesses: 200,
				WorkDir:      "/custom",
				Timeout:      2 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			cfg.Validate()

			if cfg.Image != tt.expected.Image {
				t.Errorf("Image = %q, want %q", cfg.Image, tt.expected.Image)
			}
			if cfg.MemoryMB != tt.expected.MemoryMB {
				t.Errorf("MemoryMB = %d, want %d", cfg.MemoryMB, tt.expected.MemoryMB)
			}
			if cfg.CPUPercent != tt.expected.CPUPercent {
				t.Errorf("CPUPercent = %f, want %f", cfg.CPUPercent, tt.expected.CPUPercent)
			}
			if cfg.MaxProcesses != tt.expected.MaxProcesses {
				t.Errorf("MaxProcesses = %d, want %d", cfg.MaxProcesses, tt.expected.MaxProcesses)
			}
			if cfg.WorkDir != tt.expected.WorkDir {
				t.Errorf("WorkDir = %q, want %q", cfg.WorkDir, tt.expected.WorkDir)
			}
			if cfg.Timeout != tt.expected.Timeout {
				t.Errorf("Timeout = %v, want %v", cfg.Timeout, tt.expected.Timeout)
			}
		})
	}
}

func TestConfigImmutability(t *testing.T) {
	// Verify that With* methods don't modify the original config
	original := DefaultConfig()
	originalImage := original.Image

	modified := original.WithImage("different:image")

	if original.Image != originalImage {
		t.Errorf("original config was modified: Image = %q, want %q", original.Image, originalImage)
	}
	if modified.Image == originalImage {
		t.Errorf("modified config has original value: Image = %q", modified.Image)
	}
}
