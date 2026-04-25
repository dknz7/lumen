package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lumen/internal/config"
)

func TestHandlePlay_RejectsGET(t *testing.T) {
	s := New(&config.Config{}, nil, "127.0.0.1:0")
	req := httptest.NewRequest(http.MethodGet, "/api/play", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", rr.Code)
	}
}

func TestHandlePlay_BadJSON(t *testing.T) {
	s := New(&config.Config{}, nil, "127.0.0.1:0")
	req := httptest.NewRequest(http.MethodPost, "/api/play", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rr.Code)
	}
}

func TestHandlePlay_UnknownServer(t *testing.T) {
	s := New(&config.Config{}, nil, "127.0.0.1:0")
	body, _ := json.Marshal(playRequest{ServerID: "missing", RatingKey: "1"})
	req := httptest.NewRequest(http.MethodPost, "/api/play", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rr.Code)
	}
}
