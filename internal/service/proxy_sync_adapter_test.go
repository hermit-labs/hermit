package service

import (
	"encoding/json"
	"testing"
)

func TestHasFileDescriptors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  json.RawMessage
		want bool
	}{
		{name: "empty", raw: nil, want: false},
		{name: "null", raw: json.RawMessage(`null`), want: false},
		{name: "empty array", raw: json.RawMessage(`[]`), want: false},
		{name: "invalid json", raw: json.RawMessage(`{`), want: false},
		{name: "one file metadata", raw: json.RawMessage(`[{"path":"SKILL.md","size":10}]`), want: true},
		{name: "one file with legacy content", raw: json.RawMessage(`[{"path":"SKILL.md","content":"abc","contentEncoding":"utf-8"}]`), want: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := hasFileDescriptors(tt.raw)
			if got != tt.want {
				t.Fatalf("hasFileDescriptors(%s) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
