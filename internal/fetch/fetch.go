// Package fetch finds media on the web and downloads it. Searching goes
// through each site's public API or page, or through yt-dlp and gallery-dl
// where those already know the site; downloading goes through yt-dlp for
// pages and plain HTTP for files, and ffmpeg converts between formats.
package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// Item is one search result.
type Item struct {
	Provider string
	ID       string
	Title    string
	Author   string
	URL      string // a page yt-dlp understands, or the media file itself
	Direct   bool   // URL is the media file
	Thumb    string
	Kind     string  // video, gif, image, audio
	Ext      string  // file extension of a direct URL
	Duration float64 // seconds, 0 when unknown
	Views    int64
	Date     string
	Width    int
	Height   int
	Size     int64
	Desc     string
}

// Query is what the user asked for.
type Query struct {
	Text     string
	Kind     string // any, video, gif, image
	Duration string // any, short (< 4 min), medium, long (> 20 min)
	Sort     string // relevance, date, views
	Offset   int    // results already shown, for "more"
	Limit    int
}

// Filter values, in the order the interface cycles through them.
var (
	Kinds     = []string{"any", "video", "gif", "image"}
	Durations = []string{"any", "short", "medium", "long"}
	Sorts     = []string{"relevance", "date", "views"}
)

// Keys are the optional API keys. Every provider works without one.
type Keys struct {
	YouTube string // one key, or several separated by commas
	Giphy   string
}

// Provider searches one site.
type Provider struct {
	Name   string
	Hint   string // what it is good for, and which filters it honours
	Needs  string // external program it relies on, if any
	Search func(ctx context.Context, q Query, k Keys) ([]Item, error)
}

// Providers lists the sites in the order the interface shows them.
var Providers = []*Provider{
	{Name: "YouTube", Hint: "videos · filters: duration, sort", Needs: "yt-dlp", Search: searchYouTube},
	{Name: "Giphy", Hint: "GIFs, stickers and short loops", Search: searchGiphy},
	{Name: "Tenor", Hint: "reaction GIFs and memes", Search: searchTenor},
	{Name: "Pinterest", Hint: "pins: pictures, GIFs and short videos · filter: kind", Needs: "gallery-dl", Search: searchPinterest},
	{Name: "Archive", Hint: "archive.org: films, clips, old TV · filter: sort", Search: searchArchive},
	{Name: "Wikimedia", Hint: "free videos, GIFs and pictures · filter: kind", Search: searchWikimedia},
	{Name: "URL", Hint: "paste any link that yt-dlp or ffmpeg can open", Search: searchURL},
}

// Missing reports the external program a provider needs but cannot find.
func (p *Provider) Missing() string {
	if p.Needs == "" {
		return ""
	}
	if _, err := exec.LookPath(p.Needs); err != nil {
		return p.Needs
	}
	return ""
}

const userAgent = "Mozilla/5.0 (X11; Linux x86_64) termo/3 (+https://github.com/Szmelc-INC/Terminal-Motion-Engine)"

var client = &http.Client{Timeout: 25 * time.Second}

// get fetches a URL and returns its body (at most 8 MiB).
func get(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "en")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return body, &httpError{resp.StatusCode, hostOf(u)}
	}
	return body, nil
}

type httpError struct {
	code int
	host string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("%s answered %d %s", e.host, e.code, http.StatusText(e.code))
}

func hostOf(u string) string {
	if p, err := url.Parse(u); err == nil && p.Host != "" {
		return p.Host
	}
	return u
}

func getJSON(ctx context.Context, u string, v any) error {
	body, err := get(ctx, u)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("%s sent something that is not JSON", hostOf(u))
	}
	return nil
}

// run executes a helper program and returns its standard output. On
// failure the last line of its error output becomes the error.
func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if _, err := exec.LookPath(name); err != nil {
		return nil, fmt.Errorf("%s is not installed", name)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		if len(out) > 0 {
			return out, nil // partial results beat none (e.g. one bad entry)
		}
		return nil, errors.New(name + ": " + lastLine(stderr.String(), err.Error()))
	}
	return out, nil
}

func lastLine(s, fallback string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			l = strings.TrimPrefix(l, "ERROR: ")
			if len(l) > 160 {
				l = l[:160] + "…"
			}
			return l
		}
	}
	return fallback
}

// IsURL reports whether s looks like a web address.
func IsURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

var directExts = map[string]string{
	".mp4": "video", ".webm": "video", ".mkv": "video", ".mov": "video", ".m4v": "video", ".ogv": "video",
	".avi": "video", ".m3u8": "video", ".gif": "gif", ".apng": "gif", ".png": "image", ".jpg": "image",
	".jpeg": "image", ".webp": "image", ".bmp": "image", ".mp3": "audio", ".wav": "audio", ".flac": "audio",
	".ogg": "audio", ".opus": "audio", ".m4a": "audio",
}

// extOf returns the lower-case extension of a URL's path ("" if none).
func extOf(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	path := p.Path
	if i := strings.LastIndexByte(path, '.'); i >= 0 && i > strings.LastIndexByte(path, '/') && len(path)-i <= 6 {
		return strings.ToLower(path[i:])
	}
	return ""
}

func limit(q Query, def int) int {
	if q.Limit > 0 {
		return q.Limit
	}
	return def
}

// keep applies the filters a provider could not apply itself.
func keep(items []Item, q Query) []Item {
	out := items[:0]
	for _, it := range items {
		if q.Kind != "" && q.Kind != "any" && it.Kind != "" && it.Kind != q.Kind {
			continue
		}
		if it.Duration > 0 {
			switch q.Duration {
			case "short":
				if it.Duration >= 240 {
					continue
				}
			case "medium":
				if it.Duration < 240 || it.Duration > 1200 {
					continue
				}
			case "long":
				if it.Duration <= 1200 {
					continue
				}
			}
		}
		out = append(out, it)
	}
	return out
}
