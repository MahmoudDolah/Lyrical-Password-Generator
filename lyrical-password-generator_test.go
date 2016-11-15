package main

import "testing"

func TestGetArtist(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		if got := GetArtist(); got != tt.want {
			t.Errorf("%q. GetArtist() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestGetSong(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		if got := GetSong(); got != tt.want {
			t.Errorf("%q. GetSong() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestGetLyrics(t *testing.T) {
	type args struct {
		artist string
		title  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		if got := GetLyrics(tt.args.artist, tt.args.title); got != tt.want {
			t.Errorf("%q. GetLyrics() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func Test_main(t *testing.T) {
	tests := []struct {
		name string
	}{
	// TODO: Add test cases.
	}
	for range tests {
		main()
	}
}
