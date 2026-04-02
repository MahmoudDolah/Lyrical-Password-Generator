package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetLyrics(t *testing.T) {
	tests := []struct {
		name        string
		artist      string
		title       string
		wantContain string
		wantErr     bool
	}{
		{
			name:        "valid artist and song",
			artist:      "The Beatles",
			title:       "Hey Jude",
			wantContain: "Jude",
			wantErr:     false,
		},
		{
			name:    "unknown song returns error",
			artist:  "zzz_no_such_artist_zzz",
			title:   "zzz_no_such_song_zzz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetLyrics(tt.artist, tt.title)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLyrics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantContain != "" && !strings.Contains(got, tt.wantContain) {
				t.Errorf("GetLyrics() = %q, want it to contain %q", got, tt.wantContain)
			}
		})
	}
}

func TestGeneratePassword(t *testing.T) {
	tests := []struct {
		name       string
		lyrics     string
		wantEmpty  bool
		minLen     int
	}{
		{
			name:      "normal lyrics produce a password",
			lyrics:    "Hey Jude don't make it bad\nTake a sad song and make it better\nRemember to let her into your heart",
			wantEmpty: false,
			minLen:    5,
		},
		{
			name:      "lyrics with only short lines produce empty string",
			lyrics:    "Hey\nJude\nNo",
			wantEmpty: true,
		},
		{
			name:      "empty lyrics produce empty string",
			lyrics:    "",
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GeneratePassword(tt.lyrics)
			if tt.wantEmpty && got != "" {
				t.Errorf("GeneratePassword() = %q, want empty", got)
			}
			if !tt.wantEmpty && len(got) < tt.minLen {
				t.Errorf("GeneratePassword() = %q, want len >= %d", got, tt.minLen)
			}
		})
	}
}

func TestGeneratePKCE(t *testing.T) {
	v1, c1, err := generatePKCE()
	if err != nil {
		t.Fatalf("generatePKCE() error = %v", err)
	}
	v2, _, err := generatePKCE()
	if err != nil {
		t.Fatalf("generatePKCE() second call error = %v", err)
	}

	if len(v1) < 43 {
		t.Errorf("verifier length %d < 43 (RFC 7636 minimum)", len(v1))
	}
	if v1 == v2 {
		t.Error("two calls produced identical verifiers")
	}

	// Re-derive the challenge from v1 and confirm it matches c1.
	h := sha256.Sum256([]byte(v1))
	wantChallenge := base64.RawURLEncoding.EncodeToString(h[:])
	if c1 != wantChallenge {
		t.Errorf("challenge = %q, want %q", c1, wantChallenge)
	}
}

func TestTokenFreshness(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		wantFresh bool
	}{
		{"expired", time.Now().UTC().Add(-1 * time.Hour), false},
		{"expires in 30s (within buffer)", time.Now().UTC().Add(30 * time.Second), false},
		{"expires in 2 minutes", time.Now().UTC().Add(2 * time.Minute), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := &SpotifyToken{ExpiresAt: tt.expiresAt}
			if got := isTokenFresh(tok); got != tt.wantFresh {
				t.Errorf("isTokenFresh() = %v, want %v", got, tt.wantFresh)
			}
		})
	}
}

func TestSaveLoadToken(t *testing.T) {
	dir := t.TempDir()
	tokenConfigOverride = dir
	t.Cleanup(func() { tokenConfigOverride = "" })

	original := &SpotifyToken{
		AccessToken:  "access-abc",
		RefreshToken: "refresh-xyz",
		ExpiresAt:    time.Now().UTC().Add(1 * time.Hour).Truncate(time.Second),
	}

	if err := saveToken(original); err != nil {
		t.Fatalf("saveToken() error = %v", err)
	}

	// Verify file permissions.
	path := filepath.Join(dir, "spotify.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("token file permissions = %o, want 0600", perm)
	}

	loaded, err := loadToken()
	if err != nil {
		t.Fatalf("loadToken() error = %v", err)
	}
	if loaded == nil {
		t.Fatal("loadToken() returned nil, want token")
	}
	if loaded.AccessToken != original.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, original.AccessToken)
	}
	if loaded.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", loaded.RefreshToken, original.RefreshToken)
	}
	if !loaded.ExpiresAt.Equal(original.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", loaded.ExpiresAt, original.ExpiresAt)
	}
}

func TestLoadTokenMissingFile(t *testing.T) {
	tokenConfigOverride = t.TempDir()
	t.Cleanup(func() { tokenConfigOverride = "" })

	tok, err := loadToken()
	if err != nil {
		t.Fatalf("loadToken() on missing file returned error: %v", err)
	}
	if tok != nil {
		t.Errorf("loadToken() = %v, want nil for missing file", tok)
	}
}

func TestParseTopTracks(t *testing.T) {
	makeJSON := func(items []map[string]any) []byte {
		b, _ := json.Marshal(map[string]any{"items": items})
		return b
	}

	singleArtist := makeJSON([]map[string]any{
		{"name": "Hey Jude", "artists": []map[string]any{{"name": "The Beatles"}}},
	})
	multiArtist := makeJSON([]map[string]any{
		{
			"name": "Collab Song",
			"artists": []map[string]any{
				{"name": "Artist A"},
				{"name": "Artist B"},
			},
		},
	})
	emptyItems := makeJSON([]map[string]any{})

	tests := []struct {
		name       string
		data       []byte
		wantArtist string
		wantTitle  string
		wantErr    bool
	}{
		{"single artist", singleArtist, "The Beatles", "Hey Jude", false},
		{"multi-artist uses first", multiArtist, "Artist A", "Collab Song", false},
		{"empty items", emptyItems, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artist, title, err := parseTopTracks(tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTopTracks() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if artist != tt.wantArtist {
					t.Errorf("artist = %q, want %q", artist, tt.wantArtist)
				}
				if !strings.Contains(title, tt.wantTitle) {
					t.Errorf("title = %q, want %q", title, tt.wantTitle)
				}
			}
		})
	}
}
