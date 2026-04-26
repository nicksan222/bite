package media

import (
	"path/filepath"
	"strings"
)

// Kind classifies a media source.
type Kind string

const (
	KindImage Kind = "image"
	KindAudio Kind = "audio"
	KindVideo Kind = "video"
)

// DetectKind classifies a file path or URL by extension. Falls back to image.
func DetectKind(pathOrURL string) Kind {
	ext := strings.ToLower(filepath.Ext(strings.SplitN(pathOrURL, "?", 2)[0]))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".heic":
		return KindImage
	case ".mp3", ".wav", ".m4a", ".ogg", ".flac", ".webm":
		return KindAudio
	case ".mp4", ".mov", ".mkv", ".avi":
		return KindVideo
	}
	return KindImage
}
