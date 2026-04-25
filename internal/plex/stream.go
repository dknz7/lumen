package plex

import (
	"fmt"
	"net/url"
)

// DirectPlayURL builds the URL Pot Player should hit for direct play. Per
// spec §8.1: /library/parts/{partID}/{id}/file.{ext}?X-Plex-Token=...
// The {id} segment is unused by Plex (any value works); we hardcode 0.
func DirectPlayURL(s *Server, partID, ext string) string {
	q := url.Values{
		"X-Plex-Token": []string{s.AccessToken},
	}
	return fmt.Sprintf("%s/library/parts/%s/0/file.%s?%s",
		s.BaseURL, partID, ext, q.Encode())
}

// TranscodeURL builds the HLS transcode URL used as fallback when direct
// play fails. The session parameter MUST match the value passed to
// transcode/universal/ping for the keep-alive ticker. Spec §8.1 + §9.3.
func TranscodeURL(s *Server, ratingKey, session string) string {
	q := url.Values{
		"path":            []string{fmt.Sprintf("/library/metadata/%s", ratingKey)},
		"directPlay":      []string{"0"},
		"directStream":    []string{"0"},
		"protocol":        []string{"hls"},
		"videoQuality":    []string{"100"},
		"videoResolution": []string{"1920x1080"},
		"mediaBufferSize": []string{"204800"},
		"session":         []string{session},
		"X-Plex-Token":    []string{s.AccessToken},
	}
	return fmt.Sprintf("%s/video/:/transcode/universal/start.m3u8?%s",
		s.BaseURL, q.Encode())
}
