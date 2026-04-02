package main

import (
	"strings"
	"testing"
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
