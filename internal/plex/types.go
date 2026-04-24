package plex

// Server is one Plex Media Server the account has access to.
// BaseURL is set by PickConnection.
type Server struct {
	Name              string
	MachineIdentifier string
	AccessToken       string
	BaseURL           string      // populated by connection picker
	Connections       []Connection
}

// Connection is a single advertised URI for a Plex server.
type Connection struct {
	URI      string // e.g. https://1-2-3-4.plex.direct:32400
	Address  string
	Port     int
	Local    bool
	Relay    bool
	Protocol string // "https" only in practice (we pass includeHttps=1)
	IPv6     bool
}

// Library is a top-level section on a server (Movies, TV Shows, Anime, etc.).
type Library struct {
	ID    string
	Key   string // numeric section key used in URLs
	Title string
	Type  string // "movie" | "show" | ...
}

// Item is a single piece of media — movie, show, season, or episode.
type Item struct {
	RatingKey string
	GUID      string // plex://movie/<guid>, plex://show/<guid>, etc.
	Title     string
	Type      string
	Year      int
	Summary   string
}

// HubItem is one card on a plex.tv Discover hub (home or watchlist namespace).
type HubItem struct {
	GUID      string
	RatingKey string
	Title     string
	Type      string
	Year      int
}

// ItemQuery carries optional filter/sort parameters for GetItems.
type ItemQuery struct {
	Sort    string // e.g. "addedAt:desc"
	Filters map[string]string
	Start   int // pagination offset
	Size    int // page size
}
