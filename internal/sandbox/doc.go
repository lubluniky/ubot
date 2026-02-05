// Package sandbox provides a secure container-based execution environment for uBot.
//
// The sandbox package offers isolated command execution using Docker containers
// with extensive security hardening. It provides:
//
// # Sandbox Container
//
// The Sandbox type creates secure Docker containers with:
//   - ReadonlyRootfs: Prevents modification of the container filesystem
//   - NetworkMode "none": Network isolation when NetworkEnabled is false
//   - CapDrop ALL: Drops all Linux capabilities
//   - SecurityOpt "no-new-privileges": Prevents privilege escalation
//   - Memory, CPU, and PID limits: Resource constraints
//   - Tmpfs mounts: Writable areas without persistence
//   - AutoRemove: Automatic cleanup when stopped
//   - User "nobody": Unprivileged execution
//
// Optional gVisor (runsc) runtime support provides additional kernel-level isolation.
//
// # Command Guard
//
// The GuardCommand function validates commands before execution, blocking
// dangerous patterns including:
//   - Recursive file deletion (rm -rf, del /s)
//   - Disk formatting commands (format, mkfs, fdisk)
//   - System shutdown/reboot commands
//   - Fork bombs and resource exhaustion attacks
//   - Direct writes to disk devices
//
// # Sandbox Pool
//
// The Pool type maintains a pool of pre-warmed containers for faster execution.
// It provides:
//   - Pre-warming capability to avoid cold-start latency
//   - Automatic container lifecycle management
//   - Thread-safe acquire/release operations
//
// # Fallback Executor
//
// The LocalExecutor provides command execution when Docker is not available.
// It uses os/exec directly but still applies command guard checks.
//
// # Usage
//
// Basic sandbox usage:
//
//	cfg := sandbox.DefaultConfig().
//	    WithImage("python:3.11-alpine").
//	    WithTimeout(60 * time.Second)
//
//	sb, err := sandbox.New(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer sb.Close()
//
//	if err := sb.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	stdout, stderr, exitCode, err := sb.ExecuteShell(ctx, "echo hello")
//
// Using the pool for multiple executions:
//
//	pool := sandbox.NewPool(sandbox.DefaultConfig(), 5)
//	defer pool.Close()
//
//	stdout, stderr, exitCode, err := pool.ExecuteShellInPool(ctx, "echo hello")
//
// Automatic executor selection:
//
//	executor, err := sandbox.NewExecutor(sandbox.DefaultConfig())
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	stdout, stderr, exitCode, err := executor.ExecuteShell(ctx, "echo hello")
package sandbox
