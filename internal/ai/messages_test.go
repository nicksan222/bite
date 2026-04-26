package ai

import (
	"os"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToSchemaMessages_TextOnly(t *testing.T) {
	in := []Message{
		{Role: RoleSystem, Content: "you are bite"},
		{Role: RoleUser, Content: "hi"},
		{Role: RoleAssistant, Content: "hello"},
	}

	got, err := toSchemaMessages(in)
	require.NoError(t, err)
	require.Len(t, got, len(in))

	wantRoles := []schema.RoleType{schema.System, schema.User, schema.Assistant}
	for i, m := range got {
		assert.Equal(t, wantRoles[i], m.Role, "msg[%d] role", i)
		assert.Equal(t, in[i].Content, m.Content, "msg[%d] content", i)
	}
}

func TestToSchemaMessages_ImageURL(t *testing.T) {
	in := []Message{{
		Role:    RoleUser,
		Content: "what's in this photo?",
		Attachments: []Attachment{
			{URL: "https://example.com/meal.jpg"},
		},
	}}
	got, err := toSchemaMessages(in)
	require.NoError(t, err)
	require.Len(t, got[0].UserInputMultiContent, 2, "expected 2 parts (text + image)")
	p := got[0].UserInputMultiContent[1]
	assert.Equal(t, schema.ChatMessagePartTypeImageURL, p.Type, "part 1 type")
	require.NotNil(t, p.Image)
	require.NotNil(t, p.Image.URL)
	assert.Equal(t, "https://example.com/meal.jpg", *p.Image.URL)
}

func TestToSchemaMessages_UnsupportedMime(t *testing.T) {
	in := []Message{{
		Role: RoleUser,
		Attachments: []Attachment{
			{URL: "https://example.com/song.mp3", MimeType: "audio/mpeg"},
		},
	}}
	_, err := toSchemaMessages(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image")
}

func TestSniffMime_extensions(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"photo.jpg", "image/jpeg"},
		{"photo.jpeg", "image/jpeg"},
		{"photo.PNG", "image/png"},
		{"anim.GIF", "image/gif"},
		{"img.webp", "image/webp"},
		{"photo.heic", "image/heic"},
		{"photo.HEIC", "image/heic"},
		{"file.bin", "application/octet-stream"},
		{"", "application/octet-stream"},
	}
	for _, tc := range cases {
		got := sniffMime(tc.path, "")
		assert.Equal(t, tc.want, got, "sniffMime(%q, '')", tc.path)
	}
}

func TestSniffMime_fallsBackToURL(t *testing.T) {
	got := sniffMime("", "https://example.com/img.png")
	assert.Equal(t, "image/png", got)
}

func TestSniffMime_stripsQueryString(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://cdn.example.com/meal.jpg?v=2", "image/jpeg"},
		{"https://cdn.example.com/photo.png?size=large&ts=1234", "image/png"},
		{"https://cdn.example.com/anim.gif?token=abc", "image/gif"},
		{"https://cdn.example.com/img.webp?w=800", "image/webp"},
	}
	for _, tc := range cases {
		got := sniffMime("", tc.url)
		assert.Equal(t, tc.want, got, "sniffMime(%q)", tc.url)
	}
}

func TestSniffMime_detectsViaMagicBytes(t *testing.T) {
	// PNG magic bytes — no file extension, so sniffMime must use DetectContentType.
	pngMagic := "\x89PNG\r\n\x1a\n"
	got := sniffMime(pngMagic, "")
	assert.Equal(t, "image/png", got)
}

func TestToPart_mimeTypeOverride(t *testing.T) {
	// Attachment with no extension but explicit MimeType override.
	a := Attachment{URL: "https://example.com/image", MimeType: "image/jpeg"}
	part, err := a.toPart()
	require.NoError(t, err)
	require.NotNil(t, part.Image)
	require.NotNil(t, part.Image.URL)
	assert.Equal(t, "https://example.com/image", *part.Image.URL)
}

func TestToPart_imageURLError(t *testing.T) {
	// Path that doesn't exist → imageURL will fail to ReadFile.
	a := Attachment{Path: "/nonexistent/file.jpg"}
	_, err := a.toPart()
	require.Error(t, err, "expected error when reading nonexistent image file")
}

func TestImageInputPart_returnsURL(t *testing.T) {
	a := Attachment{URL: "https://example.com/meal.jpg"}
	got, err := a.imageInputPart("image/jpeg")
	require.NoError(t, err)
	require.NotNil(t, got.URL)
	assert.Equal(t, "https://example.com/meal.jpg", *got.URL)
}

func TestImageInputPart_readsFileAsBase64(t *testing.T) {
	f, err := os.CreateTemp("", "test*.jpg")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	f.WriteString("fake-image-bytes")
	f.Close()

	a := Attachment{Path: f.Name()}
	got, err := a.imageInputPart("image/jpeg")
	require.NoError(t, err)
	assert.NotNil(t, got.Base64Data)
	assert.Equal(t, "image/jpeg", got.MIMEType)
}

func TestImageInputPart_missingPathAndURL(t *testing.T) {
	a := Attachment{}
	_, err := a.imageInputPart("image/jpeg")
	require.Error(t, err, "expected error for empty path and URL")
}

func TestImageInputPart_badPath(t *testing.T) {
	a := Attachment{Path: "/nonexistent/path/image.jpg"}
	_, err := a.imageInputPart("image/jpeg")
	require.Error(t, err, "expected error for unreadable file")
}
