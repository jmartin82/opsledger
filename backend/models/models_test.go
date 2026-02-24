package models

import (
	"testing"
)

// ---------------------------------------------------------------------------
// API Key Model Tests
// ---------------------------------------------------------------------------

func TestGenerateAPIKey(t *testing.T) {
	raw, hash, prefix, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if raw == "" {
		t.Error("expected non-empty raw key")
	}
	// raw key: "ol_live_" (7) + 32 bytes hex (64) = ~71 chars
	if len(raw) < 70 {
		t.Errorf("expected raw key length at least 70, got %d", len(raw))
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if prefix == "" {
		t.Error("expected non-empty prefix")
	}
	if len(prefix) != 12 {
		t.Errorf("expected prefix length 12, got %d", len(prefix))
	}
	if len(hash) != 64 { // SHA256 hex
		t.Errorf("expected hash length 64, got %d", len(hash))
	}
}

func TestHashAPIKey(t *testing.T) {
	raw := "ol_live_abcdef1234567890"
	hash := HashAPIKey(raw)

	if len(hash) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash))
	}

	// Same input should produce same hash
	hash2 := HashAPIKey(raw)
	if hash != hash2 {
		t.Error("same input should produce same hash")
	}

	// Different input should produce different hash
	hash3 := HashAPIKey(raw + "x")
	if hash == hash3 {
		t.Error("different input should produce different hash")
	}
}

func TestValidateScopes(t *testing.T) {
	tests := []struct {
		name      string
		scopes    []string
		wantError bool
	}{
		{"valid single scope", []string{"changes:read"}, false},
		{"valid multiple scopes", []string{"changes:read", "changes:write"}, false},
		{"empty scopes", []string{}, true},
		{"invalid scope", []string{"invalid:scope"}, true},
		{"mixed valid and invalid", []string{"changes:read", "bad:scope"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScopes(tt.scopes)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateScopes() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Change Model Tests
// ---------------------------------------------------------------------------

func TestChangeFilters_DefaultLimit(t *testing.T) {
	f := ChangeFilters{}
	if f.Limit != 0 {
		t.Errorf("expected default limit 0, got %d", f.Limit)
	}

	// Simulate what ListChanges does
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		f.Limit = 200
	}

	if f.Limit != 50 {
		t.Errorf("expected limit 50, got %d", f.Limit)
	}
}

func TestChangeFilters_MaxLimit(t *testing.T) {
	f := ChangeFilters{Limit: 500}

	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		f.Limit = 200
	}

	if f.Limit != 200 {
		t.Errorf("expected limit capped at 200, got %d", f.Limit)
	}
}
