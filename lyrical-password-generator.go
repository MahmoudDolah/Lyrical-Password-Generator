package main

import (
	"log"
	"net/http"
	"io/ioutil"
	"net/url"
	"fmt"
	"os"
	"bufio"
)

func GetFavoriteArtist() string {
	fmt.Printf("Enter favorite artist: ")
	bio := bufio.NewReader(os.Stdin)
	artist, _ := bio.ReadString('\n')
	return artist
}

func GetFavoriteSong() string{
	fmt.Printf("Enter favorite song: ")
	bio := bufio.NewReader(os.Stdin)
	song, _ := bio.ReadString('\n')
	return song
}

func GetLyrics(artist string, title string) string {
	var Url *url.URL
	Url, err := url.Parse("https://makeitpersonal.co")
	if err != nil{
		panic("boom")
	}
	Url.Path += "/lyrics"
	parameters := url.Values{}
	parameters.Add("artist", artist)
	parameters.Add("title", title)
	Url.RawQuery = parameters.Encode()
	resp, err := http.Get(Url.String())

	if err != nil {
		log.Fatalf("http.Get => %v", err.Error())
	}
	body, _ := ioutil.ReadAll(resp.Body)
	
	return string(body)
}
func main() {
	artist := GetFavoriteArtist()
	song := GetFavoriteSong()
	lyrics := GetLyrics(artist, song)
	fmt.Printf(artist)
	fmt.Printf(song)
	fmt.Printf(lyrics)
}
