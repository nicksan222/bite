package ai

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/media"
)

// ─── mock helpers ─────────────────────────────────────────────────────────────

type mockStreamer struct{ resp string }

func (m *mockStreamer) Stream(_ context.Context, _ []Message, _ ...StreamOption) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 2)
	ch <- StreamEvent{Delta: m.resp}
	ch <- StreamEvent{Done: true, Final: m.resp}
	close(ch)
	return ch, nil
}

type mockTranscriber struct{ text string }

func (t *mockTranscriber) Transcribe(_ context.Context, _ string) (string, error) {
	return t.text, nil
}

type mockFrameExtractor struct{ paths []string }

func (e *mockFrameExtractor) ExtractFrames(_ context.Context, _ string, _ int) ([]string, string, error) {
	return e.paths, "", nil
}

type errTranscriber struct{}

func (e *errTranscriber) Transcribe(_ context.Context, _ string) (string, error) {
	return "", errors.New("transcription failed")
}

type errFrameExtractor struct{ err error }

func (e *errFrameExtractor) ExtractFrames(_ context.Context, _ string, _ int) ([]string, string, error) {
	return nil, "", e.err
}

type errStreamer struct{ err error }

func (e *errStreamer) Stream(_ context.Context, _ []Message, _ ...StreamOption) (<-chan StreamEvent, error) {
	return nil, e.err
}

type chanErrStreamer struct{}

func (e *chanErrStreamer) Stream(_ context.Context, _ []Message, _ ...StreamOption) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 1)
	ch <- StreamEvent{Err: errors.New("mid-stream failure")}
	close(ch)
	return ch, nil
}

type doneOnlyStreamer struct{ resp string }

func (s *doneOnlyStreamer) Stream(_ context.Context, _ []Message, _ ...StreamOption) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 1)
	ch <- StreamEvent{Done: true, Final: s.resp}
	close(ch)
	return ch, nil
}

type deltaOnlyStreamer struct{ resp string }

func (s *deltaOnlyStreamer) Stream(_ context.Context, _ []Message, _ ...StreamOption) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 2)
	ch <- StreamEvent{Delta: s.resp}
	ch <- StreamEvent{Done: true}
	close(ch)
	return ch, nil
}

const mealJSON = `{"title":"salad","description":"green salad","items":["lettuce","tomato"],"kcal":150,"protein_g":5,"carbs_g":20,"fat_g":3,"confidence":"high"}`

// ─── parseMealAnalysis ────────────────────────────────────────────────────────

func TestParseMealAnalysis_CleanJSON(t *testing.T) {
	in := `{"title":"pasta","kcal":600,"protein_g":20,"carbs_g":80,"fat_g":15,"confidence":"medium"}`
	got, err := parseMealAnalysis(in)
	require.NoError(t, err)
	assert.Equal(t, "pasta", got.Title)
	assert.Equal(t, float64(600), got.Kcal)
	assert.Equal(t, "medium", got.Confidence)
}

func TestParseMealAnalysis_StripFences(t *testing.T) {
	in := "```json\n{\"title\":\"x\",\"kcal\":1}\n```"
	got, err := parseMealAnalysis(in)
	require.NoError(t, err)
	assert.Equal(t, "x", got.Title)
	assert.Equal(t, float64(1), got.Kcal)
}

func TestParseMealAnalysis_SalvageFromProse(t *testing.T) {
	in := "Sure, here's the analysis:\n\n{\"title\":\"salad\",\"kcal\":250}\n\nHope that helps!"
	got, err := parseMealAnalysis(in)
	require.NoError(t, err)
	assert.Equal(t, "salad", got.Title)
	assert.Equal(t, float64(250), got.Kcal)
}

func TestParseMealAnalysis_Garbage(t *testing.T) {
	_, err := parseMealAnalysis("nope")
	assert.Error(t, err)
}

func TestParseMealAnalysis_BracesButInvalidJSON(t *testing.T) {
	_, err := parseMealAnalysis("{bad json}")
	assert.Error(t, err)
}

// ─── AnalyzeMeal ──────────────────────────────────────────────────────────────

func TestAnalyzeMeal_textOnly(t *testing.T) {
	got, err := AnalyzeMeal(context.Background(), &mockStreamer{resp: mealJSON}, AnalyzeInput{Text: "green salad"})
	require.NoError(t, err)
	assert.Equal(t, "salad", got.Title)
	assert.Equal(t, float64(150), got.Kcal)
}

func TestAnalyzeMeal_nothingToAnalyze(t *testing.T) {
	_, err := AnalyzeMeal(context.Background(), &mockStreamer{}, AnalyzeInput{})
	assert.Error(t, err)
}

func TestAnalyzeMeal_withMockTranscriber(t *testing.T) {
	in := AnalyzeInput{
		MediaPaths:  []string{"recording.mp3"},
		Transcriber: &mockTranscriber{text: "I had a salad"},
	}
	got, err := AnalyzeMeal(context.Background(), &mockStreamer{resp: mealJSON}, in)
	require.NoError(t, err)
	assert.Equal(t, "salad", got.Title)
}

