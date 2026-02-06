// Package cron provides a proactive scheduler that fires LLM-driven messages
// on cron schedules. Jobs are persisted to ~/.ubot/cron_jobs.json and
// survive restarts.
package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hkuds/ubot/internal/bus"
	"github.com/hkuds/ubot/internal/providers"
)

// Job represents a single scheduled job.
type Job struct {
	ID          string `json:"id"`
	Schedule    string `json:"schedule"`    // cron expression or "@every 5m"
	Instruction string `json:"instruction"` // prompt instruction
	Channel     string `json:"channel"`
	ChatID      string `json:"chat_id"`
}

// jobEntry wraps a Job with runtime state for the scheduler.
type jobEntry struct {
	Job    Job
	cancel context.CancelFunc
}

// Scheduler manages proactive cron jobs that call the LLM on schedule and
// publish results to the message bus.
type Scheduler struct {
	bus      *bus.MessageBus
	provider providers.Provider
	model    string

	mu      sync.RWMutex
	entries map[string]*jobEntry
	nextID  int

	persistPath string
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewScheduler creates a new Scheduler.
func NewScheduler(msgBus *bus.MessageBus, provider providers.Provider, model string) *Scheduler {
	home, _ := os.UserHomeDir()
	return &Scheduler{
		bus:         msgBus,
		provider:    provider,
		model:       model,
		entries:     make(map[string]*jobEntry),
		nextID:      1,
		persistPath: filepath.Join(home, ".ubot", "cron_jobs.json"),
	}
}

// Start loads persisted jobs and begins all cron timers.
func (s *Scheduler) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	if err := s.load(); err != nil {
		// Non-fatal: file might not exist yet.
		_ = err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, entry := range s.entries {
		s.startJobLocked(entry)
	}
	return nil
}

// Stop cancels all running jobs and the scheduler context.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// AddJob registers a new cron job and starts it. Returns the job ID.
func (s *Scheduler) AddJob(schedule, instruction, channel, chatID string) (string, error) {
	// Validate schedule by parsing it once.
	if _, err := parseDuration(schedule); err != nil {
		if _, err2 := parseCronFields(schedule); err2 != nil {
			return "", fmt.Errorf("invalid schedule %q: %w", schedule, err2)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := strconv.Itoa(s.nextID)
	s.nextID++

	entry := &jobEntry{
		Job: Job{
			ID:          id,
			Schedule:    schedule,
			Instruction: instruction,
			Channel:     channel,
			ChatID:      chatID,
		},
	}
	s.entries[id] = entry

	if s.ctx != nil {
		s.startJobLocked(entry)
	}

	if err := s.saveLocked(); err != nil {
		return id, fmt.Errorf("job added but failed to persist: %w", err)
	}

	return id, nil
}

// RemoveJob stops and removes a job by ID.
func (s *Scheduler) RemoveJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[id]
	if !ok {
		return fmt.Errorf("job %q not found", id)
	}

	if entry.cancel != nil {
		entry.cancel()
	}
	delete(s.entries, id)

	return s.saveLocked()
}

// ListJobs returns all registered jobs.
func (s *Scheduler) ListJobs() []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]Job, 0, len(s.entries))
	for _, entry := range s.entries {
		jobs = append(jobs, entry.Job)
	}
	return jobs
}

// startJobLocked launches the goroutine for a single job entry.
// Caller must hold at least an RLock on s.mu (entry is not modified here).
func (s *Scheduler) startJobLocked(entry *jobEntry) {
	jobCtx, jobCancel := context.WithCancel(s.ctx)
	entry.cancel = jobCancel

	go s.runJob(jobCtx, entry.Job)
}

// runJob is the goroutine that sleeps until the next fire time, then calls the
// LLM and publishes the result.
func (s *Scheduler) runJob(ctx context.Context, job Job) {
	// Determine if this is an interval or a cron expression.
	if d, err := parseDuration(job.Schedule); err == nil {
		s.runIntervalJob(ctx, job, d)
		return
	}

	fields, err := parseCronFields(job.Schedule)
	if err != nil {
		return // should not happen, validated in AddJob
	}
	s.runCronJob(ctx, job, fields)
}

// runIntervalJob fires at a fixed interval.
func (s *Scheduler) runIntervalJob(ctx context.Context, job Job, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fireJob(ctx, job)
		}
	}
}

// runCronJob fires at times matching cron fields.
func (s *Scheduler) runCronJob(ctx context.Context, job Job, fields cronFields) {
	for {
		now := time.Now()
		next := fields.nextAfter(now)
		delay := next.Sub(now)
		if delay < 0 {
			delay = time.Second
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.fireJob(ctx, job)
		}
	}
}

// fireJob calls the LLM with the job's instruction and publishes the result.
func (s *Scheduler) fireJob(ctx context.Context, job Job) {
	now := time.Now().Format(time.RFC1123)
	prompt := fmt.Sprintf(
		"It is now %s. Based on your instruction: %s\nWhat should you tell the user?",
		now, job.Instruction,
	)

	req := providers.ChatRequest{
		Messages: []providers.ChatMessage{
			{Role: "user", Content: prompt},
		},
		Model:       s.model,
		MaxTokens:   512,
		Temperature: 0.7,
	}

	resp, err := s.provider.Chat(ctx, req)
	if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
		return
	}

	s.bus.PublishOutbound(bus.OutboundMessage{
		Channel: job.Channel,
		ChatID:  job.ChatID,
		Content: resp.Content,
	})
}

// --- persistence ---

