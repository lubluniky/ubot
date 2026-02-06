package channels

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/hkuds/ubot/internal/bus"
	"github.com/hkuds/ubot/internal/config"
	"github.com/hkuds/ubot/internal/voice"
)

// Manager manages the lifecycle of communication channels.
type Manager struct {
	config   *config.Config
	bus      *bus.MessageBus
	channels map[string]Channel
	mu       sync.RWMutex
}

// NewManager creates a new channel manager.
func NewManager(cfg *config.Config, msgBus *bus.MessageBus) *Manager {
	return &Manager{
		config:   cfg,
		bus:      msgBus,
		channels: make(map[string]Channel),
	}
}

// Initialize creates enabled channels based on configuration.
// This must be called before StartAll.
func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Initialize Telegram channel if enabled
	if m.config.Channels.Telegram.Enabled {
		if m.config.Channels.Telegram.Token == "" {
			return fmt.Errorf("telegram channel enabled but token not configured")
		}

		// Build voice transcriber if a suitable API key is available
		transcriber := m.buildTranscriber()

		telegram := NewTelegramChannel(
			m.config.Channels.Telegram,
			m.bus,
			transcriber,
		)
		m.channels["telegram"] = telegram
		log.Println("Telegram channel initialized")
	}

	// Add other channels here as they are implemented
	// Example: WhatsApp, Discord, etc.

	if len(m.channels) == 0 {
		log.Println("Warning: No channels are enabled")
	}

	return nil
}

// StartAll starts all initialized channels.
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var errs []error

	for name, ch := range m.channels {
		if err := ch.Start(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to start channel %s: %w", name, err))
			continue
		}
		log.Printf("Channel %s started", name)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors starting channels: %v", errs)
	}

	return nil
}

// StopAll gracefully stops all running channels.
func (m *Manager) StopAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var errs []error

	for name, ch := range m.channels {
		if !ch.IsRunning() {
			continue
		}

		if err := ch.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop channel %s: %w", name, err))
			continue
		}
		log.Printf("Channel %s stopped", name)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping channels: %v", errs)
	}

	return nil
}

// GetChannel returns a channel by name, or nil if not found.
func (m *Manager) GetChannel(name string) Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.channels[name]
}

// ListChannels returns a sorted list of all channel names.
func (m *Manager) ListChannels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.channels))
	for name := range m.channels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// RegisterChannel adds a custom channel to the manager.
// This allows for dynamic channel registration beyond config-based initialization.
func (m *Manager) RegisterChannel(ch Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ch == nil {
		return fmt.Errorf("cannot register nil channel")
	}

	name := ch.Name()
	if _, exists := m.channels[name]; exists {
		return fmt.Errorf("channel %s already registered", name)
	}

	m.channels[name] = ch
	return nil
}

// UnregisterChannel removes a channel from the manager.
// The channel must be stopped before unregistering.
func (m *Manager) UnregisterChannel(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, exists := m.channels[name]
	if !exists {
		return fmt.Errorf("channel %s not found", name)
	}

	if ch.IsRunning() {
		return fmt.Errorf("cannot unregister running channel %s", name)
	}

	delete(m.channels, name)
	return nil
}

// RunningChannels returns a list of currently running channel names.
func (m *Manager) RunningChannels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var running []string
	for name, ch := range m.channels {
		if ch.IsRunning() {
			running = append(running, name)
		}
	}
	sort.Strings(running)
	return running
}

// ChannelCount returns the total number of registered channels.
func (m *Manager) ChannelCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.channels)
}

// buildTranscriber creates a voice Transcriber based on config.
// Returns nil (no voice support) when no suitable API key is available.
func (m *Manager) buildTranscriber() *voice.Transcriber {
	voiceCfg := m.config.Tools.Voice

	// Determine backend and API key
	backend := voice.Backend(voiceCfg.Backend)
	var apiKey string

	switch backend {
	case voice.BackendOpenAI:
		apiKey = m.config.Providers.OpenAI.APIKey
	case voice.BackendGroq:
		apiKey = m.config.Providers.Groq.APIKey
	default:
		// Auto-detect: prefer Groq, fall back to OpenAI
		if m.config.Providers.Groq.APIKey != "" {
			backend = voice.BackendGroq
			apiKey = m.config.Providers.Groq.APIKey
		} else if m.config.Providers.OpenAI.APIKey != "" {
			backend = voice.BackendOpenAI
			apiKey = m.config.Providers.OpenAI.APIKey
		}
	}

	if apiKey == "" {
		log.Println("Voice transcription disabled: no API key for backend")
		return nil
	}

	var opts []voice.Option
	if voiceCfg.Model != "" {
		opts = append(opts, voice.WithModel(voiceCfg.Model))
	}

	t, err := voice.NewTranscriber(backend, apiKey, opts...)
	if err != nil {
		log.Printf("Failed to create voice transcriber: %v", err)
		return nil
	}
	log.Printf("Voice transcription enabled (backend=%s)", backend)
	return t
}
