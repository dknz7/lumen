package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"lumen/internal/playback"
	"lumen/internal/plex"
)

type playRequest struct {
	ServerID         string `json:"serverID"`
	RatingKey        string `json:"ratingKey"`
	ResumeFromOffset int64  `json:"resumeFromOffset,omitempty"` // ms
}

func (s *Server) handlePlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req playRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	srv := s.serverByID(req.ServerID)
	if srv == nil {
		writeError(w, http.StatusNotFound, "unknown server")
		return
	}
	plexSrv := toPlexServer(srv)

	item, err := s.plex.GetItem(plexSrv, req.RatingKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetch item: "+err.Error())
		return
	}
	if len(item.Media) == 0 || len(item.Media[0].Part) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "item has no playable parts")
		return
	}
	part := item.Media[0].Part[0]
	ext := containerToExt(part.Container)

	streamURL := plex.DirectPlayURL(plexSrv, string(part.ID), ext)

	args := playback.StartArgs{
		Server:        plexSrv,
		RatingKey:     req.RatingKey,
		ShowRatingKey: item.GrandparentRatingKey,
		IsEpisode:     item.Type == "episode",
		PartID:        string(part.ID),
		Container:     part.Container,
		StreamURL:     streamURL,
		Transcoding:   false,
		Duration:      msToDuration(item.Duration),
		Title:         item.Title,
		ShowTitle:     item.GrandparentTitle,
		ThumbPath:     pickThumbPath(item),
		Quality:       formatQuality(item),

		EpisodeIndex:          item.Index,
		SeasonIndex:           item.ParentIndex,
		AddedAt:               item.AddedAt,
		OriginallyAvailableAt: item.OriginallyAvailableAt,

		ResumeOffsetMs: req.ResumeFromOffset,
	}

	if err := s.playback.Start(args); err != nil {
		if err == playback.ErrAlreadyActive {
			writeError(w, http.StatusConflict, "another session is already active")
			return
		}
		writeError(w, http.StatusInternalServerError, "start: "+err.Error())
		return
	}

	writeJSON(w, s.playback.SnapshotState())
}

// containerToExt maps a Plex container value to the file extension Pot Player
// expects in the direct-play URL. Plex uses "mkv", "mp4", "avi" etc. mostly
// directly — pass through, default to "mp4" if blank.
func containerToExt(container string) string {
	if container == "" {
		return "mp4"
	}
	return container
}

// msToDuration converts Plex's millisecond integer to time.Duration.
// Item.Duration is int64 — match that type, NOT the plan's `int`.
func msToDuration(ms int64) time.Duration {
	return time.Duration(ms) * time.Millisecond
}

// pickThumbPath returns the best thumb for the Now Playing strip. For
// episodes, prefers the show's portrait poster (GrandparentThumb); for
// movies, falls back to the item's own Thumb.
func pickThumbPath(it plex.Item) string {
	if it.GrandparentThumb != "" {
		return it.GrandparentThumb
	}
	return it.Thumb
}

// formatQuality builds a human-readable "1080p H.264" string from the first
// Media entry. Returns empty if no Media exists or both fields are empty.
func formatQuality(it plex.Item) string {
	if len(it.Media) == 0 {
		return ""
	}
	m := it.Media[0]
	res := m.VideoResolution
	codec := m.VideoCodec
	switch {
	case res != "" && codec != "":
		return fmt.Sprintf("%s %s", res, codec)
	case res != "":
		return res
	default:
		return codec
	}
}

func (s *Server) handlePlayTranscode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req playRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	srv := s.serverByID(req.ServerID)
	if srv == nil {
		writeError(w, http.StatusNotFound, "unknown server")
		return
	}
	plexSrv := toPlexServer(srv)
	item, err := s.plex.GetItem(plexSrv, req.RatingKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetch item: "+err.Error())
		return
	}

	session := newTranscodeSession()
	streamURL := plex.TranscodeURL(plexSrv, req.RatingKey, session)

	args := playback.StartArgs{
		Server:           plexSrv,
		RatingKey:        req.RatingKey,
		ShowRatingKey:    item.GrandparentRatingKey,
		IsEpisode:        item.Type == "episode",
		PartID:           "",
		Container:        "",
		StreamURL:        streamURL,
		Transcoding:      true,
		TranscodeSession: session,
		Duration:         msToDuration(item.Duration),
		Title:            item.Title,
		ShowTitle:        item.GrandparentTitle,
		ThumbPath:        pickThumbPath(item),
		Quality:          "transcoded 1080p", // TODO(phase-g+): derive from actual transcode profile

		EpisodeIndex:          item.Index,
		SeasonIndex:           item.ParentIndex,
		AddedAt:               item.AddedAt,
		OriginallyAvailableAt: item.OriginallyAvailableAt,

		ResumeOffsetMs: req.ResumeFromOffset,
	}
	if err := s.playback.Start(args); err != nil {
		if err == playback.ErrAlreadyActive {
			writeError(w, http.StatusConflict, "another session is already active")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, s.playback.SnapshotState())
}

// newTranscodeSession returns a fresh hex-encoded session identifier prefixed
// with "lumen-" so the value is recognisable in Plex's transcode session list.
// 8 bytes of crypto/rand → 16 hex chars → "lumen-XXXXXXXXXXXXXXXX" (22 chars).
func newTranscodeSession() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "lumen-" + hex.EncodeToString(b[:])
}

func (s *Server) handlePlayStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	s.playback.Stop()
	writeJSON(w, map[string]string{"status": "stopped"})
}
