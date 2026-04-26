package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/nicksan222/bite/internal/media"
)

// MealAnalysis is the structured nutrition output.
type MealAnalysis struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Items       []string `json:"items"`
	Kcal        float64  `json:"kcal"`
	ProteinG    float64  `json:"protein_g"`
	CarbsG      float64  `json:"carbs_g"`
	FatG        float64  `json:"fat_g"`
	Confidence  string   `json:"confidence"`
}

// AnalyzeInput is everything the user can throw at AnalyzeMeal — any mix of
// text, images, audio, and video. Audio is transcribed (Whisper); video is
// keyframe-extracted (ffmpeg). Both fold into the final Claude vision call.
type AnalyzeInput struct {
	// Text is free-form context ("had this for lunch around 1pm"). Optional.
	Text string

	// MediaPaths are local files. Kind is detected by extension.
	MediaPaths []string

	// MediaURLs are remote http(s) URLs. Treated as images.
	MediaURLs []string

	// OpenAIAPIKey is required for audio when Transcriber is nil.
	OpenAIAPIKey string

	// FramesPerVideo controls how many keyframes are extracted per video.
	FramesPerVideo int

	// Transcriber overrides the default WhisperTranscriber for audio files.
	// Nil creates a WhisperTranscriber with OpenAIAPIKey.
	Transcriber media.Transcriber

	// FrameExtractor overrides the default FFmpegExtractor for video files.
	// Nil creates an FFmpegExtractor.
	FrameExtractor media.FrameExtractor
}

const analyzePrompt = `Analyze the meal described by the attached media and any text below.

Return ONLY a JSON object matching this exact shape — no prose, no markdown fences:

{
  "title":       "short name of the meal",
  "description": "1–2 sentence description of what's on the plate",
  "items":       ["item", "item"],
  "kcal":        0,
  "protein_g":   0,
  "carbs_g":     0,
  "fat_g":       0,
  "confidence":  "low"
}

confidence is "low" | "medium" | "high".
If multiple images are provided, treat them as different angles of the SAME meal.
Numbers are integers or one decimal place. Round generously.`

// AnalyzeMeal accepts any combination of media + text and returns a structured
// nutrition estimate. It transcribes audio and extracts video keyframes
// before sending one consolidated request to Claude.
func AnalyzeMeal(ctx context.Context, c Streamer, in AnalyzeInput) (*MealAnalysis, error) {
	if len(in.MediaPaths) == 0 && len(in.MediaURLs) == 0 && in.Text == "" {
		return nil, fmt.Errorf("analyze: nothing to analyze (pass media or text)")
	}

	prompt, attachments, cleanup, err := preprocess(ctx, in)
	defer cleanup()
	if err != nil {
		return nil, err
	}

	msg := Message{
		Role:        RoleUser,
		Content:     prompt,
		Attachments: attachments,
	}

	// Empty system prompt: the analyze prompt's "ONLY JSON" instruction must
	// dominate, and the configured nutritionist persona would loosen it.
	ch, err := c.Stream(ctx, []Message{msg}, WithSystemPrompt(""))
	if err != nil {
		return nil, err
	}

	var raw strings.Builder
	for ev := range ch {
		if ev.Err != nil {
			return nil, ev.Err
		}
		if ev.Done {
			// Use the authoritative Final string when available (handles the case
			// where a Streamer emits content only in Done.Final with no Deltas).
			if ev.Final != "" {
				return parseMealAnalysis(ev.Final)
			}
			break
		}
		raw.WriteString(ev.Delta)
	}

	return parseMealAnalysis(raw.String())
}

// preprocess walks the input, transcribing audio and extracting frames as
// needed, and returns the final prompt + image-only attachments + a cleanup
// func for any temp files it created.
func preprocess(ctx context.Context, in AnalyzeInput) (string, []Attachment, func(), error) {
	var (
		extraText []string
		images    []Attachment
		tmpDirs   []string
	)
	cleanup := func() {
		for _, d := range tmpDirs {
			_ = os.RemoveAll(d)
		}
	}

	for _, url := range in.MediaURLs {
		images = append(images, Attachment{URL: url})
	}

	for _, path := range in.MediaPaths {
		switch media.DetectKind(path) {
		case media.KindImage:
			images = append(images, Attachment{Path: path})

		case media.KindAudio:
			transcriber := in.Transcriber
			if transcriber == nil {
				if in.OpenAIAPIKey == "" {
					return "", nil, cleanup, fmt.Errorf("audio file %s needs OPENAI_API_KEY for Whisper transcription", path)
				}
				transcriber = &media.WhisperTranscriber{APIKey: in.OpenAIAPIKey}
			}
			text, err := transcriber.Transcribe(ctx, path)
			if err != nil {
				return "", nil, cleanup, fmt.Errorf("transcribe %s: %w", path, err)
			}
			if text != "" {
				extraText = append(extraText, fmt.Sprintf("[transcript of %s]\n%s", path, text))
			}

		case media.KindVideo:
			extractor := in.FrameExtractor
			if extractor == nil {
				extractor = &media.FFmpegExtractor{}
			}
			frames, dir, err := extractor.ExtractFrames(ctx, path, in.FramesPerVideo)
			if err != nil {
				if errors.Is(err, media.ErrFFmpegMissing) {
					return "", nil, cleanup, fmt.Errorf("video file %s skipped: %w", path, err)
				}
				return "", nil, cleanup, fmt.Errorf("extract frames %s: %w", path, err)
			}
			tmpDirs = append(tmpDirs, dir)
			for _, f := range frames {
				images = append(images, Attachment{Path: f})
			}
		}
	}

	prompt := analyzePrompt
	if in.Text != "" {
		prompt = in.Text + "\n\n" + prompt
	}
	if len(extraText) > 0 {
		prompt = strings.Join(extraText, "\n\n") + "\n\n" + prompt
	}
	return prompt, images, cleanup, nil
}

func parseMealAnalysis(s string) (*MealAnalysis, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	var a MealAnalysis
	if err := json.Unmarshal([]byte(s), &a); err == nil {
		return &a, nil
	}
	if first := strings.Index(s, "{"); first >= 0 {
		if last := strings.LastIndex(s, "}"); last > first {
			if err := json.Unmarshal([]byte(s[first:last+1]), &a); err == nil {
				return &a, nil
			}
		}
	}
	return nil, fmt.Errorf("model did not return parseable JSON: %s", s)
}
