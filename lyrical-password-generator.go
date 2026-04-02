package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"unicode"
)

type lyricsResponse struct {
	Lyrics string `json:"lyrics"`
	Error  string `json:"error"`
}

var stdinReader = bufio.NewReader(os.Stdin)

func GetFavoriteArtist() string {
	fmt.Printf("Enter favorite artist: ")
	artist, _ := stdinReader.ReadString('\n')
	return strings.TrimSpace(artist)
}

func GetFavoriteSong() string {
	fmt.Printf("Enter favorite song: ")
	song, _ := stdinReader.ReadString('\n')
	return strings.TrimSpace(song)
}

func GetLyrics(artist string, title string) (string, error) {
	base := "https://api.lyrics.ovh/v1/"
	endpoint := base + url.PathEscape(artist) + "/" + url.PathEscape(title)

	resp, err := http.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("http.Get: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading body: %w", err)
	}

	var result lyricsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("API error: %s", result.Error)
	}

	return result.Lyrics, nil
}

// GeneratePassword picks a random non-empty line from the lyrics, takes the
// first letter of each word, applies simple leet substitutions, and appends
// a two-digit number derived from the line index to meet basic complexity.
func GeneratePassword(lyrics string) string {
	lines := strings.Split(lyrics, "\n")

	var candidates []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(strings.Fields(line)) >= 3 {
			candidates = append(candidates, line)
		}
	}
	if len(candidates) == 0 {
		return ""
	}

	chosen := candidates[rand.Intn(len(candidates))]
	words := strings.Fields(chosen)

	leet := map[rune]rune{
		'a': '@', 'e': '3', 'i': '1', 'o': '0', 's': '$',
	}

	var sb strings.Builder
	for i, word := range words {
		ch := rune(word[0])
		if i%2 == 0 {
			ch = unicode.ToUpper(ch)
		} else {
			ch = unicode.ToLower(ch)
		}
		if sub, ok := leet[unicode.ToLower(ch)]; ok {
			sb.WriteRune(sub)
		} else {
			sb.WriteRune(ch)
		}
	}

	// Append a two-digit suffix for added complexity
	suffix := rand.Intn(90) + 10
	sb.WriteString(fmt.Sprintf("%d!", suffix))

	return sb.String()
}

func main() {
	artist := GetFavoriteArtist()
	song := GetFavoriteSong()

	lyrics, err := GetLyrics(artist, song)
	if err != nil {
		log.Fatalf("Could not fetch lyrics: %v", err)
	}

	password := GeneratePassword(lyrics)
	if password == "" {
		log.Fatal("Could not generate password from lyrics")
	}

	fmt.Printf("\nYour lyrical password: %s\n", password)
}
