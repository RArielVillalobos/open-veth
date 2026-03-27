package handlers

import (
	"encoding/json"
	"testing"
)

func TestParseResizeMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantCols uint
		wantRows uint
		wantOk   bool
	}{
		{
			name:     "valid resize message",
			input:    []byte(`{"type":"resize","cols":220,"rows":50}`),
			wantCols: 220,
			wantRows: 50,
			wantOk:   true,
		},
		{
			name:   "wrong type field",
			input:  []byte(`{"type":"input","cols":220,"rows":50}`),
			wantOk: false,
		},
		{
			name:   "zero cols",
			input:  []byte(`{"type":"resize","cols":0,"rows":50}`),
			wantOk: false,
		},
		{
			name:   "zero rows",
			input:  []byte(`{"type":"resize","cols":220,"rows":0}`),
			wantOk: false,
		},
		{
			name:   "regular keyboard input",
			input:  []byte("ls -la"),
			wantOk: false,
		},
		{
			name:   "empty message",
			input:  []byte{},
			wantOk: false,
		},
		{
			name:   "invalid json starting with brace",
			input:  []byte("{not valid json}"),
			wantOk: false,
		},
		{
			name:   "missing cols and rows",
			input:  []byte(`{"type":"resize"}`),
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, rows, ok := parseResizeMessage(tt.input)
			if ok != tt.wantOk {
				t.Errorf("parseResizeMessage() ok = %v, want %v", ok, tt.wantOk)
			}
			if ok {
				if cols != tt.wantCols {
					t.Errorf("parseResizeMessage() cols = %v, want %v", cols, tt.wantCols)
				}
				if rows != tt.wantRows {
					t.Errorf("parseResizeMessage() rows = %v, want %v", rows, tt.wantRows)
				}
			}
		})
	}
}

func TestParseResizeMessageRoundtrip(t *testing.T) {
	// Verify the format matches what the frontend sends
	msg, _ := json.Marshal(map[string]any{
		"type": "resize",
		"cols": uint(180),
		"rows": uint(40),
	})
	cols, rows, ok := parseResizeMessage(msg)
	if !ok {
		t.Fatal("expected valid resize message")
	}
	if cols != 180 || rows != 40 {
		t.Errorf("got cols=%d rows=%d, want cols=180 rows=40", cols, rows)
	}
}
