package plex

import (
	"net/url"
	"strings"
	"testing"
)

func TestDirectPlayURL(t *testing.T) {
	srv := &Server{
		BaseURL:     "https://srv.example.com",
		AccessToken: "secret",
	}
	got := DirectPlayURL(srv, "12345", "mkv")
	want := "https://srv.example.com/library/parts/12345/0/file.mkv?X-Plex-Token=secret"
	if got != want {
		t.Errorf("DirectPlayURL = %q, want %q", got, want)
	}
}

func TestTranscodeURL_HasRequiredParams(t *testing.T) {
	srv := &Server{
		BaseURL:     "https://srv.example.com",
		AccessToken: "secret",
	}
	got := TranscodeURL(srv, "12345", "session-abc")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(u.Path, "/video/:/transcode/universal/start.m3u8") {
		t.Errorf("wrong path: %s", u.Path)
	}
	q := u.Query()
	for _, key := range []string{"path", "directPlay", "directStream", "protocol", "videoQuality", "videoResolution", "session", "X-Plex-Token"} {
		if q.Get(key) == "" {
			t.Errorf("missing query param: %s", key)
		}
	}
	if q.Get("session") != "session-abc" {
		t.Errorf("session = %q, want session-abc", q.Get("session"))
	}
}
