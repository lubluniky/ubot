package cron

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hkuds/ubot/internal/bus"
	"github.com/hkuds/ubot/internal/providers"
)

// mockProvider implements providers.Provider for testing.
type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Name() string        { return "mock" }
func (m *mockProvider) DefaultModel() string { return "mock-model" }
func (m *mockProvider) Chat(_ context.Context, _ providers.ChatRequest) (*providers.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &providers.ChatResponse{
		Content:      m.response,
		FinishReason: "stop",
	}, nil
}

func newTestScheduler(t *testing.T, provider providers.Provider) (*Scheduler, *bus.MessageBus) {
	t.Helper()
	msgBus := bus.NewMessageBus(10)
	s := NewScheduler(msgBus, provider, "test-model")
	s.SetPersistPath(filepath.Join(t.TempDir(), "cron_jobs.json"))
	return s, msgBus
}

func TestAddRemoveListJobs(t *testing.T) {
	s, _ := newTestScheduler(t, &mockProvider{response: "hello"})

	// Initially empty.
	if jobs := s.ListJobs(); len(jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(jobs))
	}

	// Add a job.
	id1, err := s.AddJob("@every 1h", "check weather", "telegram", "123")
	if err != nil {
		t.Fatalf("AddJob: %v", err)
	}
	if id1 == "" {
		t.Fatal("expected non-empty job ID")
	}

	// Add another.
	id2, err := s.AddJob("*/5 * * * *", "check stocks", "cli", "456")
	if err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	jobs := s.ListJobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Remove first job.
	if err := s.RemoveJob(id1); err != nil {
		t.Fatalf("RemoveJob: %v", err)
	}

	jobs = s.ListJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].ID != id2 {
		t.Fatalf("expected job ID %s, got %s", id2, jobs[0].ID)
	}

	// Remove non-existent job.
	if err := s.RemoveJob("999"); err == nil {
		t.Fatal("expected error removing non-existent job")
	}
}

func TestInvalidSchedule(t *testing.T) {
	s, _ := newTestScheduler(t, &mockProvider{response: "hello"})

	_, err := s.AddJob("not a schedule", "test", "cli", "1")
	if err == nil {
		t.Fatal("expected error for invalid schedule")
	}

	_, err = s.AddJob("1 2 3", "test", "cli", "1")
	if err == nil {
		t.Fatal("expected error for incomplete cron expression")
	}
}

func TestPersistence(t *testing.T) {
	persistPath := filepath.Join(t.TempDir(), "cron_jobs.json")

	provider := &mockProvider{response: "test"}
	msgBus := bus.NewMessageBus(10)

	// Create scheduler and add jobs.
	s1 := NewScheduler(msgBus, provider, "test-model")
	s1.SetPersistPath(persistPath)

	id1, err := s1.AddJob("@every 10m", "reminder 1", "telegram", "100")
	if err != nil {
		t.Fatalf("AddJob: %v", err)
	}
	_, err = s1.AddJob("0 9 * * *", "morning check", "cli", "200")
	if err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	// Verify file exists.
	if _, err := os.Stat(persistPath); os.IsNotExist(err) {
		t.Fatal("persist file was not created")
	}

	// Create a new scheduler and load from the same file.
	s2 := NewScheduler(msgBus, provider, "test-model")
	s2.SetPersistPath(persistPath)
	if err := s2.load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	jobs := s2.ListJobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs after reload, got %d", len(jobs))
	}

	// IDs should be preserved.
	found := false
	for _, j := range jobs {
		if j.ID == id1 && j.Instruction == "reminder 1" {
			found = true
		}
	}
	if !found {
		t.Fatal("job with id1 not found after reload")
	}
}

func TestJobFiring(t *testing.T) {
	provider := &mockProvider{response: "Good morning! Time to check the weather."}
	s, msgBus := newTestScheduler(t, provider)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()

	// Add a very frequent job.
	_, err := s.AddJob("@every 100ms", "test firing", "telegram", "42")
	if err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	// Wait for at least one fire.
	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for job to fire")
		default:
			if msgBus.OutboundSize() > 0 {
				msg := msgBus.ConsumeOutbound()
				if msg.Channel != "telegram" {
					t.Fatalf("expected channel 'telegram', got %q", msg.Channel)
				}
				if msg.ChatID != "42" {
					t.Fatalf("expected chatID '42', got %q", msg.ChatID)
				}
				if msg.Content == "" {
					t.Fatal("expected non-empty content")
				}
				return // success
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestCronFieldsParsing(t *testing.T) {
	tests := []struct {
		spec    string
		wantErr bool
	}{
		{"* * * * *", false},
		{"0 9 * * 1-5", false},
		{"*/15 * * * *", false},
		{"0 0 1 1 *", false},
		{"5,10,15 * * * *", false},
		{"bad", true},
		{"60 * * * *", true},     // minute out of range
		{"* 25 * * *", true},     // hour out of range
		{"* * 0 * *", true},      // day-of-month out of range
		{"* * * 13 *", true},     // month out of range
		{"* * * * 7", true},      // day-of-week out of range
		{"* * * *", true},        // too few fields
		{"* * * * * *", true},    // too many fields
	}

	for _, tc := range tests {
		_, err := parseCronFields(tc.spec)
		if tc.wantErr && err == nil {
			t.Errorf("parseCronFields(%q): expected error, got nil", tc.spec)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("parseCronFields(%q): unexpected error: %v", tc.spec, err)
		}
	}
}

func TestNextAfter(t *testing.T) {
	// "0 9 * * *" means every day at 09:00.
	fields, err := parseCronFields("0 9 * * *")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// If it's 08:30, next should be 09:00 same day.
	base := time.Date(2025, 6, 15, 8, 30, 0, 0, time.Local)
	next := fields.nextAfter(base)
	if next.Hour() != 9 || next.Minute() != 0 {
		t.Errorf("expected 09:00, got %s", next.Format("15:04"))
	}
	if next.Day() != 15 {
		t.Errorf("expected day 15, got %d", next.Day())
	}

	// If it's 09:30, next should be 09:00 next day.
	base2 := time.Date(2025, 6, 15, 9, 30, 0, 0, time.Local)
	next2 := fields.nextAfter(base2)
	if next2.Hour() != 9 || next2.Minute() != 0 {
		t.Errorf("expected 09:00, got %s", next2.Format("15:04"))
	}
	if next2.Day() != 16 {
		t.Errorf("expected day 16, got %d", next2.Day())
	}
}

func TestParseDuration(t *testing.T) {
	d, err := parseDuration("@every 5m")
	if err != nil {
		t.Fatalf("parseDuration: %v", err)
	}
	if d != 5*time.Minute {
		t.Errorf("expected 5m, got %v", d)
	}

	d, err = parseDuration("@every 1h30m")
	if err != nil {
		t.Fatalf("parseDuration: %v", err)
	}
	if d != 90*time.Minute {
		t.Errorf("expected 1h30m, got %v", d)
	}

	_, err = parseDuration("not an interval")
	if err == nil {
		t.Fatal("expected error for non-interval spec")
	}
}
