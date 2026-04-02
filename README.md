# Lyrical Password Generator

Generates a memorable password from song lyrics. Enter an artist and song title, and the app fetches the lyrics, picks a random line, and transforms it into a password using first-letter initials, alternating case, and leet substitutions.

**Example:** "Take a sad song and make it better" → `T@$$@m1b43!`

## Requirements

- [Go](https://go.dev/dl/) 1.16+
- Internet connection (lyrics are fetched from [lyrics.ovh](https://api.lyrics.ovh))

## Running

```bash
go run lyrical-password-generator.go
```

Then follow the prompts:

```
Enter favorite artist: The Beatles
Enter favorite song: Hey Jude

Your lyrical password: T@$$@m1b43!
```

## Testing

```bash
go test ./...
```

Note: `TestGetLyrics` makes live HTTP requests to the lyrics API.

## How passwords are generated

1. Lyrics are fetched from `api.lyrics.ovh`
2. A random line with at least 3 words is selected
3. The first letter of each word is taken
4. Letters alternate between upper and lower case
5. Leet substitutions are applied: `a→@`, `e→3`, `i→1`, `o→0`, `s→$`
6. A random two-digit number and `!` are appended
