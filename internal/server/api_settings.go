package server

import (
	"encoding/json"
	"net/http"
)

// handleSettings dispatches GET (returns UI config) or PUT (replaces it).
// Non-UI config fields (tokens, servers, client identifier) are never exposed.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		writeJSON(w, s.cfg.UI)
	case "PUT":
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
			Kiosk           *struct {
				EnableOnStartup *bool   `json:"enableOnStartup"`
				Browser         *string `json:"browser"`
			} `json:"kiosk"`
			Playback *struct {
				PotPlayerPath *string `json:"potPlayerPath"`
			} `json:"playback"`
			HiddenLibraries *[]string       `json:"hiddenLibraries"`
			ShelfState      *map[string]any `json:"shelfState"`
		}
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}

		// Apply any fields provided. Untouched fields keep their current values.
		if incoming.Theme != nil {
			s.cfg.UI.Theme = *incoming.Theme
		}
		if incoming.Zoom != nil {
			s.cfg.UI.Zoom = *incoming.Zoom
		}
		if incoming.CardSize != nil {
			s.cfg.UI.CardSize = *incoming.CardSize
		}
		if incoming.CardDensity != nil {
			s.cfg.UI.CardDensity = *incoming.CardDensity
		}
		if incoming.RowsPerShelf != nil {
			s.cfg.UI.RowsPerShelf = *incoming.RowsPerShelf
		}
		if incoming.FontSize != nil {
			s.cfg.UI.FontSize = *incoming.FontSize
		}
		if incoming.CardLayout != nil {
			s.cfg.UI.CardLayout = *incoming.CardLayout
		}
		if incoming.DefaultSort != nil {
			s.cfg.UI.DefaultSort = *incoming.DefaultSort
		}
		if incoming.DefaultViewMode != nil {
			s.cfg.UI.DefaultViewMode = *incoming.DefaultViewMode
		}
		if incoming.Kiosk != nil {
			if incoming.Kiosk.EnableOnStartup != nil {
				s.cfg.UI.Kiosk.EnableOnStartup = *incoming.Kiosk.EnableOnStartup
			}
			if incoming.Kiosk.Browser != nil {
				s.cfg.UI.Kiosk.Browser = *incoming.Kiosk.Browser
			}
		}
		if incoming.Playback != nil && incoming.Playback.PotPlayerPath != nil {
			s.cfg.UI.Playback.PotPlayerPath = *incoming.Playback.PotPlayerPath
		}
		if incoming.HiddenLibraries != nil {
			s.cfg.UI.HiddenLibraries = *incoming.HiddenLibraries
		}
		if incoming.ShelfState != nil {
			// Re-marshal + unmarshal to coerce map[string]any → typed PageShelfState.
			raw, _ := json.Marshal(*incoming.ShelfState)
			_ = json.Unmarshal(raw, &s.cfg.UI.ShelfState)
		}

		if err := s.cfg.Save(); err != nil {
			writeError(w, http.StatusInternalServerError, "save: "+err.Error())
			return
		}
		writeJSON(w, s.cfg.UI)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or PUT only")
	}
}
