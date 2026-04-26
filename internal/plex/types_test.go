package plex

import (
	"encoding/json"
	"testing"
)

// TestPartID_AcceptsBothShapes locks in the contract that Part.id
// unmarshals cleanly whether Plex returns it as a JSON number
// (server-local responses) or a JSON string (plex.tv Discover hub
// items). Both must decode without error and round-trip the value
// as text, since downstream consumers treat the id as opaque.
//
// Regression guard: a previous Session-6 hotfix changed Part.ID to
// plain `string` — broke every server-local request because Go's
// JSON decoder rejects an unquoted number into a string field.
func TestPartID_AcceptsBothShapes(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string
	}{
		{"numeric_server_local", `{"id":12345}`, "12345"},
		{"string_discover_composite", `{"id":"abc-def-ghi"}`, "abc-def-ghi"},
		{"string_numeric_lookalike", `{"id":"42"}`, "42"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p Part
			if err := json.Unmarshal([]byte(tc.json), &p); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if string(p.ID) != tc.want {
				t.Errorf("Part.ID = %q, want %q", string(p.ID), tc.want)
			}
		})
	}
}
