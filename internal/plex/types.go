package plex

// Server is one Plex Media Server the account has access to.
// BaseURL is set by PickConnection.
type Server struct {
	Name              string       `json:"name"`
	MachineIdentifier string       `json:"machineIdentifier"`
	AccessToken       string       `json:"accessToken,omitempty"` // never sent to the SPA
	BaseURL           string       `json:"baseURL"`               // populated by connection picker
	Connections       []Connection `json:"connections,omitempty"`
}

// Connection is a single advertised URI for a Plex server.
type Connection struct {
	URI      string `json:"uri"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Local    bool   `json:"local"`
	Relay    bool   `json:"relay"`
	Protocol string `json:"protocol"`
	IPv6     bool   `json:"IPv6"`
}

// Library is a top-level section on a server (Movies, TV Shows, Anime, etc.).
type Library struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// Item is a single piece of media — movie, show, season, or episode.
// Episode-specific fields (GrandparentTitle, ParentIndex, etc.) are only
// populated when Type == "episode".
type Item struct {
	RatingKey    string `json:"ratingKey"`
	GUID         string `json:"guid,omitempty"`
	Title        string `json:"title"`
	Type         string `json:"type"` // "movie" | "show" | "season" | "episode"
	Year         int    `json:"year,omitempty"`
	Summary      string `json:"summary,omitempty"`
	Thumb        string `json:"thumb,omitempty"`        // server-relative path to portrait poster
	Art          string `json:"art,omitempty"`          // server-relative path to landscape art
	Duration     int64  `json:"duration,omitempty"`     // media length in ms
	ViewOffset   int64  `json:"viewOffset,omitempty"`   // resume position in ms (0 = unstarted)
	AddedAt      int64  `json:"addedAt,omitempty"`      // epoch seconds when added to library
	LastViewedAt int64  `json:"lastViewedAt,omitempty"` // epoch seconds of most recent view
	// Episode-specific fields — empty for non-episodes.
	Index                 int          `json:"index,omitempty"`                 // episode number within season
	ParentIndex           int          `json:"parentIndex,omitempty"`           // season number
	ParentTitle           string       `json:"parentTitle,omitempty"`           // e.g. "Season 1"
	ParentThumb           string       `json:"parentThumb,omitempty"`           // season poster
	GrandparentTitle      string       `json:"grandparentTitle,omitempty"`      // show name
	GrandparentThumb      string       `json:"grandparentThumb,omitempty"`      // show poster (portrait)
	GrandparentArt        string       `json:"grandparentArt,omitempty"`        // show backdrop (landscape)
	GrandparentRatingKey  string       `json:"grandparentRatingKey,omitempty"`  // show's ratingKey
	ViewCount             int          `json:"viewCount,omitempty"`             // 0 = unwatched, ≥1 = watched (Plex semantics)
	OriginallyAvailableAt string       `json:"originallyAvailableAt,omitempty"` // air date "YYYY-MM-DD"
	Media                 []Media      `json:"Media,omitempty"`
	IMDBId                string       `json:"imdbId,omitempty"`
	Roles                 []Person     `json:"roles,omitempty"`
	Directors             []Person     `json:"directors,omitempty"`
	Writers               []Person     `json:"writers,omitempty"`
	Trailer               *TrailerInfo `json:"trailer,omitempty"`
}

// Media is one quality variant of an Item — Plex returns one entry per
// transcode profile / source file. The first entry is normally the original.
type Media struct {
	ID              int    `json:"id"`
	Container       string `json:"container,omitempty"`
	VideoResolution string `json:"videoResolution,omitempty"`
	VideoCodec      string `json:"videoCodec,omitempty"`
	AudioCodec      string `json:"audioCodec,omitempty"`
	Bitrate         int    `json:"bitrate,omitempty"`
	Duration        int64  `json:"duration,omitempty"` // overrides Item.Duration when present
	Part            []Part `json:"Part,omitempty"`
}

// Part is a single file backing a Media. Most items have exactly one Part.
//
// ID is intentionally string: server-local Plex returns numeric ids
// (e.g. "12345") but plex.tv Discover hub items return UUID-shaped
// composite ids (e.g. "691648f137d5bdeaa81f55b1-6918087fc7abb5aa29a67b10").
// Typing as string accepts both shapes. Discovered Session 5 post-smoke
// when home/trending-trailers responses started failing to unmarshal
// after we added includeMeta=1 (which surfaces full Media→Part chains
// on hub items).
type Part struct {
	ID        string `json:"id"`
	Key       string `json:"key,omitempty"` // e.g. "/library/parts/123/file.mkv"
	Size      int64  `json:"size,omitempty"`
	Duration  int64  `json:"duration,omitempty"`
	Container string `json:"container,omitempty"`
}

// HubItem is one card on a plex.tv Discover hub (home or watchlist namespace).
//
// Thumb is an absolute URL (e.g. https://metadata-static.plex.tv/... or
// https://image.tmdb.org/...). Render directly with <img src> — does NOT
// need to go through Lumen's image proxy. Confirmed via Plex Web capture
// of home/trending-trailers in Session 5 post-smoke.
type HubItem struct {
	GUID      string `json:"guid,omitempty"`
	RatingKey string `json:"ratingKey"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Year      int    `json:"year,omitempty"`
	Thumb     string `json:"thumb,omitempty"`
}

// ItemQuery carries optional filter/sort parameters for GetItems.
// Not serialized — input to the Go client only.
type ItemQuery struct {
	Sort    string
	Filters map[string]string
	Start   int
	Size    int
}

// Person is a single cast or crew member for an Item. Tag is the role —
// character name for actors, role name for crew.
type Person struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Tag   string `json:"tag,omitempty"`   // character name (Role) or role name (Director/Writer)
	Thumb string `json:"thumb,omitempty"` // server-relative path; pass through image-proxy
}

// TrailerInfo carries a single Plex Extras-sourced trailer reference.
// PlexKey is set when the trailer is a Plex-hosted file; YouTubeID is set
// when the Extra resolves to a `youtube://...` GUID. Exactly one is non-empty.
type TrailerInfo struct {
	Title     string `json:"title,omitempty"`
	PlexKey   string `json:"plexKey,omitempty"`   // /library/parts/.../file.flv
	YouTubeID string `json:"youtubeID,omitempty"` // 11-char YouTube ID
}