type persistedState struct {
	Jobs   []Job `json:"jobs"`
	NextID int   `json:"next_id"`
}

func (s *Scheduler) saveLocked() error {
	state := persistedState{
		Jobs:   make([]Job, 0, len(s.entries)),
		NextID: s.nextID,
	}
	for _, e := range s.entries {
		state.Jobs = append(state.Jobs, e.Job)
	}

	dir := filepath.Dir(s.persistPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.persistPath, data, 0o600)
}

func (s *Scheduler) load() error {
	data, err := os.ReadFile(s.persistPath)
	if err != nil {
		return err
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	for _, job := range state.Jobs {
		s.entries[job.ID] = &jobEntry{Job: job}
	}
	if state.NextID > s.nextID {
		s.nextID = state.NextID
	}
	return nil
}

// SetPersistPath overrides the default persistence path (useful for tests).
func (s *Scheduler) SetPersistPath(path string) {
	s.persistPath = path
}

// --- cron expression parsing ---

// parseDuration handles "@every 5m" style schedules.
func parseDuration(spec string) (time.Duration, error) {
	spec = strings.TrimSpace(spec)
	if !strings.HasPrefix(spec, "@every ") {
		return 0, fmt.Errorf("not an interval spec")
	}
	return time.ParseDuration(strings.TrimPrefix(spec, "@every "))
}

// cronFields represents a parsed 5-field cron expression.
// Fields: minute, hour, day-of-month, month, day-of-week.
type cronFields struct {
	minutes    []int // 0-59
	hours      []int // 0-23
	daysOfMon  []int // 1-31
	months     []int // 1-12
	daysOfWeek []int // 0-6 (0=Sunday)
}

// parseCronFields parses a standard 5-field cron expression.
func parseCronFields(spec string) (cronFields, error) {
	spec = strings.TrimSpace(spec)
	parts := strings.Fields(spec)
	if len(parts) != 5 {
		return cronFields{}, fmt.Errorf("expected 5 fields, got %d", len(parts))
	}

	minutes, err := parseField(parts[0], 0, 59)
	if err != nil {
		return cronFields{}, fmt.Errorf("minute field: %w", err)
	}
	hours, err := parseField(parts[1], 0, 23)
	if err != nil {
		return cronFields{}, fmt.Errorf("hour field: %w", err)
	}
	dom, err := parseField(parts[2], 1, 31)
	if err != nil {
		return cronFields{}, fmt.Errorf("day-of-month field: %w", err)
	}
	months, err := parseField(parts[3], 1, 12)
	if err != nil {
		return cronFields{}, fmt.Errorf("month field: %w", err)
	}
	dow, err := parseField(parts[4], 0, 6)
	if err != nil {
		return cronFields{}, fmt.Errorf("day-of-week field: %w", err)
	}

	return cronFields{
		minutes:    minutes,
		hours:      hours,
		daysOfMon:  dom,
		months:     months,
		daysOfWeek: dow,
	}, nil
}

// parseField parses a single cron field (e.g. "*/5", "1,3,5", "1-10", "*").
func parseField(field string, min, max int) ([]int, error) {
	var result []int

	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)

		// Handle step values: */5 or 1-10/2
		step := 1
		if idx := strings.Index(part, "/"); idx >= 0 {
			s, err := strconv.Atoi(part[idx+1:])
			if err != nil || s <= 0 {
				return nil, fmt.Errorf("invalid step in %q", field)
			}
			step = s
			part = part[:idx]
		}

		// Handle wildcard
		if part == "*" {
			for i := min; i <= max; i += step {
				result = append(result, i)
			}
			continue
		}

		// Handle range: 1-5
		if idx := strings.Index(part, "-"); idx >= 0 {
			lo, err := strconv.Atoi(part[:idx])
			if err != nil {
				return nil, fmt.Errorf("invalid range in %q", field)
			}
			hi, err := strconv.Atoi(part[idx+1:])
			if err != nil {
				return nil, fmt.Errorf("invalid range in %q", field)
			}
			if lo < min || hi > max || lo > hi {
				return nil, fmt.Errorf("range %d-%d out of bounds [%d,%d]", lo, hi, min, max)
			}
			for i := lo; i <= hi; i += step {
				result = append(result, i)
			}
			continue
		}

		// Single value
		val, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q", part)
		}
		if val < min || val > max {
			return nil, fmt.Errorf("value %d out of bounds [%d,%d]", val, min, max)
		}
		result = append(result, val)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("empty field")
	}
	return result, nil
}

// nextAfter returns the next time after t that matches the cron fields.
func (cf cronFields) nextAfter(t time.Time) time.Time {
	// Start from the next minute.
	t = t.Add(time.Minute).Truncate(time.Minute)

	// Try up to 4 years of minutes (safety bound).
	limit := t.Add(4 * 365 * 24 * time.Hour)
	for t.Before(limit) {
		if cf.matches(t) {
			return t
		}
		t = t.Add(time.Minute)
	}
	// Fallback: should never happen with valid cron expressions.
	return t
}

// matches returns true if t matches all cron fields.
func (cf cronFields) matches(t time.Time) bool {
	return contains(cf.minutes, t.Minute()) &&
		contains(cf.hours, t.Hour()) &&
		contains(cf.daysOfMon, t.Day()) &&
		contains(cf.months, int(t.Month())) &&
		contains(cf.daysOfWeek, int(t.Weekday()))
}

func contains(vals []int, v int) bool {
	for _, val := range vals {
		if val == v {
			return true
		}
	}
	return false
}
