package ai

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// Role is the speaker role for a message in a conversation.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Message is bite's transport-agnostic message type. It holds text and
// optional image attachments. Audio and video are pre-processed into text or
// images by internal/media before they reach this layer.
type Message struct {
	Role        Role
	Content     string
	Attachments []Attachment
}

// Attachment is multimodal content carried alongside a Message.
//
// Only Image is wired today. Audio and video are pre-processed into text
// (Whisper) or images (ffmpeg keyframes) by internal/media before they reach
// this layer — keeping Attachment narrow stops the type from sprawling.
type Attachment struct {
	// Path to a local file. If set, bite will read + base64-encode it.
	Path string
	// URL to a remote resource. If set, the model fetches it directly.
	URL string
	// MimeType (e.g. "image/jpeg"). Sniffed from Path/URL if empty.
	MimeType string
}

func (m Message) toSchema() (*schema.Message, error) {
	sm := &schema.Message{Role: schema.RoleType(m.Role)}

	if len(m.Attachments) == 0 {
		sm.Content = m.Content
		return sm, nil
	}

	parts := make([]schema.MessageInputPart, 0, len(m.Attachments)+1)
	if m.Content != "" {
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: m.Content,
		})
	}
	for _, a := range m.Attachments {
		p, err := a.toPart()
		if err != nil {
			return nil, err
		}
		parts = append(parts, p)
	}
	sm.UserInputMultiContent = parts
	return sm, nil
}

func (a Attachment) toPart() (schema.MessageInputPart, error) {
	mime := a.MimeType
	if mime == "" {
		mime = sniffMime(a.Path, a.URL)
	}

	switch {
	case strings.HasPrefix(mime, "image/"):
		img, err := a.imageInputPart(mime)
		if err != nil {
			return schema.MessageInputPart{}, err
		}
		return schema.MessageInputPart{
			Type:  schema.ChatMessagePartTypeImageURL,
			Image: img,
		}, nil
	default:
		return schema.MessageInputPart{}, fmt.Errorf("unsupported attachment mime %q (only image/* today)", mime)
	}
}

// imageInputPart builds a MessageInputImage from either a remote URL or a
// local file (encoded as base64).
func (a Attachment) imageInputPart(mime string) (*schema.MessageInputImage, error) {
	if a.URL != "" {
		u := a.URL
		return &schema.MessageInputImage{
			MessagePartCommon: schema.MessagePartCommon{URL: &u, MIMEType: mime},
		}, nil
	}
	if a.Path == "" {
		return nil, fmt.Errorf("attachment needs Path or URL")
	}
	raw, err := os.ReadFile(a.Path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", a.Path, err)
	}
	b64 := base64.StdEncoding.EncodeToString(raw)
	return &schema.MessageInputImage{
		MessagePartCommon: schema.MessagePartCommon{Base64Data: &b64, MIMEType: mime},
	}, nil
}

func sniffMime(path, url string) string {
	src := path
	if src == "" {
		src = url
	}
	// Strip query string so filepath.Ext works correctly on URLs like
	// "https://cdn.example.com/meal.jpg?v=2".
	if i := strings.IndexByte(src, '?'); i >= 0 {
		src = src[:i]
	}
	switch strings.ToLower(filepath.Ext(src)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".heic":
		return "image/heic"
	}
	if t := http.DetectContentType([]byte(src)); strings.HasPrefix(t, "image/") {
		return t
	}
	return "application/octet-stream"
}

func toSchemaMessages(msgs []Message) ([]*schema.Message, error) {
	out := make([]*schema.Message, 0, len(msgs))
	for _, m := range msgs {
		sm, err := m.toSchema()
		if err != nil {
			return nil, err
		}
		out = append(out, sm)
	}
	return out, nil
}
