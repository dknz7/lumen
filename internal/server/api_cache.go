package server

import (
	"net/http"
	"os"
	"path/filepath"

	"lumen/internal/config"
)

// handleCacheSize walks cache subdirectories and returns per-scope byte totals.
func (s *Server) handleCacheSize(w http.ResponseWriter, r *http.Request) {
	images := dirSize(filepath.Join(config.CacheDir(), "images"))
	omdb := dirSize(filepath.Join(config.CacheDir(), "omdb"))
	writeJSON(w, map[string]int64{
		"images": images,
		"omdb":   omdb,
		"total":  images + omdb,
	})
}

// handleCacheClear wipes one of the named cache subdirectories.
// scope: "images" | "omdb" | "all"
func (s *Server) handleCacheClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	scope := r.URL.Query().Get("scope")
	switch scope {
	case "images":
		clearDir(filepath.Join(config.CacheDir(), "images"))
	case "omdb":
		clearDir(filepath.Join(config.CacheDir(), "omdb"))
	case "all":
		clearDir(filepath.Join(config.CacheDir(), "images"))
		clearDir(filepath.Join(config.CacheDir(), "omdb"))
	default:
		writeError(w, http.StatusBadRequest, `scope must be one of "images", "omdb", "all"`)
		return
	}
	writeJSON(w, map[string]string{"status": "cleared", "scope": scope})
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

func clearDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}
