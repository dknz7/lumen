package plex

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverServersFiltersToPMSAndParsesConnections(t *testing.T) {
	payload := []map[string]any{
		{
			"name":             "Stargaze",
			"clientIdentifier": "m-stargaze",
			"accessToken":      "tok-stargaze",
			"product":          "Plex Media Server",
			"connections": []map[string]any{
				{"uri": "https://1-2-3-4.plex.direct:32400", "address": "1.2.3.4", "port": 32400, "local": false, "relay": false, "protocol": "https", "IPv6": false},
				{"uri": "https://relay.plex.tv/abc", "address": "relay", "port": 443, "local": false, "relay": true, "protocol": "https", "IPv6": false},
			},
		},
		{
			"name":             "DKNZPLEX",
			"clientIdentifier": "m-dknzplex",
			"accessToken":      "tok-dknz",
			"product":          "Plex Media Server",
			"connections":      []map[string]any{{"uri": "https://5-6-7-8.plex.direct:32400", "address": "5.6.7.8", "port": 32400, "local": false, "relay": false, "protocol": "https", "IPv6": false}},
		},
		{ // non-PMS product — must be filtered out
			"name":    "Plex Web",
			"product": "Plex Web",
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("includeHttps") != "1" || r.URL.Query().Get("includeRelay") != "1" {
			http.Error(w, "missing query params", 400)
			return
		}
		if r.Header.Get("X-Plex-Token") != "acct-tok" {
			http.Error(w, "missing token", 401)
			return
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.plexTVBase = srv.URL
	servers, err := c.DiscoverServers("acct-tok")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2 (PMS-only)", len(servers))
	}
	if servers[0].Name != "Stargaze" || servers[0].MachineIdentifier != "m-stargaze" {
		t.Errorf("server 0: %+v", servers[0])
	}
	if servers[0].AccessToken != "tok-stargaze" {
		t.Errorf("AccessToken mismatch")
	}
	if len(servers[0].Connections) != 2 {
		t.Errorf("want 2 connections, got %d", len(servers[0].Connections))
	}
	if !servers[0].Connections[1].Relay {
		t.Errorf("second connection should be relay=true")
	}
}
