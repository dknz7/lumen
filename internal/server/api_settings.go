package server

import (
	"encoding/json"
	"net/http"

	"lumen/internal/config"
)

// handleSettings dispatches GET (returns UI config) or PUT (patches it).
// Non-UI config fields (tokens, servers, client identifier) are never exposed.
//
// Both directions go through the locked accessors. UIConfig contains maps
// (ShelfState and the maps nested inside it), and the SPA autosaves shelf
// layout on drag — so an unsynchronised GET marshalling while a PUT writes is
// `fatal error: concurrent map read and map write`, which kills the process.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// Marshal inside the read lock, then write the bytes outside it.
		b, err := s.marshalCfg(func(c *config.Config) any { return c.UI })
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encode settings")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(b)

	case "PUT":
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var incoming struct {
			Theme           *string `json:"theme"`
			Zoom            *int    `json:"zoom"`
			CardSize        *string `json:"cardSize"`
			CardDensity     *int    `json:"cardDensity"`
			RowsPerShelf    *int    `json:"rowsPerShelf"`
			FontSize        *int    `json:"fontSize"`
			CardLayout      *string `json:"cardLayout"`
			DefaultSort     *string `json:"defaultSort"`
			DefaultViewMode *string `json:"defaultViewMode"`
			Playback        *struct {
				PotPlayerPath *string `json:"potPlayerPath"`
			} `json:"playback"`
			Window *struct {
				CloseAction    *string `json:"closeAction"`
				MinimizeToTray *bool   `json:"minimizeToTray"`
				StartHidden    *bool   `json:"startHidden"`
			} `json:"window"`
			HiddenLibraries *[]string       `json:"hiddenLibraries"`
			ShelfState      *map[string]any `json:"shelfState"`
		}
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}

		// Apply any fields provided. Untouched fields keep their current values.
		var out []byte
		err := s.mutateCfg(func(c *config.Config) {
			if incoming.Theme != nil {
				c.UI.Theme = *incoming.Theme
			}
			if incoming.Zoom != nil {
				c.UI.Zoom = *incoming.Zoom
			}
			if incoming.CardSize != nil {
				c.UI.CardSize = *incoming.CardSize
			}
			if incoming.CardDensity != nil {
				c.UI.CardDensity = *incoming.CardDensity
			}
			if incoming.RowsPerShelf != nil {
				c.UI.RowsPerShelf = *incoming.RowsPerShelf
			}
			if incoming.FontSize != nil {
				c.UI.FontSize = *incoming.FontSize
			}
			if incoming.CardLayout != nil {
				c.UI.CardLayout = *incoming.CardLayout
			}
			if incoming.DefaultSort != nil {
				c.UI.DefaultSort = *incoming.DefaultSort
			}
			if incoming.DefaultViewMode != nil {
				c.UI.DefaultViewMode = *incoming.DefaultViewMode
			}
			if incoming.Playback != nil && incoming.Playback.PotPlayerPath != nil {
				c.UI.Playback.PotPlayerPath = *incoming.Playback.PotPlayerPath
			}
			if incoming.Window != nil {
				if incoming.Window.CloseAction != nil {
					// Anything other than "quit" means close-to-tray; guard the
					// value so a typo can't leave the window unclosable.
					if *incoming.Window.CloseAction == "quit" {
						c.UI.Window.CloseAction = "quit"
					} else {
						c.UI.Window.CloseAction = "tray"
					}
				}
				if incoming.Window.MinimizeToTray != nil {
					c.UI.Window.MinimizeToTray = *incoming.Window.MinimizeToTray
				}
				if incoming.Window.StartHidden != nil {
					c.UI.Window.StartHidden = *incoming.Window.StartHidden
				}
			}
			if incoming.HiddenLibraries != nil {
				c.UI.HiddenLibraries = *incoming.HiddenLibraries
			}
			if incoming.ShelfState != nil {
				// Re-marshal + unmarshal to coerce map[string]any → typed
				// PageShelfState. Unmarshal merges into the existing map, so
				// reset it first to make a PUT authoritative.
				raw, _ := json.Marshal(*incoming.ShelfState)
				c.UI.ShelfState = map[string]config.PageShelfState{}
				_ = json.Unmarshal(raw, &c.UI.ShelfState)
			}
			// Encode the response inside the same critical section: doing it
			// after the lock releases would race the next PUT.
			out, _ = json.Marshal(c.UI)
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "save: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(out)

	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or PUT only")
	}
}
