package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ErrFFmpegMissing is returned by ExtractFrames (and CheckFFmpeg) when ffmpeg
// is not on PATH. Callers can use errors.Is for graceful degradation.
var ErrFFmpegMissing = errors.New("ffmpeg not found in PATH (install: apt install ffmpeg | brew install ffmpeg)")

// FrameExtractor extracts frames from a video file.
type FrameExtractor interface {
	ExtractFrames(ctx context.Context, videoPath string, n int) (frames []string, tmpDir string, err error)
}

// FFmpegExtractor uses the ffmpeg binary for frame extraction.
type FFmpegExtractor struct{}

// CheckFFmpeg returns nil when ffmpeg is on PATH, ErrFFmpegMissing otherwise.
func CheckFFmpeg() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return ErrFFmpegMissing
	}
	return nil
}

func (e *FFmpegExtractor) ExtractFrames(ctx context.Context, videoPath string, n int) (frames []string, tmpDir string, err error) {
	if err := CheckFFmpeg(); err != nil {
		return nil, "", err
	}
	if n <= 0 {
		n = 4
	}

	dir, err := os.MkdirTemp("", "bite-frames-*")
	if err != nil {
		return nil, "", err
	}

	pattern := filepath.Join(dir, "frame-%03d.jpg")
	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-loglevel", "error",
		"-i", videoPath,
		"-vf", "thumbnail,select='not(mod(n\\,30))'",
		"-vsync", "vfr",
		"-frames:v", fmt.Sprintf("%d", n),
		pattern,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(dir)
		return nil, "", fmt.Errorf("ffmpeg: %w\n%s", err, string(out))
	}

	matches, err := filepath.Glob(filepath.Join(dir, "frame-*.jpg"))
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, "", err
	}
	if len(matches) == 0 {
		_ = os.RemoveAll(dir)
		return nil, "", fmt.Errorf("ffmpeg produced no frames from %s", videoPath)
	}
	return matches, dir, nil
}
