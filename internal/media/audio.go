package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

const whisperEndpoint = "https://api.openai.com/v1/audio/transcriptions"

// Transcriber transcribes an audio file to text.
type Transcriber interface {
	Transcribe(ctx context.Context, audioPath string) (string, error)
}

// WhisperTranscriber calls the OpenAI Whisper API.
// Endpoint and HTTPClient are optional; zero values use production defaults.
type WhisperTranscriber struct {
	HTTPClient *http.Client
	APIKey     string
	Endpoint   string // "" = whisperEndpoint
}

func (w *WhisperTranscriber) Transcribe(ctx context.Context, audioPath string) (string, error) {
	if w.APIKey == "" {
		return "", fmt.Errorf("transcribe: missing OpenAI API key")
	}

	f, err := os.Open(audioPath)
	if err != nil {
		return "", fmt.Errorf("open audio: %w", err)
	}
	defer f.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", fmt.Errorf("copy audio: %w", err)
	}
	if err := mw.WriteField("model", "whisper-1"); err != nil {
		return "", fmt.Errorf("write model field: %w", err)
	}
	if err := mw.WriteField("response_format", "text"); err != nil {
		return "", fmt.Errorf("write format field: %w", err)
	}
	mw.Close()

	endpoint := w.Endpoint
	if endpoint == "" {
		endpoint = whisperEndpoint
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+w.APIKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := w.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		var e struct {
			Error struct{ Message string } `json:"error"`
		}
		if json.Unmarshal(out, &e) == nil && e.Error.Message != "" {
			return "", fmt.Errorf("whisper: %s", e.Error.Message)
		}
		return "", fmt.Errorf("whisper: HTTP %d", resp.StatusCode)
	}
	return string(bytes.TrimSpace(out)), nil
}
