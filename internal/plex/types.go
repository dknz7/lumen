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
	RatingKey            string `json:"ratingKey"`
	GUID                 string `json:"guid,omitempty"`
	Title                string `json:"title"`
	Type                 string `json:"type"` // "movie" | "show" | "season" | "episode"
	Year                 int    `json:"year,omitempty"`
	Summary              string `json:"summary,omitempty"`
	Thumb                string `json:"thumb,omitempty"` // server-relative path to portrait poster
	Art                  string `json:"art,omitempty"`   // server-relative path to landscape art
	Duration             int64  `json:"duration,omitempty"`   // media length in ms
	ViewOffset           int64  `json:"viewOffset,omitempty"` // resume position in ms (0 = unstarted)
	// Episode-specific fields — empty for non-episodes.
	Index                int    `json:"index,omitempty"`                // episode number within season
	ParentIndex          int    `json:"parentIndex,omitempty"`          // season number
	ParentTitle          string `json:"parentTitle,omitempty"`          // e.g. "Season 1"
	ParentThumb          string `json:"parentThumb,omitempty"`          // season poster
	GrandparentTitle     string `json:"grandparentTitle,omitempty"`     // show name
	GrandparentThumb     string `json:"grandparentThumb,omitempty"`     // show poster (portrait)
	GrandparentArt       string `json:"grandparentArt,omitempty"`       // show backdrop (landscape)
	GrandparentRatingKey string `json:"grandparentRatingKey,omitempty"` // show's ratingKey
}

// HubItem is one card on a plex.tv Discover hub (home or watchlist namespace).
type HubItem struct {
	GUID      string `json:"guid,omitempty"`
	RatingKey string `json:"ratingKey"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Year      int    `json:"year,omitempty"`
}

// ItemQuery carries optional filter/sort parameters for GetItems.
// Not serialized — input to the Go client only.
type ItemQuery struct {
	Sort    string
	Filters map[string]string
	Start   int
	Size    int
}
