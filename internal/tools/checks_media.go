package tools

import (
	"context"
	"errors"

	"github.com/nicksan222/bite/internal/config"
	"github.com/nicksan222/bite/internal/media"
)

func init() {
	RegisterCheck(Check{
		Name:     "media: openai key",
		Severity: SeveritySoft,
		Desc:     "OPENAI_API_KEY enables Whisper audio transcription",
		Run: func(_ context.Context) (string, error) {
			cfg, err := config.Load()
			if err != nil {
				return "", err
			}
			if cfg.OpenAIAPIKey == "" {
				return "", errors.New("not set — audio (Whisper) disabled")
			}
			return "set — audio transcription available", nil
		},
	})

	RegisterCheck(Check{
		Name:     "media: ffmpeg",
		Severity: SeveritySoft,
		Desc:     "ffmpeg in PATH enables video keyframe extraction",
		Run: func(_ context.Context) (string, error) {
			if err := media.CheckFFmpeg(); err != nil {
				return "", err
			}
			return "found — video keyframe extraction available", nil
		},
	})
}
