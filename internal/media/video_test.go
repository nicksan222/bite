package media_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/media"
)

func TestCheckFFmpeg_missing(t *testing.T) {
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) })
	os.Setenv("PATH", t.TempDir()) // empty dir → ffmpeg not found

	err := media.CheckFFmpeg()
	assert.ErrorIs(t, err, media.ErrFFmpegMissing)
}

func TestFFmpegExtractor_missingBinary(t *testing.T) {
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) })
	os.Setenv("PATH", t.TempDir())

	e := &media.FFmpegExtractor{}
	_, _, err := e.ExtractFrames(context.Background(), "fake.mp4", 2)
	assert.ErrorIs(t, err, media.ErrFFmpegMissing)
}

func TestFFmpegExtractor_withFakeBinary(t *testing.T) {
	// Create a fake ffmpeg that exits non-zero to test error propagation.
	dir := t.TempDir()
	fakeFFmpeg := filepath.Join(dir, "ffmpeg")
	require.NoError(t, os.WriteFile(fakeFFmpeg, []byte("#!/bin/sh\nexit 1"), 0o755))
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) })
	os.Setenv("PATH", dir+":"+orig)

	e := &media.FFmpegExtractor{}
	_, _, err := e.ExtractFrames(context.Background(), "fake.mp4", 2)
	require.Error(t, err)
	assert.False(t, errors.Is(err, media.ErrFFmpegMissing), "error should not be ErrFFmpegMissing when binary exists but fails")
}

func TestFFmpegExtractor_defaultN(t *testing.T) {
	// Fake ffmpeg: writes 2 JPEG files into the pattern dir and exits 0.
	dir := t.TempDir()
	fakeFFmpeg := filepath.Join(dir, "ffmpeg")
	script := "#!/bin/sh\nfor last; do :; done\noutdir=$(dirname \"$last\")\ntouch \"$outdir/frame-001.jpg\"\ntouch \"$outdir/frame-002.jpg\"\nexit 0"
	require.NoError(t, os.WriteFile(fakeFFmpeg, []byte(script), 0o755))
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) })
	os.Setenv("PATH", dir+":"+orig)

	e := &media.FFmpegExtractor{}
	// n=0 triggers the default-to-4 branch
	frames, tmpDir, err := e.ExtractFrames(context.Background(), "fake.mp4", 0)
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	assert.Len(t, frames, 2)
}

func TestFFmpegExtractor_noFramesProduced(t *testing.T) {
	// Fake ffmpeg: exits 0 but produces no frame files.
	dir := t.TempDir()
	fakeFFmpeg := filepath.Join(dir, "ffmpeg")
	require.NoError(t, os.WriteFile(fakeFFmpeg, []byte("#!/bin/sh\nexit 0"), 0o755))
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) })
	os.Setenv("PATH", dir+":"+orig)

	e := &media.FFmpegExtractor{}
	_, _, err := e.ExtractFrames(context.Background(), "fake.mp4", 1)
	require.Error(t, err)
}

func TestFFmpegExtractor_mkdirTempError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based tmpdir trick not applicable on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("running as root — chmod cannot restrict access")
	}

	// Create a fake ffmpeg so CheckFFmpeg passes.
	fakeDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fakeDir, "ffmpeg"), []byte("#!/bin/sh\nexit 0"), 0o755))
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", fakeDir+":"+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	// Point TMPDIR at a read-only directory so os.MkdirTemp fails.
	readOnly := t.TempDir()
	require.NoError(t, os.Chmod(readOnly, 0o444))
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o755) })
	origTmp := os.Getenv("TMPDIR")
	os.Setenv("TMPDIR", readOnly)
	t.Cleanup(func() { os.Setenv("TMPDIR", origTmp) })

	e := &media.FFmpegExtractor{}
	_, _, err := e.ExtractFrames(context.Background(), "fake.mp4", 1)
	require.Error(t, err)
}

func TestCheckFFmpeg_found(t *testing.T) {
	// Create a fake ffmpeg that succeeds the LookPath check.
	dir := t.TempDir()
	fakeFFmpeg := filepath.Join(dir, "ffmpeg")
	require.NoError(t, os.WriteFile(fakeFFmpeg, []byte("#!/bin/sh\nexit 0"), 0o755))
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) })
	os.Setenv("PATH", dir+":"+orig)

	require.NoError(t, media.CheckFFmpeg())
}
