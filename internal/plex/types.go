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
	ParentRatingKey       string       `json:"parentRatingKey,omitempty"`       // season's ratingKey on episode items; show's ratingKey on season items
	ParentGuid            string       `json:"parentGuid,omitempty"`            // season's plex.tv GUID on episode items; show's on season items
	GrandparentGuid       string       `json:"grandparentGuid,omitempty"`       // show's plex.tv GUID on episode items
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

// PartID handles Plex's mixed id representations across surfaces:
// server-local responses use numeric ids (e.g. 12345); plex.tv Discover
// hub items use composite UUID-shaped strings (e.g.
// "691648f137d5bdeaa81f55b1-6918087fc7abb5aa29a67b10"). Both shapes
// must decode cleanly into the same Go type.
//
// Underlying string — callers convert to plain string with `string(p)`.
type PartID string

// UnmarshalJSON accepts either a JSON string or a JSON number. Numbers
// are preserved verbatim (Go marshals them back as numbers if needed,
// but practically every consumer treats id as opaque text).
func (p *PartID) UnmarshalJSON(data []byte) error {
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		*p = PartID(data[1 : len(data)-1])
		return nil
	}
	*p = PartID(data)
	return nil
}

// Part is a single file backing a Media. Most items have exactly one Part.
type Part struct {
	ID        PartID `json:"id"`
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
	// NEW (Task 12) — surfaces fields needed by DiscoverTile for rich card
	// info, watchlist parent navigation, and the trailer cascade.
	IMDBID                string `json:"imdbId,omitempty"`
	ParentRatingKey       string `json:"parentRatingKey,omitempty"`      // for clip items: the parent movie/show ratingKey
	GrandparentRatingKey  string `json:"grandparentRatingKey,omitempty"` // for episode-clips: the show ratingKey (fallback)
	ContentRating         string `json:"contentRating,omitempty"`
	Studio                string `json:"studio,omitempty"`
	Tagline               string `json:"tagline,omitempty"`
	AddedAt               int64  `json:"addedAt,omitempty"`
	OriginallyAvailableAt string `json:"originallyAvailableAt,omitempty"`
	// Per-type display fields surfaced for DiscoverTile rendering parity
	// with Plex Web's MediaContainer.Meta.DisplayFields directive
	// (Session 6.5 Coming Soon capture). Season items render
	// parentTitle / title / date; episode items render
	// grandparentTitle / S{parentIndex}E{index} / date.
	ParentTitle      string `json:"parentTitle,omitempty"`
	ParentIndex      int    `json:"parentIndex,omitempty"`
	Index            int    `json:"index,omitempty"`
	GrandparentTitle string `json:"grandparentTitle,omitempty"`
	// HLSUrl is the native HLS playback URL for clip-type hub items
	// (Trending Trailers / New Trailers). Extracted from Media[].Part[].key
	// in GetHub and qualified to an absolute URL with the account token
	// applied so the SPA can hand it straight to <video>/hls.js. Empty for
	// non-clip items and for clips that lack a Media/Part chain.
	HLSUrl string `json:"hlsUrl,omitempty"`
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
