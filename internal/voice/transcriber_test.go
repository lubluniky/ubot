package voice

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewTranscriber_ValidBackends(t *testing.T) {
	for _, b := range []Backend{BackendGroq, BackendOpenAI} {
		tr, err := NewTranscriber(b, "test-key")
		if err != nil {
			t.Fatalf("NewTranscriber(%q): unexpected error: %v", b, err)
		}
		if tr.backend != b {
			t.Errorf("expected backend %q, got %q", b, tr.backend)
		}
	}
}

func TestNewTranscriber_UnknownBackend(t *testing.T) {
	_, err := NewTranscriber("unknown", "key")
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
	if !strings.Contains(err.Error(), "unknown voice backend") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewTranscriber_EmptyAPIKey(t *testing.T) {
	_, err := NewTranscriber(BackendGroq, "")
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if !strings.Contains(err.Error(), "API key required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewTranscriber_Options(t *testing.T) {
	tr, err := NewTranscriber(BackendGroq, "key",
		WithEndpoint("http://custom/endpoint"),
		WithModel("custom-model"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.endpoint != "http://custom/endpoint" {
		t.Errorf("expected custom endpoint, got %q", tr.endpoint)
	}
	if tr.model != "custom-model" {
		t.Errorf("expected custom model, got %q", tr.model)
	}
}

func TestTranscribe_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and auth header
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("expected 'Bearer test-key', got %q", auth)
		}

		// Verify content type is multipart
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("expected multipart/form-data content type, got %q", ct)
		}

		// Parse multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse multipart form: %v", err)
		}

		// Verify model field
		model := r.FormValue("model")
		if model != "whisper-large-v3" {
			t.Errorf("expected model whisper-large-v3, got %q", model)
		}

		// Verify file field exists
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("get form file: %v", err)
		}
		defer file.Close()

		if header.Filename != "audio.ogg" {
			t.Errorf("expected filename audio.ogg, got %q", header.Filename)
		}

		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if string(data) != "fake-audio-data" {
			t.Errorf("unexpected file content: %q", string(data))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TranscribeResponse{Text: "hello world"})
	}))
	defer server.Close()

	tr, err := NewTranscriber(BackendGroq, "test-key",
		WithEndpoint(server.URL),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text, err := tr.Transcribe([]byte("fake-audio-data"), "audio.ogg")
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if text != "hello world" {
		t.Errorf("expected 'hello world', got %q", text)
	}
}

func TestTranscribe_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer server.Close()

	tr, err := NewTranscriber(BackendGroq, "bad-key",
		WithEndpoint(server.URL),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = tr.Transcribe([]byte("audio"), "test.ogg")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "status 401") {
		t.Errorf("expected status 401 in error, got: %v", err)
	}
}

func TestTranscribe_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	tr, err := NewTranscriber(BackendGroq, "key",
		WithEndpoint(server.URL),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = tr.Transcribe([]byte("audio"), "test.ogg")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected decode error, got: %v", err)
	}
}

func TestTranscribe_OpenAIBackend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		model := r.FormValue("model")
		if model != "whisper-1" {
			t.Errorf("expected OpenAI model whisper-1, got %q", model)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TranscribeResponse{Text: "openai result"})
	}))
	defer server.Close()

	tr, err := NewTranscriber(BackendOpenAI, "openai-key",
		WithEndpoint(server.URL),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text, err := tr.Transcribe([]byte("audio"), "voice.mp3")
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if text != "openai result" {
		t.Errorf("expected 'openai result', got %q", text)
	}
}

func TestBackendDefaults(t *testing.T) {
	groq, _ := NewTranscriber(BackendGroq, "k")
	if groq.model != "whisper-large-v3" {
		t.Errorf("groq default model: got %q, want whisper-large-v3", groq.model)
	}
	if groq.endpoint != "https://api.groq.com/openai/v1/audio/transcriptions" {
		t.Errorf("groq default endpoint: got %q", groq.endpoint)
	}

	openai, _ := NewTranscriber(BackendOpenAI, "k")
	if openai.model != "whisper-1" {
		t.Errorf("openai default model: got %q, want whisper-1", openai.model)
	}
	if openai.endpoint != "https://api.openai.com/v1/audio/transcriptions" {
		t.Errorf("openai default endpoint: got %q", openai.endpoint)
	}
}
