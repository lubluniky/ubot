// Package sandbox provides a secure container-based execution environment.
package sandbox

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Pool manages a pool of pre-warmed sandbox containers for faster execution.
type Pool struct {
	config    SandboxConfig
	available chan *Sandbox
	maxSize   int
	created   atomic.Int32
	mu        sync.Mutex
	closed    atomic.Bool
}

// NewPool creates a new sandbox pool with the given configuration and maximum size.
// The pool will pre-warm sandboxes up to maxSize in the background.
func NewPool(cfg SandboxConfig, maxSize int) *Pool {
	if maxSize <= 0 {
		maxSize = 1
	}

	p := &Pool{
		config:    cfg,
		available: make(chan *Sandbox, maxSize),
		maxSize:   maxSize,
	}

	return p
}

// Warmup pre-warms the pool with the specified number of sandboxes.
// This is useful to avoid cold-start latency on the first few requests.
func (p *Pool) Warmup(ctx context.Context, count int) error {
	if count <= 0 {
		return nil
	}
	if count > p.maxSize {
		count = p.maxSize
	}

	var wg sync.WaitGroup
	errCh := make(chan error, count)

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sandbox, err := p.createSandbox(ctx)
			if err != nil {
				errCh <- err
				return
			}
			// Try to add to pool, close if full
			select {
			case p.available <- sandbox:
				// Added to pool
			default:
				// Pool is full, close this sandbox
				_ = sandbox.Close()
				p.created.Add(-1)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	// Return first error if any
	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

// Acquire gets a sandbox from the pool.
// If no sandbox is available, it creates a new one (up to maxSize).
// The caller must call Release() when done with the sandbox.
func (p *Pool) Acquire(ctx context.Context) (*Sandbox, error) {
	if p.closed.Load() {
		return nil, fmt.Errorf("pool is closed")
	}

	// Try to get from available pool first
	select {
	case sandbox := <-p.available:
		if sandbox.IsRunning() {
			return sandbox, nil
		}
		// Sandbox stopped unexpectedly, clean up and create new one
		_ = sandbox.Close()
		p.created.Add(-1)
	default:
		// No sandbox available
	}

	// Check if we can create a new one
	p.mu.Lock()
	currentCount := int(p.created.Load())
	if currentCount >= p.maxSize {
		p.mu.Unlock()
		// Wait for one to become available
		select {
		case sandbox := <-p.available:
			if sandbox.IsRunning() {
				return sandbox, nil
			}
			// Sandbox stopped, try to create new one
			_ = sandbox.Close()
			p.created.Add(-1)
			return p.createSandbox(ctx)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	p.mu.Unlock()

	return p.createSandbox(ctx)
}

// createSandbox creates a new sandbox and starts it.
func (p *Pool) createSandbox(ctx context.Context) (*Sandbox, error) {
	sandbox, err := New(p.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox: %w", err)
	}

	if err := sandbox.Start(ctx); err != nil {
		_ = sandbox.Close()
		return nil, fmt.Errorf("failed to start sandbox: %w", err)
	}

	p.created.Add(1)
	return sandbox, nil
}

// Release returns a sandbox to the pool for reuse.
// If the sandbox is not running or the pool is full, it will be closed.
func (p *Pool) Release(s *Sandbox) {
	if s == nil {
		return
	}

	// If pool is closed or sandbox not running, just close it
	if p.closed.Load() || !s.IsRunning() {
		_ = s.Close()
		p.created.Add(-1)
		return
	}

	// Try to return to pool
	select {
	case p.available <- s:
		// Successfully returned to pool
	default:
		// Pool is full, close the sandbox
		_ = s.Close()
		p.created.Add(-1)
	}
}

// ReleaseWithReset returns a sandbox to the pool after resetting it.
// This ensures a clean state for the next user.
func (p *Pool) ReleaseWithReset(ctx context.Context, s *Sandbox) {
	if s == nil {
		return
	}

	// If pool is closed, just close the sandbox
	if p.closed.Load() {
		_ = s.Close()
		p.created.Add(-1)
		return
	}

	// Reset the sandbox
	if err := s.Reset(ctx); err != nil {
		// Reset failed, close and don't return to pool
		_ = s.Close()
		p.created.Add(-1)
		return
	}

	// Return to pool
	p.Release(s)
}

// Size returns the number of sandboxes currently available in the pool.
func (p *Pool) Size() int {
	return len(p.available)
}

// Created returns the total number of sandboxes created by this pool.
func (p *Pool) Created() int {
	return int(p.created.Load())
}

// MaxSize returns the maximum pool size.
func (p *Pool) MaxSize() int {
	return p.maxSize
}

// Close closes all sandboxes in the pool and releases resources.
func (p *Pool) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	// Close all available sandboxes
	close(p.available)
	for sandbox := range p.available {
		_ = sandbox.Close()
		p.created.Add(-1)
	}

	return nil
}

// Config returns a copy of the pool's sandbox configuration.
func (p *Pool) Config() SandboxConfig {
	return p.config
}

// PoolStats holds statistics about the pool.
type PoolStats struct {
	Available int
	Created   int
	MaxSize   int
	Closed    bool
}

// Stats returns current pool statistics.
func (p *Pool) Stats() PoolStats {
	return PoolStats{
		Available: len(p.available),
		Created:   int(p.created.Load()),
		MaxSize:   p.maxSize,
		Closed:    p.closed.Load(),
	}
}

// ExecuteInPool acquires a sandbox, executes the command, and releases the sandbox.
// This is a convenience method for one-off command execution.
func (p *Pool) ExecuteInPool(ctx context.Context, cmd []string) (stdout, stderr string, exitCode int, err error) {
	sandbox, err := p.Acquire(ctx)
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to acquire sandbox: %w", err)
	}
	defer p.Release(sandbox)

	return sandbox.Execute(ctx, cmd)
}

// ExecuteShellInPool acquires a sandbox, executes the shell command, and releases the sandbox.
// This is a convenience method for one-off shell command execution.
func (p *Pool) ExecuteShellInPool(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	sandbox, err := p.Acquire(ctx)
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to acquire sandbox: %w", err)
	}
	defer p.Release(sandbox)

	return sandbox.ExecuteShell(ctx, command)
}

// HealthCheck verifies that the pool can create and use sandboxes.
func (p *Pool) HealthCheck(ctx context.Context) error {
	if p.closed.Load() {
		return fmt.Errorf("pool is closed")
	}

	// Create a temporary sandbox to verify Docker is working
	sandbox, err := New(p.config)
	if err != nil {
		return fmt.Errorf("failed to create sandbox: %w", err)
	}
	defer sandbox.Close()

	// Check if Docker daemon is accessible
	if err := sandbox.Ping(ctx); err != nil {
		return fmt.Errorf("docker daemon not accessible: %w", err)
	}

	return nil
}

// WarmupAsync starts warming up the pool in the background.
// It returns immediately and the warming happens asynchronously.
func (p *Pool) WarmupAsync(count int) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_ = p.Warmup(ctx, count)
	}()
}
