package media_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/media"
)

func tempAudioFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.mp3")
	require.NoError(t, os.WriteFile(path, []byte("fake audio bytes"), 0o644))
	return path
}

func TestWhisperTranscriber_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("  I had a salad  "))
	}))
	defer srv.Close()

	tr := &media.WhisperTranscriber{
		APIKey:     "test-key",
		Endpoint:   srv.URL,
		HTTPClient: srv.Client(),
	}

	got, err := tr.Transcribe(context.Background(), tempAudioFile(t))
	require.NoError(t, err)
	assert.Equal(t, "I had a salad", got)
}

func TestWhisperTranscriber_missingKey(t *testing.T) {
	tr := &media.WhisperTranscriber{}
	_, err := tr.Transcribe(context.Background(), tempAudioFile(t))
	require.Error(t, err)
}

func TestWhisperTranscriber_httpError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid auth"}}`))
	}))
	defer srv.Close()

	tr := &media.WhisperTranscriber{
		APIKey:     "bad-key",
		Endpoint:   srv.URL,
		HTTPClient: srv.Client(),
	}

	_, err := tr.Transcribe(context.Background(), tempAudioFile(t))
	require.Error(t, err)
	assert.Equal(t, "whisper: invalid auth", err.Error())
}

func TestWhisperTranscriber_httpErrorNoBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	tr := &media.WhisperTranscriber{
		APIKey:     "key",
		Endpoint:   srv.URL,
		HTTPClient: srv.Client(),
	}

	_, err := tr.Transcribe(context.Background(), tempAudioFile(t))
	require.Error(t, err)
	assert.Equal(t, "whisper: HTTP 500", err.Error())
}

func TestWhisperTranscriber_missingFile(t *testing.T) {
	tr := &media.WhisperTranscriber{APIKey: "key"}
	_, err := tr.Transcribe(context.Background(), "/nonexistent/path.mp3")
	require.Error(t, err)
}

func TestWhisperTranscriber_defaultHTTPClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("rice bowl"))
	}))
	defer srv.Close()

	tr := &media.WhisperTranscriber{
		APIKey:   "test-key",
		Endpoint: srv.URL,
		// HTTPClient intentionally nil → exercises client == nil branch
	}

	got, err := tr.Transcribe(context.Background(), tempAudioFile(t))
	require.NoError(t, err)
	assert.Equal(t, "rice bowl", got)
}

func TestWhisperTranscriber_defaultEndpoint(t *testing.T) {
	// Endpoint == "" → uses whisperEndpoint (api.openai.com). Intercept via custom RoundTripper.
	var gotURL string
	rt := &roundTripFunc{fn: func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("salad")),
		}, nil
	}}

	tr := &media.WhisperTranscriber{
		APIKey:     "test-key",
		Endpoint:   "", // intentionally empty → default whisper endpoint
		HTTPClient: &http.Client{Transport: rt},
	}

	got, err := tr.Transcribe(context.Background(), tempAudioFile(t))
	require.NoError(t, err)
	assert.Equal(t, "salad", got)
	assert.Contains(t, gotURL, "openai.com")
}

func TestWhisperTranscriber_doError(t *testing.T) {
	rt := &roundTripFunc{fn: func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	}}

	tr := &media.WhisperTranscriber{
		APIKey:     "test-key",
		Endpoint:   "https://example.com/transcribe",
		HTTPClient: &http.Client{Transport: rt},
	}

	_, err := tr.Transcribe(context.Background(), tempAudioFile(t))
	require.Error(t, err)
}

// roundTripFunc lets tests inject a custom HTTP transport.
type roundTripFunc struct {
	fn func(*http.Request) (*http.Response, error)
}

func (r *roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return r.fn(req)
}

func TestWhisperTranscriber_invalidEndpoint(t *testing.T) {
	// An invalid URL causes http.NewRequestWithContext to fail.
	tr := &media.WhisperTranscriber{
		APIKey:   "test-key",
		Endpoint: "://invalid url with spaces",
	}
	_, err := tr.Transcribe(context.Background(), tempAudioFile(t))
	require.Error(t, err)
}

func TestWhisperTranscriber_bodyReadError(t *testing.T) {
	// Server returns 200 but the body reader errors on Read.
	rt := &roundTripFunc{fn: func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(&errReader{}),
		}, nil
	}}

	tr := &media.WhisperTranscriber{
		APIKey:     "test-key",
		Endpoint:   "https://example.com/whisper",
		HTTPClient: &http.Client{Transport: rt},
	}

	_, err := tr.Transcribe(context.Background(), tempAudioFile(t))
	require.Error(t, err)
}

// errReader is an io.Reader that always returns an error.
type errReader struct{}

func (e *errReader) Read([]byte) (int, error) { return 0, errors.New("read error") }
