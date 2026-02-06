package voice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Backend identifies which transcription service to use.
type Backend string

const (
	BackendGroq   Backend = "groq"
	BackendOpenAI Backend = "openai"
)

// backendConfig holds endpoint and model defaults for each backend.
type backendConfig struct {
	Endpoint string
	Model    string
}

var backends = map[Backend]backendConfig{
	BackendGroq: {
		Endpoint: "https://api.groq.com/openai/v1/audio/transcriptions",
		Model:    "whisper-large-v3",
	},
	BackendOpenAI: {
		Endpoint: "https://api.openai.com/v1/audio/transcriptions",
		Model:    "whisper-1",
	},
}

// TranscribeResponse holds the result of a transcription request.
type TranscribeResponse struct {
	Text string `json:"text"`
}

// Transcriber sends audio data to a Whisper-compatible API and returns text.
type Transcriber struct {
	apiKey   string
	backend  Backend
	endpoint string
	model    string
	client   *http.Client
}

// Option configures a Transcriber.
type Option func(*Transcriber)

// WithEndpoint overrides the default API endpoint for the chosen backend.
func WithEndpoint(endpoint string) Option {
	return func(t *Transcriber) {
		t.endpoint = endpoint
	}
}

// WithModel overrides the default model for the chosen backend.
func WithModel(model string) Option {
	return func(t *Transcriber) {
		t.model = model
	}
}

// WithHTTPClient sets a custom HTTP client (useful for testing).
func WithHTTPClient(client *http.Client) Option {
	return func(t *Transcriber) {
		t.client = client
	}
}

// NewTranscriber creates a Transcriber for the given backend and API key.
// Returns an error if the backend is unknown or the API key is empty.
func NewTranscriber(b Backend, apiKey string, opts ...Option) (*Transcriber, error) {
	cfg, ok := backends[b]
	if !ok {
		return nil, fmt.Errorf("unknown voice backend: %q", b)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key required for voice backend %q", b)
	}

	t := &Transcriber{
		apiKey:   apiKey,
		backend:  b,
		endpoint: cfg.Endpoint,
		model:    cfg.Model,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(t)
	}
	return t, nil
}

// Transcribe sends audio bytes to the Whisper API and returns the transcribed text.
// filename is used as the form field name (e.g. "audio.ogg").
func (t *Transcriber) Transcribe(audioData []byte, filename string) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return "", fmt.Errorf("write audio data: %w", err)
	}

	if err := writer.WriteField("model", t.model); err != nil {
		return "", fmt.Errorf("write model field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequest("POST", t.endpoint, &buf)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("transcription request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("transcription failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result TranscribeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode transcription response: %w", err)
	}

	return result.Text, nil
}
