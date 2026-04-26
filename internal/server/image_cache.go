package server

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"lumen/internal/config"
)

// imageCache stores proxied images on disk at %APPDATA%\Lumen\cache\images\.
// Two files per entry:
//
//	<hash>      the raw image bytes
//	<hash>.ct   the stored Content-Type as plain text
//
// Cache is write-through: fetch from Plex → write to disk → serve. Subsequent
// requests for the same key read straight from disk without hitting Plex.
//
// No eviction policy — spec §13.6 exposes a "Clear image cache" button in
// Settings that wipes the whole directory. Per-file TTL would add complexity
// for little benefit when the admin control is right there.
type imageCache struct {
	dir string
}

func newImageCache() *imageCache {
	dir := filepath.Join(config.CacheDir(), "images")
	_ = os.MkdirAll(dir, 0o755)
	return &imageCache{dir: dir}
}

// key hashes the request identity into a stable filename-safe string.
// serverID + path + width + height are the only things that change the bytes.
func (c *imageCache) key(serverID, path string, w, h int) string {
	sum := sha256.Sum256([]byte(serverID + "|" + path + "|" + strconv.Itoa(w) + "x" + strconv.Itoa(h)))
	return hex.EncodeToString(sum[:])
}

// get returns the cached image bytes and content-type, or ok=false if not cached.
func (c *imageCache) get(key string) (contentType string, data []byte, ok bool) {
	dataPath := filepath.Join(c.dir, key)
	ctPath := dataPath + ".ct"
	ctBytes, err := os.ReadFile(ctPath)
	if err != nil {
		return "", nil, false
	}
	data, err = os.ReadFile(dataPath)
	if err != nil {
		return "", nil, false
	}
	return strings.TrimSpace(string(ctBytes)), data, true
}

// put writes the image + content-type sidecar atomically.
// Uses tmp-then-rename so partial writes don't leave half-baked cache entries.
func (c *imageCache) put(key, contentType string, data []byte) error {
	if contentType == "" {
		contentType = "image/jpeg"
	}
	dataPath := filepath.Join(c.dir, key)
	ctPath := dataPath + ".ct"

	if err := writeFileAtomic(dataPath, data); err != nil {
		return err
	}
	return writeFileAtomic(ctPath, []byte(contentType))
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		// If rename fails for any reason, make sure we don't leave the tmp behind.
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// exists returns true if both sidecar files are present.
func (c *imageCache) exists(key string) bool {
	dataPath := filepath.Join(c.dir, key)
	if _, err := os.Stat(dataPath); errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if _, err := os.Stat(dataPath + ".ct"); errors.Is(err, fs.ErrNotExist) {
		return false
	}
	return true
}
