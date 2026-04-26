package media

import "testing"

func TestDetectKind(t *testing.T) {
	cases := []struct {
		in   string
		want Kind
	}{
		// images
		{"meal.jpg", KindImage},
		{"meal.JPG", KindImage},
		{"photo.jpeg", KindImage},
		{"photo.png", KindImage},
		{"food.webp", KindImage},
		{"anim.gif", KindImage},
		{"photo.heic", KindImage},
		// audio
		{"voice.mp3", KindAudio},
		{"note.m4a", KindAudio},
		{"clip.wav", KindAudio},
		{"clip.ogg", KindAudio},
		{"clip.flac", KindAudio},
		{"clip.webm", KindAudio},
		// video
		{"clip.mp4", KindVideo},
		{"clip.MOV", KindVideo},
		{"clip.mkv", KindVideo},
		{"clip.avi", KindVideo},
		// URL with query string
		{"https://example.com/food.png?v=1", KindImage},
		// fallback
		{"unknown", KindImage},
		{"", KindImage},
	}
	for _, c := range cases {
		if got := DetectKind(c.in); got != c.want {
			t.Errorf("DetectKind(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
