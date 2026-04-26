// Package media handles non-AI media plumbing: classifying files, transcribing
// audio (OpenAI Whisper), and extracting video keyframes (ffmpeg). The AI
// layer only consumes the *outputs* of this package — text and image paths —
// so swapping providers (Whisper → local whisper.cpp, ffmpeg → pure-Go decoder)
// stays a localised change.
//
// External binaries:
//
//   - audio: requires OPENAI_API_KEY (HTTP, no binary)
//   - video: requires `ffmpeg` in PATH; CheckFFmpeg returns nil when present
//
// Add new media types here, not in internal/ai.
package media