func TestAnalyzeMeal_withMockFrameExtractor(t *testing.T) {
	tmpFrame, err := os.CreateTemp("", "frame*.jpg")
	require.NoError(t, err)
	tmpFrame.Close()
	defer os.Remove(tmpFrame.Name())

	in := AnalyzeInput{
		Text:           "video meal",
		MediaPaths:     []string{"meal.mp4"},
		FrameExtractor: &mockFrameExtractor{paths: []string{tmpFrame.Name()}},
	}
	got, err := AnalyzeMeal(context.Background(), &mockStreamer{resp: mealJSON}, in)
	require.NoError(t, err)
	assert.Equal(t, "salad", got.Title)
}

func TestAnalyzeMeal_videoDefaultExtractor_noFFmpeg(t *testing.T) {
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
	os.Setenv("PATH", t.TempDir())

	_, err := AnalyzeMeal(context.Background(), &mockStreamer{resp: mealJSON}, AnalyzeInput{
		MediaPaths: []string{"meal.mp4"},
	})
	assert.Error(t, err)
}

func TestAnalyzeMeal_audioMissingKeyFallsThrough(t *testing.T) {
	_, err := AnalyzeMeal(context.Background(), &mockStreamer{}, AnalyzeInput{
		MediaPaths: []string{"clip.mp3"},
	})
	assert.Error(t, err)
}

func TestAnalyzeMeal_streamEventError(t *testing.T) {
	_, err := AnalyzeMeal(context.Background(), &chanErrStreamer{}, AnalyzeInput{Text: "lunch"})
	assert.Error(t, err)
}

func TestAnalyzeMeal_withMediaURLs(t *testing.T) {
	got, err := AnalyzeMeal(context.Background(), &mockStreamer{resp: mealJSON}, AnalyzeInput{
		MediaURLs: []string{"https://example.com/meal.jpg"},
	})
	require.NoError(t, err)
	assert.Equal(t, "salad", got.Title)
}

func TestAnalyzeMeal_transcribeError(t *testing.T) {
	_, err := AnalyzeMeal(context.Background(), &mockStreamer{resp: mealJSON}, AnalyzeInput{
		MediaPaths:  []string{"clip.mp3"},
		Transcriber: &errTranscriber{},
	})
	assert.Error(t, err)
}

func TestAnalyzeMeal_videoFFmpegMissing(t *testing.T) {
	_, err := AnalyzeMeal(context.Background(), &mockStreamer{resp: mealJSON}, AnalyzeInput{
		MediaPaths:     []string{"meal.mp4"},
		FrameExtractor: &errFrameExtractor{err: media.ErrFFmpegMissing},
	})
	assert.Error(t, err)
}

func TestAnalyzeMeal_videoExtractError(t *testing.T) {
	_, err := AnalyzeMeal(context.Background(), &mockStreamer{resp: mealJSON}, AnalyzeInput{
		Text:           "video",
		MediaPaths:     []string{"meal.mp4"},
		FrameExtractor: &errFrameExtractor{err: errors.New("bad video")},
	})
	assert.Error(t, err)
}

func TestAnalyzeMeal_streamError(t *testing.T) {
	_, err := AnalyzeMeal(context.Background(), &errStreamer{err: errors.New("api down")}, AnalyzeInput{Text: "lunch"})
	assert.Error(t, err)
}

func TestAnalyzeMeal_imageLocalPath(t *testing.T) {
	tmpImg, err := os.CreateTemp("", "img*.jpg")
	require.NoError(t, err)
	tmpImg.Close()
	defer os.Remove(tmpImg.Name())

	got, err := AnalyzeMeal(context.Background(), &mockStreamer{resp: mealJSON}, AnalyzeInput{
		MediaPaths: []string{tmpImg.Name()},
	})
	require.NoError(t, err)
	assert.Equal(t, "salad", got.Title)
}

func TestAnalyzeMeal_audioWithKeyNoTranscriber(t *testing.T) {
	_, err := AnalyzeMeal(context.Background(), &mockStreamer{resp: mealJSON}, AnalyzeInput{
		MediaPaths:   []string{"clip.mp3"},
		OpenAIAPIKey: "sk-test-fake",
	})
	assert.Error(t, err, "WhisperTranscriber should fail without a real server")
}

func TestAnalyzeMeal_contentOnlyInFinal(t *testing.T) {
	got, err := AnalyzeMeal(context.Background(), &doneOnlyStreamer{resp: mealJSON}, AnalyzeInput{Text: "salad"})
	require.NoError(t, err)
	assert.Equal(t, "salad", got.Title)
	assert.Equal(t, float64(150), got.Kcal)
}

func TestAnalyzeMeal_contentOnlyInDeltas(t *testing.T) {
	got, err := AnalyzeMeal(context.Background(), &deltaOnlyStreamer{resp: mealJSON}, AnalyzeInput{Text: "salad"})
	require.NoError(t, err)
	assert.Equal(t, "salad", got.Title)
	assert.Equal(t, float64(150), got.Kcal)
}

// ─── interface compile-time checks ───────────────────────────────────────────

var _ Streamer = (*Client)(nil)
var _ media.Transcriber = (*media.WhisperTranscriber)(nil)
var _ media.FrameExtractor = (*media.FFmpegExtractor)(nil)
