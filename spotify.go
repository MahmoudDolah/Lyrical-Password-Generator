package main

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	spotifyAuthURL   = "https://accounts.spotify.com/authorize"
	spotifyTokenURL  = "https://accounts.spotify.com/api/token"
	spotifyTopTracks = "https://api.spotify.com/v1/me/top/tracks"
	callbackAddr     = "localhost:8080"
	callbackPath     = "/callback"
)

// tokenConfigOverride allows tests to redirect token storage to a temp directory.
var tokenConfigOverride string

// SpotifyToken is the on-disk and in-memory representation of OAuth tokens.
type SpotifyToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type spotifyTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

type spotifyTopTracksResponse struct {
	Items []struct {
		Name    string `json:"name"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
	} `json:"items"`
}

// tokenConfigPath returns the path to the token JSON file.
func tokenConfigPath() (string, error) {
	if tokenConfigOverride != "" {
		return filepath.Join(tokenConfigOverride, "spotify.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}
	return filepath.Join(home, ".config", "lyrical-passwd", "spotify.json"), nil
}

// loadToken reads the stored token from disk. Returns nil, nil if no token file exists yet.
func loadToken() (*SpotifyToken, error) {
	path, err := tokenConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading token file: %w", err)
	}
	var t SpotifyToken
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parsing token file: %w", err)
	}
	return &t, nil
}

// saveToken writes the token to disk with restrictive permissions.
func saveToken(t *SpotifyToken) error {
	path, err := tokenConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing token: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// isTokenFresh reports whether the token has more than 60 seconds of lifetime left.
func isTokenFresh(t *SpotifyToken) bool {
	return time.Now().UTC().Before(t.ExpiresAt.Add(-60 * time.Second))
}

// generatePKCE produces a PKCE verifier and its S256 challenge per RFC 7636.
func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 64)
	if _, err = cryptorand.Read(b); err != nil {
		return "", "", fmt.Errorf("generating PKCE verifier: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

// openBrowser opens the given URL in the default system browser.
func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", rawURL)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

// buildAuthURL constructs the Spotify authorization URL and a random state value.
func buildAuthURL(clientID, challenge string) (authURL, state string, err error) {
	stateBytes := make([]byte, 8)
	if _, err = cryptorand.Read(stateBytes); err != nil {
		return "", "", fmt.Errorf("generating state: %w", err)
	}
	state = hex.EncodeToString(stateBytes)

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", "http://"+callbackAddr+callbackPath)
	params.Set("scope", "user-top-read")
	params.Set("code_challenge_method", "S256")
	params.Set("code_challenge", challenge)
	params.Set("state", state)

	authURL = spotifyAuthURL + "?" + params.Encode()
	return authURL, state, nil
}

// runCallbackServer spins up a temporary HTTP server on localhost:8080 to catch
// the OAuth redirect from Spotify. It returns the authorization code.
func runCallbackServer(expectedState string) (string, error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != expectedState {
			errCh <- fmt.Errorf("state mismatch: possible CSRF")
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		if e := q.Get("error"); e != "" {
			errCh <- fmt.Errorf("spotify auth error: %s", e)
			http.Error(w, e, http.StatusBadRequest)
			return
		}
		code := q.Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no authorization code in callback")
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<h1>Authentication successful! You can close this tab.</h1>")
		codeCh <- code
	})

	srv := &http.Server{Addr: callbackAddr, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case code := <-codeCh:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx) //nolint:errcheck
		return code, nil
	case err := <-errCh:
		srv.Close() //nolint:errcheck
		return "", err
	case <-time.After(2 * time.Minute):
		srv.Close() //nolint:errcheck
		return "", fmt.Errorf("timed out waiting for Spotify callback")
	}
}

// exchangeCodeForToken exchanges an authorization code for access and refresh tokens.
func exchangeCodeForToken(clientID, code, verifier string) (*SpotifyToken, error) {
	body := url.Values{}
	body.Set("grant_type", "authorization_code")
	body.Set("code", code)
	body.Set("redirect_uri", "http://"+callbackAddr+callbackPath)
	body.Set("client_id", clientID)
	body.Set("code_verifier", verifier)

	return postToTokenEndpoint(body)
}

// refreshSpotifyToken exchanges a refresh token for a new access token.
func refreshSpotifyToken(clientID string, t *SpotifyToken) (*SpotifyToken, error) {
	body := url.Values{}
	body.Set("grant_type", "refresh_token")
	body.Set("refresh_token", t.RefreshToken)
	body.Set("client_id", clientID)

	refreshed, err := postToTokenEndpoint(body)
	if err != nil {
		return nil, err
	}
	// Spotify may or may not rotate the refresh token — preserve the old one if absent.
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = t.RefreshToken
	}
	return refreshed, nil
}

// postToTokenEndpoint posts form values to the Spotify token endpoint and returns the token.
func postToTokenEndpoint(body url.Values) (*SpotifyToken, error) {
	resp, err := http.PostForm(spotifyTokenURL, body)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}

	var tr spotifyTokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}
	if tr.Error != "" {
		return nil, fmt.Errorf("token error %s: %s", tr.Error, tr.ErrorDesc)
	}

	return &SpotifyToken{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(tr.ExpiresIn) * time.Second),
	}, nil
}

// ensureValidToken loads a stored token, refreshes it if needed, or runs a full
// PKCE auth flow if no usable token exists.
func ensureValidToken(clientID string) (*SpotifyToken, error) {
	t, err := loadToken()
	if err != nil {
		return nil, err
	}

	if t != nil && isTokenFresh(t) {
		return t, nil
	}

	if t != nil && t.RefreshToken != "" {
		refreshed, err := refreshSpotifyToken(clientID, t)
		if err == nil {
			if saveErr := saveToken(refreshed); saveErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not save refreshed token: %v\n", saveErr)
			}
			return refreshed, nil
		}
		fmt.Fprintf(os.Stderr, "token refresh failed, re-authenticating: %v\n", err)
	}

	// Full PKCE flow.
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, err
	}

	authURL, state, err := buildAuthURL(clientID, challenge)
	if err != nil {
		return nil, err
	}

	fmt.Println("Opening browser for Spotify authentication...")
	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Could not open browser automatically. Visit this URL to authenticate:\n%s\n", authURL)
	}

	fmt.Println("Waiting for authorization...")
	code, err := runCallbackServer(state)
	if err != nil {
		return nil, err
	}

	t, err = exchangeCodeForToken(clientID, code, verifier)
	if err != nil {
		return nil, err
	}

	if saveErr := saveToken(t); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save token: %v\n", saveErr)
	}
	return t, nil
}

// parseTopTracks unmarshals the Spotify top tracks response and returns a random track.
func parseTopTracks(data []byte) (artist, title string, err error) {
	var result spotifyTopTracksResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", "", fmt.Errorf("parsing top tracks: %w", err)
	}
	if len(result.Items) == 0 {
		return "", "", fmt.Errorf("no top tracks found")
	}
	track := result.Items[rand.Intn(len(result.Items))]
	if len(track.Artists) == 0 {
		return "", "", fmt.Errorf("track has no artists")
	}
	return track.Artists[0].Name, track.Name, nil
}

// getTopTrack fetches the user's top 20 tracks and returns a random one.
func getTopTrack(accessToken string) (artist, title string, err error) {
	req, err := http.NewRequest(http.MethodGet, spotifyTopTracks, nil)
	if err != nil {
		return "", "", fmt.Errorf("building top tracks request: %w", err)
	}
	q := req.URL.Query()
	q.Set("time_range", "medium_term")
	q.Set("limit", "20")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetching top tracks: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("reading top tracks response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("top tracks API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return parseTopTracks(body)
}
